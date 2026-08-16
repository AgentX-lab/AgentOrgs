package matrix

import (
	"fmt"
	"regexp"
	"sort"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
)

// skipMentionPattern finds -@token in Matrix message bodies (exclude intent).
var skipMentionPattern = regexp.MustCompile(`-@([^\s]+)`)

// parseDispatchIntentFromBody turns "-@be-2 ..." into SkipTheseMembers.
// Positive @targets stay in m.mentions; -@ must not be treated as a wake target.
func parseDispatchIntentFromBody(body string, index *organization.MentionIndex) (protocol.DispatchIntent, error) {
	matches := skipMentionPattern.FindAllStringSubmatch(body, -1)
	if len(matches) == 0 {
		return protocol.DispatchIntent{}, nil
	}
	skip := map[string]struct{}{}
	for _, m := range matches {
		token := m[1]
		target, err := index.ResolveMentionToken(token)
		if err != nil {
			return protocol.DispatchIntent{}, err
		}
		if target.Kind != agentorgsv1alpha1.MemberKind {
			return protocol.DispatchIntent{}, fmt.Errorf(
				"exclude -@%s resolved to %s/%s; only Member is allowed", token, target.Kind, target.Name)
		}
		skip[target.Name] = struct{}{}
	}
	names := make([]string, 0, len(skip))
	for name := range skip {
		names = append(names, name)
	}
	sort.Strings(names)
	return protocol.DispatchIntent{SkipTheseMembers: names}, nil
}

// dropSkippedTargets removes excluded members from positive @ targets.
func dropSkippedTargets(targets []protocol.ObjectTarget, intent protocol.DispatchIntent) []protocol.ObjectTarget {
	if len(intent.SkipTheseMembers) == 0 {
		return targets
	}
	skip := map[string]struct{}{}
	for _, name := range intent.SkipTheseMembers {
		skip[name] = struct{}{}
	}
	out := make([]protocol.ObjectTarget, 0, len(targets))
	for _, t := range targets {
		if t.Kind == agentorgsv1alpha1.MemberKind {
			if _, ok := skip[t.Name]; ok {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}
