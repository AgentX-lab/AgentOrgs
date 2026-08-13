package protocol

import "time"

const (
	EventTypeMemberRequest = "MemberRequest"
	EventTypeMessage       = "Message"
	EventTypeResult        = "Result"
	EventTypeCancel        = "Cancel"
	EventTypeError         = "Error"
)

const (
	RunStatusRunning   = "Running"
	RunStatusCompleted = "Completed"
	RunStatusFailed    = "Failed"
	RunStatusCancelled = "Cancelled"
	RunStatusTimedOut  = "TimedOut"
)

const (
	PolicyActionStart    = "Start"
	PolicyActionContinue = "Continue"
	PolicyActionCancel   = "Cancel"
)

// ObjectTarget identifies a Member or Group target.
type ObjectTarget struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// EventSource identifies who sent an event.
type EventSource struct {
	Member        string `json:"member"`
	ActingAsGroup string `json:"actingAsGroup,omitempty"`
}

// ArtifactRef points to stored content without exposing provider paths.
type ArtifactRef struct {
	Name    string `json:"name"`
	Ref     string `json:"ref"`
	SHA256  string `json:"sha256,omitempty"`
}

// CollaborationEvent is the provider-neutral collaboration message envelope.
type CollaborationEvent struct {
	EventID      string                 `json:"eventId"`
	RunID        string                 `json:"runId,omitempty"`
	Namespace    string                 `json:"namespace"`
	Collaboration string                `json:"collaborationName"`
	Type         string                 `json:"type"`
	Source       EventSource            `json:"source"`
	Targets      []ObjectTarget         `json:"targets"`
	Payload      map[string]interface{} `json:"payload,omitempty"`
	ArtifactRefs []ArtifactRef          `json:"artifactRefs,omitempty"`
	CreatedAt    time.Time              `json:"createdAt"`
}

// CollaborationRun tracks one collaboration execution.
type CollaborationRun struct {
	RunID             string                 `json:"runId"`
	Namespace         string                 `json:"namespace"`
	CollaborationName string                 `json:"collaborationName"`
	StartedBy         string                 `json:"startedBy"`
	ActingAsGroup     string                 `json:"actingAsGroup,omitempty"`
	ResolvedTargets   []string               `json:"resolvedTargets"`
	Status            string                 `json:"status"`
	Round             int                    `json:"round"`
	MaxReplyRounds    int                    `json:"maxReplyRounds"`
	StartedAt         time.Time              `json:"startedAt"`
	Deadline          time.Time              `json:"deadline,omitempty"`
	Result            map[string]interface{} `json:"result,omitempty"`
}
