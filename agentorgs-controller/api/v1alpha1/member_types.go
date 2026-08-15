package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	MemberKind = "Member"
	// MemberRuntimeReadyAnnotation is set by the member process after it can accept work.
	// Only the Member reconciler may derive status.phase from this fact.
	MemberRuntimeReadyAnnotation = "agentorgs.io/runtime-ready"
)

// MemberType says what kind of participant this is.
type MemberType string

const (
	MemberTypeAgent MemberType = "Agent" // an AI agent
	MemberTypeHuman MemberType = "Human" // a human user
)

// MemberPhase is the current state of a Member.
type MemberPhase string

const (
	MemberPhaseWaiting  MemberPhase = "Waiting"  // still starting
	MemberPhaseReady    MemberPhase = "Ready"    // ready to collaborate
	MemberPhaseNotReady MemberPhase = "NotReady" // failed or incomplete setup
)

// MemberSpec is what the user configures for a Member.
// +kubebuilder:object:generate=true
type MemberSpec struct {
	DisplayName string            `json:"displayName,omitempty"` // name shown in the UI
	Type        MemberType        `json:"type"`                  // Agent / Human
	Image       string            `json:"image,omitempty"`       // container image, only for Agent
	Runtime     *ProviderBinding  `json:"runtime,omitempty"`     // agent runtime, only for Agent
	Execution   *ProviderBinding  `json:"execution,omitempty"`   // where the agent runs
	Channels    []ProviderBinding `json:"channels,omitempty"`    // communication channels
	// SkillPack is a builtin craft preset (e.g. coordination, backend). Expands to multiple skills.
	// Independent of Group.members.role (collaboration role).
	SkillPack string `json:"skillPack,omitempty"`
	// Skills lists extra builtin skill names beyond the pack (incremental, not a full inventory).
	Skills []string `json:"skills,omitempty"`
}

// MemberConnections records the live provider instances for this Member.
type MemberConnections struct {
	Runtime   string `json:"runtime,omitempty"`   // runtime instance id
	Execution string `json:"execution,omitempty"` // execution instance id
}

// MemberStatus is what the system reports back.
// +kubebuilder:object:generate=true
type MemberStatus struct {
	AppliedConfigVersion int64             `json:"appliedConfigVersion,omitempty"` // which config version is applied
	Phase                MemberPhase       `json:"phase,omitempty"`                // Waiting / Ready / NotReady
	Connections          MemberConnections `json:"connections,omitempty"`          // live provider links
	MatrixUserID         string            `json:"matrixUserId,omitempty"`         // resolved Matrix MXID for @mentions
	StatusDetails        []Condition       `json:"statusDetails,omitempty"`        // detailed success or error notes
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=mem
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`

type Member struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MemberSpec   `json:"spec,omitempty"`
	Status MemberStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

type MemberList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ListMeta   `json:"metadata,omitempty"`
	Items           []Member `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Member{}, &MemberList{})
}
