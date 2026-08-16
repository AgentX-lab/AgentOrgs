package hermes

import (
	"context"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	openclawadapter "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/runtime/openclaw"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const providerName = "hermes"

// Adapter configures Hermes Members. Workspace config reuses openclaw.json;
// the Hermes image bridges that file into HERMES_HOME at container start.
type Adapter struct {
	Config  config.Config
	Storage provider.StorageProvider
	K8s     client.Client
}

func NewAdapter(cfg config.Config, storage provider.StorageProvider, k8s client.Client) *Adapter {
	return &Adapter{Config: cfg, Storage: storage, K8s: k8s}
}

func (a *Adapter) Name() string { return providerName }

func (a *Adapter) Apply(ctx context.Context, member provider.MemberContext) error {
	return openclawadapter.ApplyMemberWorkspace(ctx, a.Config, a.Storage, a.K8s, member)
}

func (a *Adapter) Delete(_ context.Context, _ provider.MemberContext) error {
	return nil
}
