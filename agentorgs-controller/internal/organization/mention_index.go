package organization

import (
	"fmt"
	"strings"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
)

// MentionIndex maps a Matrix user id (@local:domain) to a Member or Group.
//
// Matrix only knows users. AgentOrgs Groups need their own Matrix identity
// so people can @backend-team in a room. This index is how we translate
// those MXIDs back into CR names.
type MentionIndex struct {
	byMXID map[string]protocol.ObjectTarget
}

// NewMentionIndex builds an index from Member/Group status.matrixUserId.
func NewMentionIndex(members []agentorgsv1alpha1.Member, groups []agentorgsv1alpha1.Group) *MentionIndex {
	idx := &MentionIndex{byMXID: map[string]protocol.ObjectTarget{}}
	for _, m := range members {
		if mxid := normalizeMXID(m.Status.MatrixUserID); mxid != "" {
			idx.byMXID[mxid] = protocol.ObjectTarget{
				Kind: agentorgsv1alpha1.MemberKind,
				Name: m.Name,
			}
		}
	}
	for _, g := range groups {
		if mxid := normalizeMXID(g.Status.MatrixUserID); mxid != "" {
			idx.byMXID[mxid] = protocol.ObjectTarget{
				Kind: agentorgsv1alpha1.GroupKind,
				Name: g.Name,
			}
		}
	}
	return idx
}

// Resolve turns one Matrix MXID into a Member or Group target.
func (i *MentionIndex) Resolve(mxid string) (protocol.ObjectTarget, error) {
	key := normalizeMXID(mxid)
	if key == "" {
		return protocol.ObjectTarget{}, fmt.Errorf("empty matrix user id")
	}
	target, ok := i.byMXID[key]
	if !ok {
		return protocol.ObjectTarget{}, fmt.Errorf("no Member or Group for matrix user %q", key)
	}
	return target, nil
}

// ResolveMany resolves several MXIDs. Unknown ids return an error.
func (i *MentionIndex) ResolveMany(mxids []string) ([]protocol.ObjectTarget, error) {
	out := make([]protocol.ObjectTarget, 0, len(mxids))
	seen := map[string]struct{}{}
	for _, mxid := range mxids {
		target, err := i.Resolve(mxid)
		if err != nil {
			return nil, err
		}
		key := target.Kind + "/" + target.Name
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, target)
	}
	return out, nil
}

func normalizeMXID(mxid string) string {
	return strings.ToLower(strings.TrimSpace(mxid))
}
