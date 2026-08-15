package workspace

import "fmt"

// CommonSkill is always seeded for Agent Members.
const CommonSkill = "workspace-sync"

// builtinPacks maps Member.spec.skillPack to builtin skill directory names.
var builtinPacks = map[string][]string{
	"coordination": {"team-coordination"},
	"backend":      {"backend-dev"},
	"qa":           {"qa-test"},
}

// ResolveSkills expands skillPack + extras into an ordered, de-duplicated skill list.
// Unknown pack names return an error. Empty pack yields only CommonSkill (+ extras).
func ResolveSkills(skillPack string, extra []string) ([]string, error) {
	out := []string{CommonSkill}
	if skillPack != "" {
		skills, ok := builtinPacks[skillPack]
		if !ok {
			return nil, fmt.Errorf("unknown skillPack %q (want one of: coordination, backend, qa)", skillPack)
		}
		out = append(out, skills...)
	}
	out = append(out, extra...)
	return dedupeStrings(out), nil
}

func dedupeStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
