package policy_test

import (
	"testing"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/policy"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEvaluatorAllow(t *testing.T) {
	members := []agentorgsv1alpha1.Member{
		{ObjectMeta: metav1.ObjectMeta{Name: "product-owner"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "developer"}},
	}
	groups := []agentorgsv1alpha1.Group{{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-team"},
		Spec: agentorgsv1alpha1.GroupSpec{
			Members: []agentorgsv1alpha1.GroupMember{
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "product-owner"}},
				{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "developer"}},
			},
		},
	}}
	groups[0].Name = "backend-team"

	policies := []agentorgsv1alpha1.Policy{{
		ObjectMeta: metav1.ObjectMeta{Name: "backend-permission"},
		Spec: agentorgsv1alpha1.PolicySpec{
			Effect: agentorgsv1alpha1.PolicyEffectAllow,
			From:   []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
			To:     []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.MemberKind, Name: "developer"}},
			Actions: []agentorgsv1alpha1.PolicyAction{agentorgsv1alpha1.PolicyActionStart},
		},
	}}

	resolver := organization.NewResolver(members, groups)
	evaluator := policy.NewEvaluator(policies, resolver)
	decision := evaluator.Check("product-owner", protocol.ObjectTarget{Kind: agentorgsv1alpha1.MemberKind, Name: "developer"}, protocol.PolicyActionStart)
	if decision != policy.DecisionAllow {
		t.Fatalf("expected allow, got %s", decision)
	}
}
