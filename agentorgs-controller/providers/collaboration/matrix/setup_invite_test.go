package matrix

import (
	"fmt"
	"testing"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCollaborationInviteUserIDsGrowsWithGroup(t *testing.T) {
	members := []agentorgsv1alpha1.Member{
		{ObjectMeta: metav1.ObjectMeta{Name: "requester"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "lead"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "extra"}},
	}
	groups := []agentorgsv1alpha1.Group{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "team"},
			Spec: agentorgsv1alpha1.GroupSpec{
				Members: []agentorgsv1alpha1.GroupMember{
					{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "lead"}, Role: "Leader"},
					{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "worker"}},
				},
			},
		},
	}
	collab := agentorgsv1alpha1.Collaboration{
		Spec: agentorgsv1alpha1.CollaborationSpec{
			Participants: []agentorgsv1alpha1.CollaborationParticipant{
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "requester"}},
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.GroupKind, Name: "team"}},
			},
		},
	}
	memberMXID := map[string]string{
		"requester": "@requester:matrix.local",
		"lead":      "@lead:matrix.local",
		"worker":    "@worker:matrix.local",
		"extra":     "@extra:matrix.local",
	}
	groupMXID := map[string]string{"team": "@team:matrix.local"}

	got := collaborationInviteUserIDs(collab, organization.NewResolver(members, groups), memberMXID, groupMXID)
	want := []string{"@lead:matrix.local", "@requester:matrix.local", "@team:matrix.local", "@worker:matrix.local"}
	if len(got) != len(want) {
		t.Fatalf("invite=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("invite=%v want %v", got, want)
		}
	}

	groups[0].Spec.Members = append(groups[0].Spec.Members, agentorgsv1alpha1.GroupMember{
		Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "extra"},
	})
	got = collaborationInviteUserIDs(collab, organization.NewResolver(members, groups), memberMXID, groupMXID)
	want = []string{"@extra:matrix.local", "@lead:matrix.local", "@requester:matrix.local", "@team:matrix.local", "@worker:matrix.local"}
	if len(got) != len(want) {
		t.Fatalf("after group add invite=%v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("after group add invite=%v want %v", got, want)
		}
	}
}

func TestAlreadyInRoomError(t *testing.T) {
	if alreadyInRoomError(nil) {
		t.Fatal("nil should not match")
	}
	err := fmt.Errorf("matrix invite: HTTP 403 (@lead:matrix.local is already in the room.)")
	if !alreadyInRoomError(err) {
		t.Fatalf("want already-in-room for %v", err)
	}
	if alreadyInRoomError(fmt.Errorf("matrix invite: HTTP 403 (forbidden)")) {
		t.Fatal("generic 403 should not match")
	}
}
