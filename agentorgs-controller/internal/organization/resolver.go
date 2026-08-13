package organization

import (
	"fmt"
	"strings"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
)

// MemberRole pairs a member name with a role in a group context.
type MemberRole struct {
	Name string
	Role string
}

// Resolver expands groups and routes group targets to concrete members.
type Resolver struct {
	members map[string]agentorgsv1alpha1.Member
	groups  map[string]agentorgsv1alpha1.Group
}

func NewResolver(members []agentorgsv1alpha1.Member, groups []agentorgsv1alpha1.Group) *Resolver {
	r := &Resolver{
		members: map[string]agentorgsv1alpha1.Member{},
		groups:  map[string]agentorgsv1alpha1.Group{},
	}
	for _, m := range members {
		r.members[m.Name] = m
	}
	for _, g := range groups {
		r.groups[g.Name] = g
	}
	return r
}

func (r *Resolver) ExpandGroup(groupName string) ([]MemberRole, error) {
	visited := map[string]struct{}{}
	return r.expandGroup(groupName, visited)
}

func (r *Resolver) expandGroup(groupName string, visited map[string]struct{}) ([]MemberRole, error) {
	if _, ok := visited[groupName]; ok {
		return nil, fmt.Errorf("group cycle detected at %q", groupName)
	}
	visited[groupName] = struct{}{}

	group, ok := r.groups[groupName]
	if !ok {
		return nil, fmt.Errorf("group %q not found", groupName)
	}

	var out []MemberRole
	seen := map[string]struct{}{}
	for _, item := range group.Spec.Members {
		switch item.Who.Kind {
		case agentorgsv1alpha1.MemberKind:
			if _, dup := seen[item.Who.Name]; dup {
				continue
			}
			seen[item.Who.Name] = struct{}{}
			out = append(out, MemberRole{Name: item.Who.Name, Role: item.Role})
		case agentorgsv1alpha1.GroupKind:
			nested, err := r.expandGroup(item.Who.Name, visited)
			if err != nil {
				return nil, err
			}
			for _, m := range nested {
				if _, dup := seen[m.Name]; dup {
					continue
				}
				seen[m.Name] = struct{}{}
				out = append(out, m)
			}
		default:
			return nil, fmt.Errorf("unsupported member kind %q in group %q", item.Who.Kind, groupName)
		}
	}
	return out, nil
}

func (r *Resolver) ResolveTargets(strategy agentorgsv1alpha1.GroupTargetStrategy, role string, target agentorgsv1alpha1.ObjectRef) ([]string, error) {
	switch target.Kind {
	case agentorgsv1alpha1.MemberKind:
		return []string{target.Name}, nil
	case agentorgsv1alpha1.GroupKind:
		members, err := r.ExpandGroup(target.Name)
		if err != nil {
			return nil, err
		}
		switch strategy {
		case agentorgsv1alpha1.GroupTargetAll, "":
			return names(members), nil
		case agentorgsv1alpha1.GroupTargetAny:
			if len(members) == 0 {
				return nil, fmt.Errorf("group %q has no members", target.Name)
			}
			return []string{members[0].Name}, nil
		case agentorgsv1alpha1.GroupTargetRole:
			var matched []string
			for _, m := range members {
				if strings.EqualFold(m.Role, role) {
					matched = append(matched, m.Name)
				}
			}
			if len(matched) == 0 {
				return nil, fmt.Errorf("no members with role %q in group %q", role, target.Name)
			}
			return matched, nil
		case agentorgsv1alpha1.GroupTargetLeader:
			if len(members) == 0 {
				return nil, fmt.Errorf("group %q has no members", target.Name)
			}
			leader := members[0]
			for _, m := range members {
				if strings.EqualFold(m.Role, "Leader") {
					leader = m
					break
				}
			}
			return []string{leader.Name}, nil
		default:
			return nil, fmt.Errorf("unsupported group target strategy %q", strategy)
		}
	default:
		return nil, fmt.Errorf("unsupported target kind %q", target.Kind)
	}
}

func names(members []MemberRole) []string {
	out := make([]string, 0, len(members))
	for _, m := range members {
		out = append(out, m.Name)
	}
	return out
}
