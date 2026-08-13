package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	CollaborationKind = "Collaboration"
)

// CollaborationPhase is the current state of a Collaboration.
type CollaborationPhase string

const (
	CollaborationPhaseWaiting CollaborationPhase = "Waiting" // still checking participants
	CollaborationPhaseReady   CollaborationPhase = "Ready"   // ready to start runs
	CollaborationPhaseInvalid CollaborationPhase = "Invalid" // config has errors
)

// StartFrom says who is allowed to start a collaboration run.
type StartFrom string

const (
	StartFromMember   StartFrom = "Member"   // a human or agent starts it
	StartFromSchedule StartFrom = "Schedule" // a timer starts it
)

// GroupTargetStrategy says how to pick members when the target is a Group.
type GroupTargetStrategy string

const (
	GroupTargetAll    GroupTargetStrategy = "All"    // send to every member
	GroupTargetAny    GroupTargetStrategy = "Any"    // send to one available member
	GroupTargetRole   GroupTargetStrategy = "Role"   // send to members with a role
	GroupTargetLeader GroupTargetStrategy = "Leader" // send to the leader
)

// CollaborationParticipant is one Member or Group in this Collaboration.
type CollaborationParticipant struct {
	Who  ObjectRef `json:"who"`            // which Member or Group
	Role string    `json:"role,omitempty"` // role inside this Collaboration
}

// WhenTargetIsGroup controls member selection for Group targets.
type WhenTargetIsGroup struct {
	Strategy GroupTargetStrategy `json:"strategy"`
	Role     string              `json:"role,omitempty"`
}

// ExpectedResult lists fields that a collaboration result should include.
type ExpectedResult struct {
	MustHave []string `json:"mustHave,omitempty"`
	MayHave  []string `json:"mayHave,omitempty"`
}

type CollaborationLimits struct {
	MaxReplyRounds int `json:"maxReplyRounds,omitempty"`
	TimeoutMinutes int `json:"timeoutMinutes,omitempty"`
}

// CollaborationSpec is what the user configures for a Collaboration.
// +kubebuilder:object:generate=true
type CollaborationSpec struct {
	Participants      []CollaborationParticipant `json:"participants,omitempty"`      // who can take part
	Channel           ProviderBinding            `json:"channel,omitempty"`           // communication channel
	AllowStartFrom    []StartFrom                `json:"allowStartFrom,omitempty"`    // who may start a run
	WhenTargetIsGroup WhenTargetIsGroup          `json:"whenTargetIsGroup,omitempty"` // how to pick Group targets
	ExpectedResult    ExpectedResult             `json:"expectedResult,omitempty"`    // required result fields
	Limits            CollaborationLimits        `json:"limits,omitempty"`            // round and time limits
}

// CollaborationStatus is what the system reports back.
// +kubebuilder:object:generate=true
type CollaborationStatus struct {
	AppliedConfigVersion int64              `json:"appliedConfigVersion,omitempty"` // which config version is applied
	Phase                CollaborationPhase `json:"phase,omitempty"`                // Waiting / Ready / Invalid
	MemberCount          int                `json:"memberCount,omitempty"`          // how many members after expansion
	StatusDetails        []Condition        `json:"statusDetails,omitempty"`        // detailed success or error notes
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=col
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Members",type=integer,JSONPath=`.status.memberCount`

type Collaboration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CollaborationSpec   `json:"spec,omitempty"`
	Status CollaborationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type CollaborationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Collaboration `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Collaboration{}, &CollaborationList{})
}
