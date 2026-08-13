package matrix

import (
	"context"
	"fmt"
	"sync"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const providerName = "matrix"

// Provider transports collaboration events through Matrix rooms.
//
// Inbound path: AppServiceHandler → emit → EventHandler.
// Outbound path: Engine → Deliver → Matrix room message.
type Provider struct {
	Config config.Config
	K8s    client.Client
	API    *Client

	mu      sync.RWMutex
	handler provider.EventHandler
}

func NewProvider(cfg config.Config, k8s client.Client) *Provider {
	return &Provider{
		Config: cfg,
		K8s:    k8s,
		API:    NewClient(cfg.MatrixHomeserver, cfg.MatrixDomain, cfg.MatrixAppServiceASToken),
	}
}

func (p *Provider) Name() string { return providerName }

// Deliver posts a human-readable progress line into the Collaboration room.
func (p *Provider) Deliver(ctx context.Context, event protocol.CollaborationEvent) error {
	roomID, err := p.lookupRoomID(ctx, event.Namespace, event.Collaboration)
	if err != nil {
		return err
	}
	token, err := p.getRoomOperatorToken(ctx)
	if err != nil {
		return err
	}
	return p.API.SendMessage(ctx, token, roomID, formatEventText(event))
}

func (p *Provider) Subscribe(ctx context.Context, handler provider.EventHandler) error {
	p.mu.Lock()
	p.handler = handler
	p.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

func (p *Provider) emit(ctx context.Context, event protocol.CollaborationEvent) error {
	p.mu.RLock()
	handler := p.handler
	p.mu.RUnlock()
	if handler == nil {
		return fmt.Errorf("matrix provider has no inbound handler")
	}
	return handler(ctx, event)
}

func (p *Provider) lookupRoomID(ctx context.Context, namespace, collaborationName string) (string, error) {
	var collab agentorgsv1alpha1.Collaboration
	if err := p.K8s.Get(ctx, client.ObjectKey{Namespace: namespace, Name: collaborationName}, &collab); err != nil {
		return "", fmt.Errorf("load collaboration %s/%s: %w", namespace, collaborationName, err)
	}
	if collab.Spec.Channel.Provider != "" && collab.Spec.Channel.Provider != providerName {
		return "", fmt.Errorf("collaboration %q channel is %q, not matrix", collaborationName, collab.Spec.Channel.Provider)
	}
	cmName := collab.Spec.Channel.Config.Name
	cmKey := collab.Spec.Channel.Config.Key
	if cmKey == "" {
		cmKey = "roomId"
	}
	roomID, err := ReadConfigMapKey(ctx, p.K8s, namespace, cmName, cmKey)
	if err != nil {
		return "", err
	}
	if roomID == "" {
		return "", fmt.Errorf("collaboration %q has empty roomId in ConfigMap %q", collaborationName, cmName)
	}
	return roomID, nil
}

// getRoomOperatorToken returns a token used to post messages into collaboration rooms.
func (p *Provider) getRoomOperatorToken(ctx context.Context) (string, error) {
	if p.Config.MatrixAccessToken != "" {
		return p.Config.MatrixAccessToken, nil
	}
	if p.Config.MatrixAppServiceASToken == "" {
		return "", fmt.Errorf("no matrix access token: set AGENTORGS_MATRIX_ACCESS_TOKEN or AppService AS token")
	}
	return p.API.LoginAppServiceUser(ctx, p.Config.MatrixAppServiceSenderLocalpart)
}

func formatEventText(event protocol.CollaborationEvent) string {
	text, _ := event.Payload["text"].(string)
	if text == "" {
		text = fmt.Sprintf("[%s] collaboration event", event.Type)
	}
	names := make([]string, 0, len(event.Targets))
	for _, t := range event.Targets {
		names = append(names, t.Kind+"/"+t.Name)
	}
	return fmt.Sprintf("%s -> %v: %s", event.Source.Member, names, text)
}
