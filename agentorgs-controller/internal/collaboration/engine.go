package collaboration

import (
	"context"
	"fmt"
	"sort"
	"time"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/policy"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	"github.com/google/uuid"
)

// Engine orchestrates collaboration runs and events.
type Engine struct {
	Registry *provider.Registry
	Client   Reader
}

// Reader loads organization resources for the engine.
type Reader interface {
	ListMembers(ctx context.Context, namespace string) ([]agentorgsv1alpha1.Member, error)
	ListGroups(ctx context.Context, namespace string) ([]agentorgsv1alpha1.Group, error)
	ListPolicies(ctx context.Context, namespace string) ([]agentorgsv1alpha1.Policy, error)
	GetCollaboration(ctx context.Context, namespace, name string) (agentorgsv1alpha1.Collaboration, error)
	GetMember(ctx context.Context, namespace, name string) (agentorgsv1alpha1.Member, error)
}

// StartCollaboration starts one collaboration from a member to one or more targets.
func (e *Engine) StartCollaboration(ctx context.Context, namespace, collaborationName, from string, to []protocol.ObjectTarget, payload map[string]interface{}) (protocol.CollaborationRun, error) {
	collab, err := e.Client.GetCollaboration(ctx, namespace, collaborationName)
	if err != nil {
		return protocol.CollaborationRun{}, err
	}

	members, groups, policies, err := e.loadMembersGroupsPolicies(ctx, namespace)
	if err != nil {
		return protocol.CollaborationRun{}, err
	}
	resolver := organization.NewResolver(members, groups)
	evaluator := policy.NewEvaluator(policies, resolver)

	for _, target := range to {
		if evaluator.Check(from, target, protocol.PolicyActionStart) != policy.DecisionAllow {
			return protocol.CollaborationRun{}, fmt.Errorf("policy denied %s -> %s/%s", from, target.Kind, target.Name)
		}
	}

	resolved := map[string]struct{}{}
	for _, target := range to {
		names, err := resolver.ResolveTargets(collab.Spec.WhenTargetIsGroup.Strategy, collab.Spec.WhenTargetIsGroup.Role, agentorgsv1alpha1.ObjectRef{
			Kind: target.Kind,
			Name: target.Name,
		})
		if err != nil {
			return protocol.CollaborationRun{}, err
		}
		for _, name := range names {
			resolved[name] = struct{}{}
		}
	}

	targetNames := make([]string, 0, len(resolved))
	for name := range resolved {
		targetNames = append(targetNames, name)
	}
	sort.Strings(targetNames)
	if len(targetNames) == 0 {
		return protocol.CollaborationRun{}, fmt.Errorf("no resolved targets for collaboration %q", collaborationName)
	}

	now := time.Now().UTC()
	maxRounds := collab.Spec.Limits.MaxReplyRounds
	if maxRounds == 0 {
		maxRounds = 10
	}
	timeout := collab.Spec.Limits.TimeoutMinutes
	if timeout == 0 {
		timeout = 30
	}

	run := protocol.CollaborationRun{
		RunID:             uuid.NewString(),
		Namespace:         namespace,
		CollaborationName: collaborationName,
		StartedBy:         from,
		ResolvedTargets:   targetNames,
		Status:            protocol.RunStatusRunning,
		Round:             0,
		MaxReplyRounds:    maxRounds,
		StartedAt:         now,
		Deadline:          now.Add(time.Duration(timeout) * time.Minute),
	}

	storage, err := e.Registry.DefaultStorage()
	if err != nil {
		return protocol.CollaborationRun{}, err
	}
	if err := storage.WriteRun(ctx, run); err != nil {
		return protocol.CollaborationRun{}, err
	}

	event := protocol.CollaborationEvent{
		EventID:       uuid.NewString(),
		RunID:         run.RunID,
		Namespace:     namespace,
		Collaboration: collaborationName,
		Type:          protocol.EventTypeMemberRequest,
		Source:        protocol.EventSource{Member: from},
		Targets:       to,
		Payload:       payload,
		CreatedAt:     now,
	}
	if err := e.processMessage(ctx, run, event, collab, evaluator); err != nil {
		return protocol.CollaborationRun{}, err
	}
	return run, nil
}

// ReceiveMessage handles an incoming collaboration message for an existing run.
func (e *Engine) ReceiveMessage(ctx context.Context, event protocol.CollaborationEvent) (protocol.CollaborationRun, error) {
	storage, err := e.Registry.DefaultStorage()
	if err != nil {
		return protocol.CollaborationRun{}, err
	}
	run, err := storage.ReadRun(ctx, event.Namespace, event.RunID)
	if err != nil {
		return protocol.CollaborationRun{}, err
	}
	if run.Status != protocol.RunStatusRunning {
		return run, fmt.Errorf("run %s is not running", run.RunID)
	}

	collab, err := e.Client.GetCollaboration(ctx, event.Namespace, run.CollaborationName)
	if err != nil {
		return protocol.CollaborationRun{}, err
	}
	members, groups, policies, err := e.loadMembersGroupsPolicies(ctx, event.Namespace)
	if err != nil {
		return protocol.CollaborationRun{}, err
	}
	resolver := organization.NewResolver(members, groups)
	evaluator := policy.NewEvaluator(policies, resolver)

	if err := e.processMessage(ctx, run, event, collab, evaluator); err != nil {
		return protocol.CollaborationRun{}, err
	}
	return storage.ReadRun(ctx, event.Namespace, event.RunID)
}

func (e *Engine) processMessage(ctx context.Context, run protocol.CollaborationRun, event protocol.CollaborationEvent, collab agentorgsv1alpha1.Collaboration, evaluator *policy.Evaluator) error {
	storage, err := e.Registry.DefaultStorage()
	if err != nil {
		return err
	}
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	if err := storage.WriteEvent(ctx, event); err != nil {
		return err
	}

	switch event.Type {
	case protocol.EventTypeMemberRequest, protocol.EventTypeMessage:
		action := protocol.PolicyActionContinue
		if event.Type == protocol.EventTypeMemberRequest {
			action = protocol.PolicyActionStart
		}
		for _, target := range event.Targets {
			if evaluator.Check(event.Source.Member, target, action) != policy.DecisionAllow {
				return fmt.Errorf("policy denied event from %s", event.Source.Member)
			}
		}
		run.Round++
		if run.Round > run.MaxReplyRounds {
			run.Status = protocol.RunStatusFailed
			return storage.WriteRun(ctx, run)
		}
		if err := storage.WriteRun(ctx, run); err != nil {
			return err
		}
		return e.dispatch(ctx, event, run.ResolvedTargets, collab)
	case protocol.EventTypeResult:
		run.Result = event.Payload
		if status, ok := event.Payload["status"].(string); ok && status == "failed" {
			run.Status = protocol.RunStatusFailed
		} else {
			run.Status = protocol.RunStatusCompleted
		}
		return storage.WriteRun(ctx, run)
	case protocol.EventTypeCancel:
		if evaluator.Check(event.Source.Member, protocol.ObjectTarget{Kind: agentorgsv1alpha1.MemberKind, Name: run.StartedBy}, protocol.PolicyActionCancel) != policy.DecisionAllow {
			return fmt.Errorf("policy denied cancel")
		}
		run.Status = protocol.RunStatusCancelled
		return storage.WriteRun(ctx, run)
	case protocol.EventTypeError:
		run.Status = protocol.RunStatusFailed
		return storage.WriteRun(ctx, run)
	default:
		return fmt.Errorf("unsupported event type %q", event.Type)
	}
}

// dispatch wakes resolved Agent members via visible Matrix @mentions in the Collaboration room.
func (e *Engine) dispatch(ctx context.Context, event protocol.CollaborationEvent, memberNames []string, collab agentorgsv1alpha1.Collaboration) error {
	mentions := make([]string, 0, len(memberNames))
	for _, name := range memberNames {
		member, err := e.Client.GetMember(ctx, event.Namespace, name)
		if err != nil {
			return err
		}
		if member.Spec.Type != agentorgsv1alpha1.MemberTypeAgent {
			continue
		}
		mxid := member.Status.MatrixUserID
		if mxid == "" {
			return fmt.Errorf("member %q has empty status.matrixUserId; cannot wake via Matrix mention", name)
		}
		mentions = append(mentions, mxid)
	}
	event.MentionUserIDs = mentions

	channel, err := e.collaborationChannel(collab)
	if err != nil {
		return err
	}
	return channel.Deliver(ctx, event)
}

func (e *Engine) collaborationChannel(collab agentorgsv1alpha1.Collaboration) (provider.CollaborationProvider, error) {
	name := collab.Spec.Channel.Provider
	if name == "" {
		return e.Registry.DefaultCollaboration()
	}
	return e.Registry.Collaboration(name)
}

func (e *Engine) loadMembersGroupsPolicies(ctx context.Context, namespace string) ([]agentorgsv1alpha1.Member, []agentorgsv1alpha1.Group, []agentorgsv1alpha1.Policy, error) {
	members, err := e.Client.ListMembers(ctx, namespace)
	if err != nil {
		return nil, nil, nil, err
	}
	groups, err := e.Client.ListGroups(ctx, namespace)
	if err != nil {
		return nil, nil, nil, err
	}
	policies, err := e.Client.ListPolicies(ctx, namespace)
	if err != nil {
		return nil, nil, nil, err
	}
	return members, groups, policies, nil
}

// GetCollaborationRun returns one collaboration run.
func (e *Engine) GetCollaborationRun(ctx context.Context, namespace, runID string) (protocol.CollaborationRun, error) {
	storage, err := e.Registry.DefaultStorage()
	if err != nil {
		return protocol.CollaborationRun{}, err
	}
	return storage.ReadRun(ctx, namespace, runID)
}

// ListCollaborationMessages returns messages recorded for one collaboration run.
func (e *Engine) ListCollaborationMessages(ctx context.Context, namespace, runID string) ([]protocol.CollaborationEvent, error) {
	storage, err := e.Registry.DefaultStorage()
	if err != nil {
		return nil, err
	}
	return storage.ListEvents(ctx, namespace, runID)
}
