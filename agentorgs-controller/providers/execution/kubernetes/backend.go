package kubernetes

import (
	"context"
	"fmt"
	"strconv"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const providerName = "kubernetes"

// Backend creates member agent pods in Kubernetes.
type Backend struct {
	Client client.Client
	Config config.Config
}

func NewBackend(c client.Client, cfg config.Config) *Backend {
	return &Backend{Client: c, Config: cfg}
}

func (b *Backend) Name() string { return providerName }

func (b *Backend) Apply(ctx context.Context, member provider.MemberContext) (provider.ExecutionInstanceRef, error) {
	podName := podNameFor(member.Name)
	var existing corev1.Pod
	err := b.Client.Get(ctx, client.ObjectKey{Namespace: member.Namespace, Name: podName}, &existing)
	if err == nil {
		return provider.ExecutionInstanceRef{Ref: podName}, nil
	}
	if !apierrors.IsNotFound(err) {
		return provider.ExecutionInstanceRef{}, err
	}

	image, err := b.agentImage(member)
	if err != nil {
		return provider.ExecutionInstanceRef{}, err
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: member.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":  "agentorgs-member",
				"agentorgs.io/member":     member.Name,
				"agentorgs.io/managed-by": "agentorgs-controller",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyAlways,
			Containers: []corev1.Container{{
				Name:            "agent",
				Image:           image,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Env:             b.agentEnv(member),
			}},
		},
	}
	if err := b.Client.Create(ctx, pod); err != nil {
		return provider.ExecutionInstanceRef{}, err
	}
	return provider.ExecutionInstanceRef{Ref: podName}, nil
}

func (b *Backend) Delete(ctx context.Context, member provider.MemberContext) error {
	podName := podNameFor(member.Name)
	var pod corev1.Pod
	if err := b.Client.Get(ctx, client.ObjectKey{Namespace: member.Namespace, Name: podName}, &pod); err != nil {
		return client.IgnoreNotFound(err)
	}
	return b.Client.Delete(ctx, &pod)
}

func (b *Backend) agentImage(member provider.MemberContext) (string, error) {
	if member.Spec.Image != "" {
		return member.Spec.Image, nil
	}
	runtimeName := ""
	if member.Spec.Runtime != nil {
		runtimeName = member.Spec.Runtime.Provider
	}
	switch runtimeName {
	case "openclaw", "":
		if b.Config.OpenClawAgentImage == "" {
			return "", fmt.Errorf("AGENTORGS_OPENCLAW_AGENT_IMAGE is not set")
		}
		return b.Config.OpenClawAgentImage, nil
	case "hermes":
		if b.Config.HermesAgentImage == "" {
			return "", fmt.Errorf("AGENTORGS_HERMES_AGENT_IMAGE is not set")
		}
		return b.Config.HermesAgentImage, nil
	default:
		return "", fmt.Errorf("no default agent image for runtime %q; set Member.spec.image", runtimeName)
	}
}

func (b *Backend) agentEnv(member provider.MemberContext) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "AGENTORGS_MEMBER_NAME", Value: member.Name},
		{Name: "AGENTORGS_NAMESPACE", Value: member.Namespace},
		{Name: "AGENTORGS_WORKSPACE_DIR", Value: "/workspace"},
		{Name: "AGENTORGS_SYNC_INTERVAL_SECONDS", Value: strconv.Itoa(b.Config.WorkspaceSyncSeconds)},
		{Name: "AGENTORGS_MINIO_ENDPOINT", Value: b.Config.MinIOEndpoint},
		{Name: "AGENTORGS_MINIO_ACCESS_KEY", Value: b.Config.MinIOAccessKey},
		{Name: "AGENTORGS_MINIO_SECRET_KEY", Value: b.Config.MinIOSecretKey},
		{Name: "AGENTORGS_MINIO_BUCKET", Value: b.Config.MinIOBucket},
		{Name: "AGENTORGS_MINIO_USE_SSL", Value: strconv.FormatBool(b.Config.MinIOUseSSL)},
		{Name: "AGENTORGS_CONTROLLER_URL", Value: b.Config.ControllerURL},
		// LLM credentials are injected by the controller; runtimes read OpenAI-compatible env vars.
		{Name: "OPENAI_API_KEY", Value: b.Config.LLMAPIKey},
		{Name: "OPENAI_BASE_URL", Value: b.Config.LLMBaseURL},
	}
}

func podNameFor(member string) string {
	return fmt.Sprintf("member-%s", member)
}
