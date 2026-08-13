package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PolicyKind = "Policy"
)

// PolicyEffect says whether this rule allows or blocks an action.
type PolicyEffect string

const (
	PolicyEffectAllow PolicyEffect = "Allow"
	PolicyEffectDeny  PolicyEffect = "Deny"
)

// PolicyAction is a collaboration action controlled by Policy.
type PolicyAction string

const (
	PolicyActionStart    PolicyAction = "Start"    // start a new run
	PolicyActionContinue PolicyAction = "Continue" // continue an existing run
	PolicyActionCancel   PolicyAction = "Cancel"   // cancel a run
)

// PolicyPhase is the current state of a Policy.
type PolicyPhase string

const (
	PolicyPhaseWaiting PolicyPhase = "Waiting" // still checking config
	PolicyPhaseReady   PolicyPhase = "Ready"   // policy can be used
	PolicyPhaseInvalid PolicyPhase = "Invalid" // config has errors
)

// PolicySpec is what the user configures for a Policy.
// +kubebuilder:object:generate=true
type PolicySpec struct {
	// Priority decides which policy wins when rules conflict.
	// Larger number means higher priority.
	Priority int            `json:"priority,omitempty"`
	Effect   PolicyEffect   `json:"effect"`             // Allow or Deny
	From     []ObjectRef    `json:"from,omitempty"`     // who may act
	To       []ObjectRef    `json:"to,omitempty"`       // who they may act on
	Actions  []PolicyAction `json:"actions,omitempty"`  // which actions are covered
}

// PolicyStatus is what the system reports back.
// +kubebuilder:object:generate=true
type PolicyStatus struct {
	AppliedConfigVersion int64       `json:"appliedConfigVersion,omitempty"` // which config version is applied
	Phase                PolicyPhase `json:"phase,omitempty"`                // Waiting / Ready / Invalid
	StatusDetails        []Condition `json:"statusDetails,omitempty"`        // detailed success or error notes
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=pol
// +kubebuilder:printcolumn:name="Effect",type=string,JSONPath=`.spec.effect`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

type Policy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PolicySpec   `json:"spec,omitempty"`
	Status PolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type PolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Policy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Policy{}, &PolicyList{})
}
