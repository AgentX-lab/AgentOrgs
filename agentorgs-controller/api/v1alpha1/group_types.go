package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	GroupKind = "Group"
)

// GroupPhase is the current state of a Group.
type GroupPhase string

const (
	GroupPhaseWaiting GroupPhase = "Waiting" // still checking members
	GroupPhaseReady   GroupPhase = "Ready"   // all members are valid
	GroupPhaseInvalid GroupPhase = "Invalid" // member list has errors
)

// GroupMember is one person or subgroup inside this Group.
type GroupMember struct {
	Who  ObjectRef `json:"who"`            // which Member or Group
	Role string    `json:"role,omitempty"` // role inside this Group
}

// GroupSpec is what the user configures for a Group.
// +kubebuilder:object:generate=true
type GroupSpec struct {
	DisplayName string            `json:"displayName,omitempty"` // name shown in the UI
	Members     []GroupMember     `json:"members,omitempty"`     // who belongs to this Group
	Channels    []ProviderBinding `json:"channels,omitempty"`    // e.g. Matrix identity so the Group can be @mentioned
}

// GroupStatus is what the system reports back.
// +kubebuilder:object:generate=true
type GroupStatus struct {
	AppliedConfigVersion int64       `json:"appliedConfigVersion,omitempty"` // which config version is applied
	Phase                GroupPhase  `json:"phase,omitempty"`                // Waiting / Ready / Invalid
	MemberCount          int         `json:"memberCount,omitempty"`          // how many members after expansion
	MatrixUserID         string      `json:"matrixUserId,omitempty"`         // resolved Matrix MXID for @mentions
	StatusDetails        []Condition `json:"statusDetails,omitempty"`        // detailed success or error notes
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=grp
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Members",type=integer,JSONPath=`.status.memberCount`

type Group struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GroupSpec   `json:"spec,omitempty"`
	Status GroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type GroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Group `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Group{}, &GroupList{})
}
