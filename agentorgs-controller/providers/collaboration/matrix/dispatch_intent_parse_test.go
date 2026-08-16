package matrix

import (
	"strings"
	"testing"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testMentionIndex() *organization.MentionIndex {
	members := []agentorgsv1alpha1.Member{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "be-1"},
			Status:     agentorgsv1alpha1.MemberStatus{MatrixUserID: "@be-1:matrix.local"},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "be-2"},
			Status:     agentorgsv1alpha1.MemberStatus{MatrixUserID: "@be-2:matrix.local"},
		},
	}
	groups := []agentorgsv1alpha1.Group{{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-team"},
		Status:     agentorgsv1alpha1.GroupStatus{MatrixUserID: "@backend-team:matrix.local"},
	}}
	return organization.NewMentionIndex(members, groups)
}

func TestParseDispatchIntentFromBodyLocalpart(t *testing.T) {
	intent, err := parseDispatchIntentFromBody(
		"@backend-team:matrix.local -@be-2 please sync",
		testMentionIndex(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(intent.SkipTheseMembers) != 1 || intent.SkipTheseMembers[0] != "be-2" {
		t.Fatalf("intent=%v", intent)
	}
}

func TestParseDispatchIntentFromBodyFullMXID(t *testing.T) {
	intent, err := parseDispatchIntentFromBody(
		"@backend-team -@be-2:matrix.local fix auth",
		testMentionIndex(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(intent.SkipTheseMembers) != 1 || intent.SkipTheseMembers[0] != "be-2" {
		t.Fatalf("intent=%v", intent)
	}
}

func TestParseDispatchIntentNoExclude(t *testing.T) {
	intent, err := parseDispatchIntentFromBody("@backend-team hello", testMentionIndex())
	if err != nil {
		t.Fatal(err)
	}
	if !intent.Empty() {
		t.Fatalf("want empty intent, got %#v", intent)
	}
}

func TestDropSkippedTargets(t *testing.T) {
	targets := []protocol.ObjectTarget{
		{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"},
		{Kind: agentorgsv1alpha1.MemberKind, Name: "be-2"},
	}
	intent := protocol.DispatchIntent{SkipTheseMembers: []string{"be-2"}}
	out := dropSkippedTargets(targets, intent)
	if len(out) != 1 || out[0].Name != "backend-team" {
		t.Fatalf("out=%v", out)
	}
}

func TestParseDispatchIntentUnknownToken(t *testing.T) {
	_, err := parseDispatchIntentFromBody("-@nobody sync", testMentionIndex())
	if err == nil || !strings.Contains(err.Error(), "nobody") {
		t.Fatalf("expected unknown token error, got %v", err)
	}
}
