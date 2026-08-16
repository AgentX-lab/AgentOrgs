package collaboration

import (
	"fmt"
	"sort"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/policy"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
)

// applyDispatchIntent filters expanded member names for this one request.
// OnlyTheseMembers (include) must already be in the expanded set or it errors.
func applyDispatchIntent(memberNames []string, intent protocol.DispatchIntent) ([]string, error) {
	inExpanded := make(map[string]struct{}, len(memberNames))
	for _, name := range memberNames {
		inExpanded[name] = struct{}{}
	}

	out := memberNames
	if len(intent.OnlyTheseMembers) > 0 {
		only := make([]string, 0, len(intent.OnlyTheseMembers))
		seen := map[string]struct{}{}
		for _, name := range intent.OnlyTheseMembers {
			if _, ok := inExpanded[name]; !ok {
				return nil, fmt.Errorf("include member %q is not in resolved targets", name)
			}
			if _, dup := seen[name]; dup {
				continue
			}
			seen[name] = struct{}{}
			only = append(only, name)
		}
		out = only
	}

	if len(intent.SkipTheseMembers) > 0 {
		skip := make(map[string]struct{}, len(intent.SkipTheseMembers))
		for _, name := range intent.SkipTheseMembers {
			skip[name] = struct{}{}
		}
		kept := make([]string, 0, len(out))
		for _, name := range out {
			if _, drop := skip[name]; drop {
				continue
			}
			kept = append(kept, name)
		}
		out = kept
	}

	sort.Strings(out)
	if len(out) == 0 {
		return nil, fmt.Errorf("no members left after dispatch intent")
	}
	return out, nil
}

// filterMembersByStartPolicy keeps members the starter is allowed to Start.
func filterMembersByStartPolicy(from string, memberNames []string, evaluator *policy.Evaluator) ([]string, error) {
	kept := make([]string, 0, len(memberNames))
	for _, name := range memberNames {
		target := protocol.ObjectTarget{Kind: agentorgsv1alpha1.MemberKind, Name: name}
		if evaluator.Check(from, target, protocol.PolicyActionStart) != policy.DecisionAllow {
			continue
		}
		kept = append(kept, name)
	}
	sort.Strings(kept)
	if len(kept) == 0 {
		return nil, fmt.Errorf("policy denied all resolved members for %s", from)
	}
	return kept, nil
}
