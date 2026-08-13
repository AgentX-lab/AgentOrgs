package organization_test

import (
	"testing"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMentionIndexResolveMany(t *testing.T) {
	members := []agentorgsv1alpha1.Member{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "architect"},
			Status:     agentorgsv1alpha1.MemberStatus{MatrixUserID: "@architect:matrix-local.agentorgs.io"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "product-owner"},
			Status:     agentorgsv1alpha1.MemberStatus{MatrixUserID: "@product-owner:matrix-local.agentorgs.io"},
		},
	}
	groups := []agentorgsv1alpha1.Group{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "backend-team"},
			Status:     agentorgsv1alpha1.GroupStatus{MatrixUserID: "@backend-team:matrix-local.agentorgs.io"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "qa-team"},
			Status:     agentorgsv1alpha1.GroupStatus{MatrixUserID: "@qa-team:matrix-local.agentorgs.io"},
		},
	}

	idx := organization.NewMentionIndex(members, groups)
	targets, err := idx.ResolveMany([]string{
		"@backend-team:matrix-local.agentorgs.io",
		"@QA-TEAM:matrix-local.agentorgs.io",
		"@architect:matrix-local.agentorgs.io",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 3 {
		t.Fatalf("got %d targets, want 3", len(targets))
	}
	if targets[0].Kind != agentorgsv1alpha1.GroupKind || targets[0].Name != "backend-team" {
		t.Fatalf("target[0]=%+v", targets[0])
	}
	if targets[1].Kind != agentorgsv1alpha1.GroupKind || targets[1].Name != "qa-team" {
		t.Fatalf("target[1]=%+v", targets[1])
	}
	if targets[2].Kind != agentorgsv1alpha1.MemberKind || targets[2].Name != "architect" {
		t.Fatalf("target[2]=%+v", targets[2])
	}
}

func TestMentionIndexUnknown(t *testing.T) {
	idx := organization.NewMentionIndex(nil, nil)
	if _, err := idx.Resolve("@nobody:matrix-local.agentorgs.io"); err == nil {
		t.Fatal("expected error for unknown mxid")
	}
}
