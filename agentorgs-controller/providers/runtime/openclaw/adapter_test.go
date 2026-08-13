package openclaw_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	openclawadapter "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/providers/runtime/openclaw"
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
func (s *workspaceStorage) EnsureMemberWorkspace(context.Context, string, string, string) error {
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

func TestApplyWritesMatrixIntoOpenClawJSON(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "matrix-be-1", Namespace: "agentorgs"},
		Data: map[string][]byte{
			"userId":      []byte("@be-1:matrix-local.agentorgs.io"),
			"accessToken": []byte("tok-be-1"),
		},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret).Build()

	storage := &workspaceStorage{files: map[string][]byte{
		"agentorgs/be-1/openclaw.json": []byte(`{"channels":{"matrix":{"enabled":false}}}`),
	}}
	adapter := openclawadapter.NewAdapter(config.Config{
		MatrixHomeserver: "http://tuwunel:6167",
		LLMBaseURL:       "http://mock-llm:6556/v1",
		LLMAPIKey:        "sk-test",
		DefaultModel:     "gpt-4o-mini",
	}, storage, k8s)

	err := adapter.Apply(context.Background(), provider.MemberContext{
		Namespace: "agentorgs",
		Name:      "be-1",
		Spec: v1alpha1.MemberSpec{
			Channels: []v1alpha1.ProviderBinding{{Provider: "matrix"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	raw := storage.files["agentorgs/be-1/openclaw.json"]
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	matrix := cfg["channels"].(map[string]interface{})["matrix"].(map[string]interface{})
	if matrix["enabled"] != true {
		t.Fatalf("enabled=%v", matrix["enabled"])
	}
	if matrix["homeserver"] != "http://tuwunel:6167" {
		t.Fatalf("homeserver=%v", matrix["homeserver"])
	}
	if matrix["userId"] != "@be-1:matrix-local.agentorgs.io" {
		t.Fatalf("userId=%v", matrix["userId"])
	}
	if matrix["accessToken"] != "tok-be-1" {
		t.Fatalf("accessToken=%v", matrix["accessToken"])
	}
	if matrix["groupPolicy"] != "open" {
		t.Fatalf("groupPolicy=%v", matrix["groupPolicy"])
	}
	providers := cfg["models"].(map[string]interface{})["providers"].(map[string]interface{})
	openai := providers["openai"].(map[string]interface{})
	if openai["baseUrl"] != "http://mock-llm:6556/v1" {
		t.Fatalf("baseUrl=%v", openai["baseUrl"])
	}
	primary := cfg["agents"].(map[string]interface{})["defaults"].(map[string]interface{})["model"].(map[string]interface{})["primary"]
	if primary != "openai/gpt-4o-mini" {
		t.Fatalf("primary=%v", primary)
	}
}

func TestApplySkipsWhenNoMatrixChannel(t *testing.T) {
	adapter := openclawadapter.NewAdapter(config.Config{}, &workspaceStorage{files: map[string][]byte{}}, nil)
	err := adapter.Apply(context.Background(), provider.MemberContext{
		Namespace: "agentorgs",
		Name:      "be-1",
		Spec:      v1alpha1.MemberSpec{},
	})
	if err != nil {
		t.Fatal(err)
	}
}
