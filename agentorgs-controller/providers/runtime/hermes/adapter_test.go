package hermes_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	hermesadapter "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/runtime/hermes"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type workspaceStorage struct {
	files map[string][]byte
}

func (s *workspaceStorage) Name() string { return "test" }
func (s *workspaceStorage) WriteRun(context.Context, protocol.CollaborationRun) error {
	return nil
}
func (s *workspaceStorage) ReadRun(context.Context, string, string) (protocol.CollaborationRun, error) {
	return protocol.CollaborationRun{}, fmt.Errorf("unused")
}
func (s *workspaceStorage) WriteEvent(context.Context, protocol.CollaborationEvent) error {
	return nil
}
func (s *workspaceStorage) ListEvents(context.Context, string, string) ([]protocol.CollaborationEvent, error) {
	return nil, nil
}
func (s *workspaceStorage) EnsureMemberWorkspace(context.Context, string, string, string, string, []string) error {
	return nil
}
func (s *workspaceStorage) GetWorkspaceFile(_ context.Context, namespace, memberName, relativePath string) ([]byte, error) {
	key := namespace + "/" + memberName + "/" + relativePath
	data, ok := s.files[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	return data, nil
}
func (s *workspaceStorage) PutWorkspaceFile(_ context.Context, namespace, memberName, relativePath string, data []byte) error {
	key := namespace + "/" + memberName + "/" + relativePath
	s.files[key] = data
	return nil
}

func TestHermesApplyWritesOpenClawJSONWithRequireMention(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)

	const roomID = "!hermes-room:matrix-local.agentorgs.io"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "matrix-hermes-1", Namespace: "agentorgs"},
		Data: map[string][]byte{
			"userId":      []byte("@hermes-1:matrix-local.agentorgs.io"),
			"accessToken": []byte("tok-hermes"),
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "hermes-channel", Namespace: "agentorgs"},
		Data:       map[string]string{"roomId": roomID},
	}
	member := &v1alpha1.Member{
		ObjectMeta: metav1.ObjectMeta{Name: "hermes-1", Namespace: "agentorgs"},
	}
	collab := &v1alpha1.Collaboration{
		ObjectMeta: metav1.ObjectMeta{Name: "hermes-work", Namespace: "agentorgs"},
		Spec: v1alpha1.CollaborationSpec{
			Participants: []v1alpha1.CollaborationParticipant{
				{Who: v1alpha1.ObjectRef{Kind: v1alpha1.MemberKind, Name: "hermes-1"}},
			},
			Channel: v1alpha1.ProviderBinding{
				Provider: "matrix",
				Config:   v1alpha1.ConfigRef{Name: "hermes-channel", Key: "roomId"},
			},
		},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, cm, member, collab).Build()
	storage := &workspaceStorage{files: map[string][]byte{
		"agentorgs/hermes-1/openclaw.json": []byte(`{"channels":{"matrix":{"enabled":false}}}`),
	}}
	adapter := hermesadapter.NewAdapter(config.Config{
		MatrixHomeserver: "http://agentorgs-tuwunel:6167",
		LLMBaseURL:       "http://mock-llm:6556/v1",
		LLMAPIKey:        "sk-test",
		DefaultModel:     "gpt-4o-mini",
	}, storage, k8s)

	if got := adapter.Name(); got != "hermes" {
		t.Fatalf("Name=%q", got)
	}
	if err := adapter.Apply(context.Background(), provider.MemberContext{
		Namespace: "agentorgs",
		Name:      "hermes-1",
		Spec: v1alpha1.MemberSpec{
			Channels: []v1alpha1.ProviderBinding{{Provider: "matrix"}},
		},
	}); err != nil {
		t.Fatal(err)
	}

	raw := storage.files["agentorgs/hermes-1/openclaw.json"]
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	matrix := cfg["channels"].(map[string]interface{})["matrix"].(map[string]interface{})
	if matrix["requireMention"] != true {
		t.Fatalf("requireMention=%v", matrix["requireMention"])
	}
	groups := matrix["groups"].(map[string]interface{})
	if groups[roomID].(map[string]interface{})["requireMention"] != true {
		t.Fatalf("groups=%v", groups)
	}
}
