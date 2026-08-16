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
	Member string `json:"member"`
}

// DispatchIntent is who this one task should (or should not) wake.
// Empty fields mean "no extra filter" — Collaboration + Policy still apply.
type DispatchIntent struct {
	// OnlyTheseMembers: if set, keep only these names (must already be in the expanded set).
	OnlyTheseMembers []string `json:"include,omitempty"`
	// SkipTheseMembers: drop these names after expand.
	SkipTheseMembers []string `json:"exclude,omitempty"`
	// OnlyThisRole: for this request, treat Group expand as Role strategy with this role.
	OnlyThisRole string `json:"role,omitempty"`
}

// Empty reports whether no dispatch filters were set.
func (d DispatchIntent) Empty() bool {
	return len(d.OnlyTheseMembers) == 0 && len(d.SkipTheseMembers) == 0 && d.OnlyThisRole == ""
}

// CollaborationEvent is the provider-neutral collaboration message envelope.
type CollaborationEvent struct {
	EventID       string                 `json:"eventId"`
	RunID         string                 `json:"runId,omitempty"`
	Namespace     string                 `json:"namespace"`
	Collaboration string                 `json:"collaborationName"`
	Type          string                 `json:"type"`
	Source        EventSource            `json:"source"`
	Targets       []ObjectTarget         `json:"targets"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
	// DispatchIntent is optional per-request who/who-not (Matrix -@ or HTTP include/exclude).
	DispatchIntent DispatchIntent `json:"dispatchIntent,omitempty"`
	// MentionUserIDs are Matrix MXIDs to visibly @ when delivering (wake agents).
	MentionUserIDs []string  `json:"mentionUserIds,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// CollaborationRun tracks one collaboration execution.
type CollaborationRun struct {
	RunID             string                 `json:"runId"`
	Namespace         string                 `json:"namespace"`
	CollaborationName string                 `json:"collaborationName"`
	StartedBy         string                 `json:"startedBy"`
	ResolvedTargets   []string               `json:"resolvedTargets"`
	// DispatchIntent records the filters used when this run was started (audit).
	DispatchIntent    DispatchIntent         `json:"dispatchIntent,omitempty"`
	Status            string                 `json:"status"`
	Round             int                    `json:"round"`
	MaxReplyRounds    int                    `json:"maxReplyRounds"`
	StartedAt         time.Time              `json:"startedAt"`
	Deadline          time.Time              `json:"deadline,omitempty"`
	Result            map[string]interface{} `json:"result,omitempty"`
}
