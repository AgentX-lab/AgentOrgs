package collaboration_test

import (
	"context"
	"strings"
	"testing"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStartCollaborationExcludeMember(t *testing.T) {
	_, _, _, engine := fixture()
	run, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
		protocol.DispatchIntent{SkipTheseMembers: []string{"be-2"}},
		map[string]interface{}{"text": "联调"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.ResolvedTargets) != 1 || run.ResolvedTargets[0] != "be-1" {
		t.Fatalf("ResolvedTargets=%v want [be-1]", run.ResolvedTargets)
	}
	if len(run.DispatchIntent.SkipTheseMembers) != 1 || run.DispatchIntent.SkipTheseMembers[0] != "be-2" {
		t.Fatalf("DispatchIntent=%v", run.DispatchIntent)
	}
}

func TestStartCollaborationIncludeMember(t *testing.T) {
	_, _, _, engine := fixture()
	run, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
		protocol.DispatchIntent{OnlyTheseMembers: []string{"be-2"}},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.ResolvedTargets) != 1 || run.ResolvedTargets[0] != "be-2" {
		t.Fatalf("ResolvedTargets=%v want [be-2]", run.ResolvedTargets)
	}
}

func TestStartCollaborationIncludeOutsideResolved(t *testing.T) {
	_, _, _, engine := fixture()
	_, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
		protocol.DispatchIntent{OnlyTheseMembers: []string{"qa-1"}},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "not in resolved targets") {
		t.Fatalf("expected include-outside error, got %v", err)
	}
}

func TestStartCollaborationExcludeLeavesNone(t *testing.T) {
	_, _, _, engine := fixture()
	_, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
		protocol.DispatchIntent{SkipTheseMembers: []string{"be-1", "be-2"}},
		nil,
	)
	if err == nil || !strings.Contains(err.Error(), "no members left") {
		t.Fatalf("expected empty-after-intent error, got %v", err)
	}
}

func TestStartCollaborationPolicyDenyMemberAfterExpand(t *testing.T) {
	reader, _, _, engine := fixture()
	reader.pols = []agentorgsv1alpha1.Policy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "allow-group"},
			Spec: agentorgsv1alpha1.PolicySpec{
				Effect:  agentorgsv1alpha1.PolicyEffectAllow,
				From:    []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.MemberKind, Name: "product-owner"}},
				To:      []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
				Actions: []agentorgsv1alpha1.PolicyAction{agentorgsv1alpha1.PolicyActionStart, agentorgsv1alpha1.PolicyActionContinue},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "deny-be-2"},
			Spec: agentorgsv1alpha1.PolicySpec{
				Effect:  agentorgsv1alpha1.PolicyEffectDeny,
				From:    []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.MemberKind, Name: "product-owner"}},
				To:      []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.MemberKind, Name: "be-2"}},
				Actions: []agentorgsv1alpha1.PolicyAction{agentorgsv1alpha1.PolicyActionStart},
			},
		},
	}
	run, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
		protocol.DispatchIntent{},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.ResolvedTargets) != 1 || run.ResolvedTargets[0] != "be-1" {
		t.Fatalf("ResolvedTargets=%v want [be-1] (be-2 denied)", run.ResolvedTargets)
	}
}

func TestStartCollaborationOnlyThisRole(t *testing.T) {
	reader, _, _, engine := fixture()
	reader.groups = []agentorgsv1alpha1.Group{{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-team"},
		Spec: agentorgsv1alpha1.GroupSpec{
			Members: []agentorgsv1alpha1.GroupMember{
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "be-1"}, Role: "Developer"},
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "be-2"}, Role: "Reviewer"},
			},
		},
	}}
	run, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
		protocol.DispatchIntent{OnlyThisRole: "Reviewer"},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(run.ResolvedTargets) != 1 || run.ResolvedTargets[0] != "be-2" {
		t.Fatalf("ResolvedTargets=%v want [be-2]", run.ResolvedTargets)
	}
}
