package provider

import (
	"context"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
)

// MemberContext carries namespace-scoped member identity for providers.
type MemberContext struct {
	Namespace string
	Name      string
	Spec      agentorgsv1alpha1.MemberSpec
}

// ExecutionInstanceRef identifies a running member instance.
type ExecutionInstanceRef struct {
	Ref string
}

// EventHandler receives inbound collaboration events from a provider.
type EventHandler func(ctx context.Context, event protocol.CollaborationEvent) error

// RuntimeAdapter configures an agent runtime and delivers collaboration events.
type RuntimeAdapter interface {
	Name() string
	Apply(ctx context.Context, member MemberContext) error
	Delete(ctx context.Context, member MemberContext) error
	SendRequest(ctx context.Context, member MemberContext, event protocol.CollaborationEvent) error
}

// ExecutionBackend creates and deletes member runtime environments.
type ExecutionBackend interface {
	Name() string
	Apply(ctx context.Context, member MemberContext) (ExecutionInstanceRef, error)
	Delete(ctx context.Context, member MemberContext) error
}

// CollaborationProvider transports collaboration events and messages.
type CollaborationProvider interface {
	Name() string
	Deliver(ctx context.Context, event protocol.CollaborationEvent) error
	Subscribe(ctx context.Context, handler EventHandler) error
}

// StorageProvider persists runs, events, artifacts, and Member workspaces.
type StorageProvider interface {
	Name() string
	WriteRun(ctx context.Context, run protocol.CollaborationRun) error
	ReadRun(ctx context.Context, namespace, runID string) (protocol.CollaborationRun, error)
	WriteEvent(ctx context.Context, event protocol.CollaborationEvent) error
	ListEvents(ctx context.Context, namespace, runID string) ([]protocol.CollaborationEvent, error)
	// EnsureMemberWorkspace creates the default persona/skills/config objects when missing.
	EnsureMemberWorkspace(ctx context.Context, namespace, memberName, displayName string) error
	// GetWorkspaceFile reads one file from a Member workspace (relative path, e.g. "openclaw.json").
	GetWorkspaceFile(ctx context.Context, namespace, memberName, relativePath string) ([]byte, error)
	// PutWorkspaceFile writes one file into a Member workspace.
	PutWorkspaceFile(ctx context.Context, namespace, memberName, relativePath string, data []byte) error
}
