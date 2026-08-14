package openclaw

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	providerName       = "openclaw"
	openclawConfigFile = "openclaw.json"
	matrixSecretPrefix = "matrix-"
)

// Adapter connects OpenClaw runtimes to AgentOrgs collaboration events.
// It owns openclaw.json shape; Matrix Setup only supplies credentials via Secret.
type Adapter struct {
	Config  config.Config
	Storage provider.StorageProvider
	K8s     client.Client
	Client  *http.Client
}

func NewAdapter(cfg config.Config, storage provider.StorageProvider, k8s client.Client) *Adapter {
	return &Adapter{
		Config:  cfg,
		Storage: storage,
		K8s:     k8s,
		Client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (a *Adapter) Name() string { return providerName }

// Apply writes runtime config into the Member workspace (including Matrix channel when credentials exist).
func (a *Adapter) Apply(ctx context.Context, member provider.MemberContext) error {
	if a.Storage == nil {
		return fmt.Errorf("openclaw adapter: storage is required")
	}
	if !hasMatrixChannel(member) {
		return nil
	}
	userID, token, err := a.readMatrixCredentials(ctx, member.Namespace, member.Name)
	if err != nil {
		return err
	}
	if userID == "" || token == "" {
		// Matrix Setup has not created credentials yet; reconcile again later.
		return fmt.Errorf("matrix credentials for member %q are not ready", member.Name)
	}
	return a.writeMatrixChannel(ctx, member.Namespace, member.Name, userID, token)
}

func (a *Adapter) Delete(_ context.Context, _ provider.MemberContext) error {
	return nil
}

func (a *Adapter) SendRequest(ctx context.Context, member provider.MemberContext, event protocol.CollaborationEvent) error {
	body, err := json.Marshal(map[string]interface{}{
		"member":  member.Name,
		"eventId": event.EventID,
		"runId":   event.RunID,
		"type":    event.Type,
		"payload": event.Payload,
	})
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://member-%s.%s.svc.cluster.local:8080/agentorgs/events", member.Name, member.Namespace)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.Client.Do(req)
	if err != nil {
		// MVP fallback: runtime hook may not exist yet; collaboration still proceeds via Matrix/API.
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("openclaw runtime returned %s", resp.Status)
	}
	return nil
}

func (a *Adapter) readMatrixCredentials(ctx context.Context, namespace, memberName string) (userID, token string, err error) {
	if a.K8s == nil {
		return "", "", fmt.Errorf("openclaw adapter: kubernetes client is required")
	}
	var secret corev1.Secret
	err = a.K8s.Get(ctx, client.ObjectKey{Namespace: namespace, Name: matrixSecretPrefix + memberName}, &secret)
	if apierrors.IsNotFound(err) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return string(secret.Data["userId"]), string(secret.Data["accessToken"]), nil
}

// writeMatrixChannel merges Matrix settings into workspace openclaw.json.
// Field names match OpenClaw / AgentTeams (homeserver, userId, accessToken).
func (a *Adapter) writeMatrixChannel(ctx context.Context, namespace, memberName, userID, token string) error {
	raw, err := a.Storage.GetWorkspaceFile(ctx, namespace, memberName, openclawConfigFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", openclawConfigFile, err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", openclawConfigFile, err)
	}

	// OpenClaw refuses to start without gateway.mode (exit 78).
	// Containers bind 0.0.0.0 by default and also require an auth token.
	gateway, _ := cfg["gateway"].(map[string]interface{})
	if gateway == nil {
		gateway = map[string]interface{}{}
		cfg["gateway"] = gateway
	}
	gateway["mode"] = "local"
	auth, _ := gateway["auth"].(map[string]interface{})
	if auth == nil {
		auth = map[string]interface{}{}
		gateway["auth"] = auth
	}
	if _, ok := auth["token"].(string); !ok || auth["token"] == "" {
		auth["token"] = "agentorgs-local"
	}
	remote, _ := gateway["remote"].(map[string]interface{})
	if remote == nil {
		remote = map[string]interface{}{}
		gateway["remote"] = remote
	}
	if _, ok := remote["token"].(string); !ok || remote["token"] == "" {
		remote["token"] = auth["token"]
	}

	channels, _ := cfg["channels"].(map[string]interface{})
	if channels == nil {
		channels = map[string]interface{}{}
		cfg["channels"] = channels
	}
	matrix, _ := channels["matrix"].(map[string]interface{})
	if matrix == nil {
		matrix = map[string]interface{}{}
		channels["matrix"] = matrix
	}
	matrix["enabled"] = true
	matrix["homeserver"] = a.Config.MatrixHomeserver
	matrix["userId"] = userID
	matrix["accessToken"] = token
	// Collaboration rooms are group chats; allow invited rooms and require @mention.
	matrix["groupPolicy"] = "open"
	matrix["groups"] = map[string]interface{}{
		"*": map[string]interface{}{"requireMention": true},
	}
	matrix["autoJoin"] = "allowlist"
	matrix["autoJoinAllowlist"] = []string{"*"}
	matrix["dm"] = map[string]interface{}{"policy": "open"}
	// ClusterIP / private homeserver hosts fail OpenClaw SSRF checks otherwise.
	matrix["network"] = map[string]interface{}{
		"dangerouslyAllowPrivateNetwork": true,
	}

	// Point OpenClaw at the controller-configured OpenAI-compatible endpoint.
	// LLMBaseURL comes from AGENTORGS_LLM_BASE_URL (real provider in prod; mock only in e2e).
	baseURL := a.Config.LLMBaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	modelID := a.Config.DefaultModel
	if modelID == "" {
		modelID = "gpt-4o-mini"
	}
	models, _ := cfg["models"].(map[string]interface{})
	if models == nil {
		models = map[string]interface{}{}
		cfg["models"] = models
	}
	providers, _ := models["providers"].(map[string]interface{})
	if providers == nil {
		providers = map[string]interface{}{}
		models["providers"] = providers
	}
	providers["openai"] = map[string]interface{}{
		"baseUrl": baseURL,
		"apiKey":  a.Config.LLMAPIKey,
		"api":     "openai-completions",
		"models": []map[string]interface{}{
			{"id": modelID, "name": modelID},
		},
	}
	agents, _ := cfg["agents"].(map[string]interface{})
	if agents == nil {
		agents = map[string]interface{}{}
		cfg["agents"] = agents
	}
	defaults, _ := agents["defaults"].(map[string]interface{})
	if defaults == nil {
		defaults = map[string]interface{}{}
		agents["defaults"] = defaults
	}
	defaults["workspace"] = "/workspace"
	defaults["model"] = map[string]interface{}{
		"primary": "openai/" + modelID,
	}

	// External matrix plugin must be explicitly trusted or OpenClaw skips the channel.
	cfg["plugins"] = map[string]interface{}{
		"entries": map[string]interface{}{
			"matrix": map[string]interface{}{"enabled": true},
		},
	}

	out, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	out = append(out, '\n')
	return a.Storage.PutWorkspaceFile(ctx, namespace, memberName, openclawConfigFile, out)
}

func hasMatrixChannel(member provider.MemberContext) bool {
	for _, ch := range member.Spec.Channels {
		if ch.Provider == "matrix" {
			return true
		}
	}
	return false
}
