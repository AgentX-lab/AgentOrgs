package controller

import (
	"context"
	"fmt"
	"time"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const finalizerName = "agentorgs.io/finalizer"

func setCondition(conditions []agentorgsv1alpha1.Condition, condType, reason, message string, status metav1.ConditionStatus) []agentorgsv1alpha1.Condition {
	now := metav1.Now()
	for i := range conditions {
		if conditions[i].Type == condType {
			conditions[i].Status = status
			conditions[i].Reason = reason
			conditions[i].Message = message
			conditions[i].LastTransitionTime = now
			return conditions
		}
	}
	return append(conditions, agentorgsv1alpha1.Condition{
		Type:               condType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

// MemberReconciler reconciles Member resources.
type MemberReconciler struct {
	client.Client
	Registry *provider.Registry
}

func (r *MemberReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var member agentorgsv1alpha1.Member
	if err := r.Get(ctx, req.NamespacedName, &member); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if member.DeletionTimestamp != nil {
		return r.finalize(ctx, member)
	}
	if !containsString(member.Finalizers, finalizerName) {
		member.Finalizers = append(member.Finalizers, finalizerName)
		if err := r.Update(ctx, &member); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	member.Status.AppliedConfigVersion = member.Generation
	runtimeReported := member.Annotations[agentorgsv1alpha1.MemberRuntimeReadyAnnotation] == "true"
	_ = r.resolveMatrixUserID(ctx, &member)

	if member.Spec.Type != agentorgsv1alpha1.MemberTypeAgent {
		member.Status.Phase = agentorgsv1alpha1.MemberPhaseReady
		member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "Reconciled", "Non-agent member is ready", metav1.ConditionTrue)
		return ctrl.Result{}, r.Status().Update(ctx, &member)
	}

	if member.Spec.Runtime == nil {
		member.Status.Phase = agentorgsv1alpha1.MemberPhaseNotReady
		member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "MissingRuntime", "Agent members require runtime binding", metav1.ConditionFalse)
		return ctrl.Result{}, r.Status().Update(ctx, &member)
	}

	storage, err := r.Registry.DefaultStorage()
	if err != nil {
		member.Status.Phase = agentorgsv1alpha1.MemberPhaseNotReady
		member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "StorageMissing", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, r.Status().Update(ctx, &member)
	}
	displayName := member.Spec.DisplayName
	if displayName == "" {
		displayName = member.Name
	}
	if err := storage.EnsureMemberWorkspace(ctx, member.Namespace, member.Name, displayName); err != nil {
		member.Status.Phase = agentorgsv1alpha1.MemberPhaseNotReady
		member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "WorkspaceInitFailed", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{RequeueAfter: 15 * time.Second}, r.Status().Update(ctx, &member)
	}

	// Runtime config (e.g. openclaw.json + Matrix channel) before starting the Pod.
	if member.Spec.Runtime != nil {
		runtime, err := r.Registry.Runtime(member.Spec.Runtime.Provider)
		if err != nil {
			member.Status.Phase = agentorgsv1alpha1.MemberPhaseNotReady
			member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "RuntimeProviderMissing", err.Error(), metav1.ConditionFalse)
			return ctrl.Result{}, r.Status().Update(ctx, &member)
		}
		if err := runtime.Apply(ctx, memberContext(member)); err != nil {
			member.Status.Phase = agentorgsv1alpha1.MemberPhaseNotReady
			member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "RuntimeFailed", err.Error(), metav1.ConditionFalse)
			return ctrl.Result{RequeueAfter: 15 * time.Second}, r.Status().Update(ctx, &member)
		}
		member.Status.Connections.Runtime = member.Spec.Runtime.Provider
	}

	if member.Spec.Execution != nil {
		exec, err := r.Registry.Execution(member.Spec.Execution.Provider)
		if err != nil {
			member.Status.Phase = agentorgsv1alpha1.MemberPhaseNotReady
			member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "ExecutionProviderMissing", err.Error(), metav1.ConditionFalse)
			return ctrl.Result{}, r.Status().Update(ctx, &member)
		}
		ref, err := exec.Apply(ctx, memberContext(member))
		if err != nil {
			member.Status.Phase = agentorgsv1alpha1.MemberPhaseNotReady
			member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "ExecutionFailed", err.Error(), metav1.ConditionFalse)
			return ctrl.Result{RequeueAfter: 30 * time.Second}, r.Status().Update(ctx, &member)
		}
		member.Status.Connections.Execution = ref.Ref
	}

	if runtimeReported {
		member.Status.Phase = agentorgsv1alpha1.MemberPhaseReady
		member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "RuntimeReported", "runtime reported ready", metav1.ConditionTrue)
	} else {
		member.Status.Phase = agentorgsv1alpha1.MemberPhaseWaiting
		member.Status.StatusDetails = setCondition(member.Status.StatusDetails, "Ready", "WaitingRuntime", "waiting for runtime report-ready", metav1.ConditionFalse)
	}
	if err := r.Status().Update(ctx, &member); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("member reconciled", "name", member.Name, "phase", member.Status.Phase)
	return ctrl.Result{}, nil
}

func (r *MemberReconciler) finalize(ctx context.Context, member agentorgsv1alpha1.Member) (ctrl.Result, error) {
	if !containsString(member.Finalizers, finalizerName) {
		return ctrl.Result{}, nil
	}
	memberCtx := memberContext(member)
	if member.Spec.Execution != nil {
		if exec, err := r.Registry.Execution(member.Spec.Execution.Provider); err == nil {
			_ = exec.Delete(ctx, memberCtx)
		}
	}
	if member.Spec.Runtime != nil {
		if runtime, err := r.Registry.Runtime(member.Spec.Runtime.Provider); err == nil {
			_ = runtime.Delete(ctx, memberCtx)
		}
	}
	member.Finalizers = removeString(member.Finalizers, finalizerName)
	return ctrl.Result{}, r.Update(ctx, &member)
}

func memberContext(member agentorgsv1alpha1.Member) provider.MemberContext {
	return provider.MemberContext{
		Namespace: member.Namespace,
		Name:      member.Name,
		Spec:      member.Spec,
	}
}

// resolveMatrixUserID copies the Matrix MXID from the Member's matrix channel ConfigMap into status.
func (r *MemberReconciler) resolveMatrixUserID(ctx context.Context, member *agentorgsv1alpha1.Member) error {
	for _, ch := range member.Spec.Channels {
		if ch.Provider != "matrix" {
			continue
		}
		key := ch.Config.Key
		if key == "" {
			key = "userId"
		}
		var cm corev1.ConfigMap
		if err := r.Get(ctx, client.ObjectKey{Namespace: member.Namespace, Name: ch.Config.Name}, &cm); err != nil {
			return err
		}
		if cm.Data == nil {
			return nil
		}
		if v := cm.Data[key]; v != "" {
			member.Status.MatrixUserID = v
		}
		return nil
	}
	return nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func removeString(items []string, target string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != target {
			out = append(out, item)
		}
	}
	return out
}

// GroupReconciler reconciles Group resources.
type GroupReconciler struct {
	client.Client
}

func (r *GroupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var group agentorgsv1alpha1.Group
	if err := r.Get(ctx, req.NamespacedName, &group); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	group.Status.AppliedConfigVersion = group.Generation
	group.Status.Phase = agentorgsv1alpha1.GroupPhaseWaiting
	_ = r.resolveMatrixUserID(ctx, &group)

	var memberList agentorgsv1alpha1.MemberList
	if err := r.List(ctx, &memberList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	members := memberList.Items

	var groupList agentorgsv1alpha1.GroupList
	if err := r.List(ctx, &groupList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	groups := groupList.Items

	resolver := organization.NewResolver(members, groups)
	expanded, err := resolver.ExpandGroup(group.Name)
	if err != nil {
		group.Status.Phase = agentorgsv1alpha1.GroupPhaseInvalid
		group.Status.StatusDetails = setCondition(group.Status.StatusDetails, "Ready", "InvalidGroup", err.Error(), metav1.ConditionFalse)
		return ctrl.Result{}, r.Status().Update(ctx, &group)
	}

	for _, item := range group.Spec.Members {
		if item.Who.Kind == agentorgsv1alpha1.MemberKind {
			if err := r.Get(ctx, client.ObjectKey{Namespace: req.Namespace, Name: item.Who.Name}, &agentorgsv1alpha1.Member{}); err != nil {
				group.Status.Phase = agentorgsv1alpha1.GroupPhaseInvalid
				group.Status.StatusDetails = setCondition(group.Status.StatusDetails, "Ready", "MissingMember", fmt.Sprintf("member %q not found", item.Who.Name), metav1.ConditionFalse)
				return ctrl.Result{}, r.Status().Update(ctx, &group)
			}
		}
	}

	group.Status.Phase = agentorgsv1alpha1.GroupPhaseReady
	group.Status.MemberCount = len(expanded)
	group.Status.StatusDetails = setCondition(group.Status.StatusDetails, "Ready", "Reconciled", "Group is ready", metav1.ConditionTrue)
	return ctrl.Result{}, r.Status().Update(ctx, &group)
}

func (r *GroupReconciler) resolveMatrixUserID(ctx context.Context, group *agentorgsv1alpha1.Group) error {
	for _, ch := range group.Spec.Channels {
		if ch.Provider != "matrix" {
			continue
		}
		key := ch.Config.Key
		if key == "" {
			key = "userId"
		}
		var cm corev1.ConfigMap
		if err := r.Get(ctx, client.ObjectKey{Namespace: group.Namespace, Name: ch.Config.Name}, &cm); err != nil {
			return err
		}
		if cm.Data == nil {
			return nil
		}
		if v := cm.Data[key]; v != "" {
			group.Status.MatrixUserID = v
		}
		return nil
	}
	return nil
}

// CollaborationReconciler reconciles Collaboration resources.
type CollaborationReconciler struct {
	client.Client
}

func (r *CollaborationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var collab agentorgsv1alpha1.Collaboration
	if err := r.Get(ctx, req.NamespacedName, &collab); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	collab.Status.AppliedConfigVersion = collab.Generation
	collab.Status.Phase = agentorgsv1alpha1.CollaborationPhaseWaiting

	var memberList agentorgsv1alpha1.MemberList
	if err := r.List(ctx, &memberList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	var groupList agentorgsv1alpha1.GroupList
	if err := r.List(ctx, &groupList, client.InNamespace(req.Namespace)); err != nil {
		return ctrl.Result{}, err
	}
	resolver := organization.NewResolver(memberList.Items, groupList.Items)

	count := 0
	for _, p := range collab.Spec.Participants {
		switch p.Who.Kind {
		case agentorgsv1alpha1.MemberKind:
			count++
		case agentorgsv1alpha1.GroupKind:
			expanded, err := resolver.ExpandGroup(p.Who.Name)
			if err != nil {
				collab.Status.Phase = agentorgsv1alpha1.CollaborationPhaseInvalid
				collab.Status.StatusDetails = setCondition(collab.Status.StatusDetails, "Ready", "InvalidParticipants", err.Error(), metav1.ConditionFalse)
				return ctrl.Result{}, r.Status().Update(ctx, &collab)
			}
			count += len(expanded)
		}
	}
	if count < 2 {
		collab.Status.Phase = agentorgsv1alpha1.CollaborationPhaseInvalid
		collab.Status.StatusDetails = setCondition(collab.Status.StatusDetails, "Ready", "TooFewParticipants", "collaboration requires at least two members", metav1.ConditionFalse)
		return ctrl.Result{}, r.Status().Update(ctx, &collab)
	}

	collab.Status.Phase = agentorgsv1alpha1.CollaborationPhaseReady
	collab.Status.MemberCount = count
	collab.Status.StatusDetails = setCondition(collab.Status.StatusDetails, "Ready", "Reconciled", "Collaboration is ready", metav1.ConditionTrue)
	return ctrl.Result{}, r.Status().Update(ctx, &collab)
}

// PolicyReconciler reconciles Policy resources.
type PolicyReconciler struct {
	client.Client
}

func (r *PolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var pol agentorgsv1alpha1.Policy
	if err := r.Get(ctx, req.NamespacedName, &pol); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	pol.Status.AppliedConfigVersion = pol.Generation
	if pol.Spec.Effect == "" {
		pol.Status.Phase = agentorgsv1alpha1.PolicyPhaseInvalid
		pol.Status.StatusDetails = setCondition(pol.Status.StatusDetails, "Ready", "MissingEffect", "policy effect is required", metav1.ConditionFalse)
		return ctrl.Result{}, r.Status().Update(ctx, &pol)
	}

	pol.Status.Phase = agentorgsv1alpha1.PolicyPhaseReady
	pol.Status.StatusDetails = setCondition(pol.Status.StatusDetails, "Ready", "Reconciled", "Policy is ready", metav1.ConditionTrue)
	return ctrl.Result{}, r.Status().Update(ctx, &pol)
}

func SetupMemberReconciler(mgr ctrl.Manager, registry *provider.Registry) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentorgsv1alpha1.Member{}).
		Complete(&MemberReconciler{Client: mgr.GetClient(), Registry: registry})
}

func SetupGroupReconciler(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentorgsv1alpha1.Group{}).
		Complete(&GroupReconciler{Client: mgr.GetClient()})
}

func SetupCollaborationReconciler(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentorgsv1alpha1.Collaboration{}).
		Complete(&CollaborationReconciler{Client: mgr.GetClient()})
}

func SetupPolicyReconciler(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agentorgsv1alpha1.Policy{}).
		Complete(&PolicyReconciler{Client: mgr.GetClient()})
}
