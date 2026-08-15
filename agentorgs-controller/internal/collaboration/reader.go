package collaboration

import (
	"context"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// K8sReader implements Reader using the controller-runtime client.
type K8sReader struct {
	Client client.Client
}

func (r *K8sReader) ListMembers(ctx context.Context, namespace string) ([]agentorgsv1alpha1.Member, error) {
	var list agentorgsv1alpha1.MemberList
	if err := r.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *K8sReader) ListGroups(ctx context.Context, namespace string) ([]agentorgsv1alpha1.Group, error) {
	var list agentorgsv1alpha1.GroupList
	if err := r.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *K8sReader) ListPolicies(ctx context.Context, namespace string) ([]agentorgsv1alpha1.Policy, error) {
	var list agentorgsv1alpha1.PolicyList
	if err := r.Client.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (r *K8sReader) GetCollaboration(ctx context.Context, namespace, name string) (agentorgsv1alpha1.Collaboration, error) {
	var collab agentorgsv1alpha1.Collaboration
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &collab)
	return collab, err
}

func (r *K8sReader) GetMember(ctx context.Context, namespace, name string) (agentorgsv1alpha1.Member, error) {
	var member agentorgsv1alpha1.Member
	err := r.Client.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &member)
	return member, err
}
