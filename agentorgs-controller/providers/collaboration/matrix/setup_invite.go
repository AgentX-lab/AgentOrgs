package matrix

import (
	"sort"
	"strings"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
)

// collaborationInviteUserIDs is the Matrix users who should be in this
// Collaboration's room: direct Member participants, Group identities so
// @Group works, and every Member after Group expansion.
func collaborationInviteUserIDs(
	collab agentorgsv1alpha1.Collaboration,
	resolver *organization.Resolver,
	memberMXID map[string]string,
	groupMXID map[string]string,
) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0)
	add := func(userID string) {
		if userID == "" {
			return
		}
		if _, ok := seen[userID]; ok {
			return
		}
		seen[userID] = struct{}{}
		out = append(out, userID)
	}
	for _, p := range collab.Spec.Participants {
		switch p.Who.Kind {
		case agentorgsv1alpha1.MemberKind, "":
			add(memberMXID[p.Who.Name])
		case agentorgsv1alpha1.GroupKind:
			add(groupMXID[p.Who.Name])
			expanded, err := resolver.ExpandGroup(p.Who.Name)
			if err != nil {
				continue
			}
			for _, m := range expanded {
				add(memberMXID[m.Name])
			}
		}
	}
	sort.Strings(out)
	return out
}

func alreadyInRoomError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already in the room") || strings.Contains(msg, "already joined")
}
