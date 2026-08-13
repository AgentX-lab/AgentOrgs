package policy

import (
	"strings"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
)

// Decision is the result of a policy check.
type Decision string

const (
	DecisionAllow Decision = "Allow"
	DecisionDeny  Decision = "Deny"
)

// Evaluator applies Policy resources to collaboration actions.
type Evaluator struct {
	policies []agentorgsv1alpha1.Policy
	resolver *organization.Resolver
}

func NewEvaluator(policies []agentorgsv1alpha1.Policy, resolver *organization.Resolver) *Evaluator {
	return &Evaluator{policies: policies, resolver: resolver}
}

func (e *Evaluator) Check(sourceMember string, target protocol.ObjectTarget, action string) Decision {
	if len(e.policies) == 0 {
		return DecisionDeny
	}

	allowed := false
	for _, pol := range e.policies {
		if !matchesAction(pol, action) {
			continue
		}
		if !e.matchesSource(pol, sourceMember) {
			continue
		}
		if !e.matchesTarget(pol, target) {
			continue
		}
		switch pol.Spec.Effect {
		case agentorgsv1alpha1.PolicyEffectDeny:
			return DecisionDeny
		case agentorgsv1alpha1.PolicyEffectAllow:
			allowed = true
		}
	}
	if allowed {
		return DecisionAllow
	}
	return DecisionDeny
}

func matchesAction(pol agentorgsv1alpha1.Policy, action string) bool {
	if len(pol.Spec.Actions) == 0 {
		return true
	}
	for _, a := range pol.Spec.Actions {
		if string(a) == action {
			return true
		}
	}
	return false
}

func (e *Evaluator) matchesSource(pol agentorgsv1alpha1.Policy, sourceMember string) bool {
	if len(pol.Spec.From) == 0 {
		return true
	}
	for _, ref := range pol.Spec.From {
		switch ref.Kind {
		case agentorgsv1alpha1.MemberKind:
			if ref.Name == sourceMember {
				return true
			}
		case agentorgsv1alpha1.GroupKind:
			members, err := e.resolver.ExpandGroup(ref.Name)
			if err != nil {
				continue
			}
			for _, m := range members {
				if m.Name == sourceMember {
					return true
				}
			}
		}
	}
	return false
}

func (e *Evaluator) matchesTarget(pol agentorgsv1alpha1.Policy, target protocol.ObjectTarget) bool {
	if len(pol.Spec.To) == 0 {
		return true
	}
	for _, ref := range pol.Spec.To {
		if ref.Kind == target.Kind && ref.Name == target.Name {
			return true
		}
		if ref.Kind == agentorgsv1alpha1.GroupKind && target.Kind == agentorgsv1alpha1.MemberKind {
			members, err := e.resolver.ExpandGroup(ref.Name)
			if err != nil {
				continue
			}
			for _, m := range members {
				if strings.EqualFold(m.Name, target.Name) {
					return true
				}
			}
		}
	}
	return false
}
