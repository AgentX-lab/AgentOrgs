package openclaw

import (
	"context"
	"fmt"
	"sort"
	"strings"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// collaborationRoomIDsForMember returns real Matrix room IDs for Collaborations
// that include this Member (directly or via Group expansion).
//
// OpenClaw only treats a room as non-DM when channels.matrix.groups has an
// exact room key (wildcard "*" does not override DM classification). Pods
// must wait until those IDs exist so openclaw.json is correct before start.
func (a *Adapter) collaborationRoomIDsForMember(ctx context.Context, namespace, memberName string) ([]string, error) {
	if a.K8s == nil {
		return nil, fmt.Errorf("openclaw adapter: kubernetes client is required")
	}

	var collabs agentorgsv1alpha1.CollaborationList
	if err := a.K8s.List(ctx, &collabs, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	var members agentorgsv1alpha1.MemberList
	if err := a.K8s.List(ctx, &members, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	var groups agentorgsv1alpha1.GroupList
	if err := a.K8s.List(ctx, &groups, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	resolver := organization.NewResolver(members.Items, groups.Items)

	roomIDs := make([]string, 0)
	seen := map[string]struct{}{}
	var pending []string

	for i := range collabs.Items {
		c := &collabs.Items[i]
		if c.Spec.Channel.Provider != "" && c.Spec.Channel.Provider != "matrix" {
			continue
		}
		inCollab, err := memberInCollaboration(resolver, c, memberName)
		if err != nil {
			return nil, err
		}
		if !inCollab {
			continue
		}
		roomID, err := readChannelRoomID(ctx, a.K8s, namespace, c)
		if err != nil {
			return nil, err
		}
		if !isUsableRoomID(roomID) {
			pending = append(pending, c.Name)
			continue
		}
		if _, ok := seen[roomID]; ok {
			continue
		}
		seen[roomID] = struct{}{}
		roomIDs = append(roomIDs, roomID)
	}

	if len(pending) > 0 {
		sort.Strings(pending)
		return nil, fmt.Errorf("collaboration room(s) not ready for member %q: %s", memberName, strings.Join(pending, ", "))
	}
	sort.Strings(roomIDs)
	return roomIDs, nil
}

func memberInCollaboration(resolver *organization.Resolver, c *agentorgsv1alpha1.Collaboration, memberName string) (bool, error) {
	for _, p := range c.Spec.Participants {
		switch p.Who.Kind {
		case agentorgsv1alpha1.MemberKind, "":
			if p.Who.Name == memberName {
				return true, nil
			}
		case agentorgsv1alpha1.GroupKind:
			expanded, err := resolver.ExpandGroup(p.Who.Name)
			if err != nil {
				return false, err
			}
			for _, m := range expanded {
				if m.Name == memberName {
					return true, nil
				}
			}
		}
	}
	return false, nil
}

func readChannelRoomID(ctx context.Context, c client.Client, namespace string, collab *agentorgsv1alpha1.Collaboration) (string, error) {
	cmName := collab.Spec.Channel.Config.Name
	if cmName == "" {
		return "", nil
	}
	cmKey := collab.Spec.Channel.Config.Key
	if cmKey == "" {
		cmKey = "roomId"
	}
	var cm corev1.ConfigMap
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: cmName}, &cm); err != nil {
		return "", err
	}
	if cm.Data == nil {
		return "", nil
	}
	return cm.Data[cmKey], nil
}

func isUsableRoomID(roomID string) bool {
	if roomID == "" {
		return false
	}
	// Fixture placeholders use example.org until Matrix Setup replaces them.
	return !strings.Contains(roomID, "example.org")
}
