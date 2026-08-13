package organization_test

import (
	"testing"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestExpandGroup(t *testing.T) {
	members := []agentorgsv1alpha1.Member{
		{ObjectMeta: metav1.ObjectMeta{Name: "developer"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "reviewer"}},
	}

	groups := []agentorgsv1alpha1.Group{{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-team"},
		Spec: agentorgsv1alpha1.GroupSpec{
			Members: []agentorgsv1alpha1.GroupMember{
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "developer"}, Role: "Developer"},
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "reviewer"}, Role: "Reviewer"},
			},
		},
	}}
	resolver := organization.NewResolver(members, groups)
	expanded, err := resolver.ExpandGroup("backend-team")
	if err != nil {
		t.Fatalf("expand group: %v", err)
	}
	if len(expanded) != 2 {
		t.Fatalf("expected 2 members, got %d", len(expanded))
	}
}

func TestResolveTargetsByRole(t *testing.T) {
	members := []agentorgsv1alpha1.Member{
		{ObjectMeta: metav1.ObjectMeta{Name: "developer"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "reviewer"}},
	}
	groups := []agentorgsv1alpha1.Group{{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-team"},
		Spec: agentorgsv1alpha1.GroupSpec{
			Members: []agentorgsv1alpha1.GroupMember{
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "developer"}, Role: "Developer"},
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "reviewer"}, Role: "Reviewer"},
			},
		},
	}}
	resolver := organization.NewResolver(members, groups)
	targets, err := resolver.ResolveTargets(agentorgsv1alpha1.GroupTargetRole, "Developer", agentorgsv1alpha1.ObjectRef{
		Kind: agentorgsv1alpha1.GroupKind,
		Name: "backend-team",
	})
	if err != nil {
		t.Fatalf("resolve targets: %v", err)
	}
	if len(targets) != 1 || targets[0] != "developer" {
		t.Fatalf("unexpected targets: %#v", targets)
	}
}
