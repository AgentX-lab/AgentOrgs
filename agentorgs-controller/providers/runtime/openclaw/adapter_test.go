package openclaw_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = v1alpha1.AddToScheme(scheme)
	return scheme
}

func TestApplyWritesMatrixIntoOpenClawJSON(t *testing.T) {
	scheme := testScheme(t)
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
		MatrixHomeserver: "http://agentorgs-tuwunel:6167",
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
	if matrix["homeserver"] != "http://agentorgs-tuwunel:6167" {
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
	groups := matrix["groups"].(map[string]interface{})
	star := groups["*"].(map[string]interface{})
	if star["requireMention"] != true {
		t.Fatalf("groups[*].requireMention=%v", star["requireMention"])
	}
	dm := matrix["dm"].(map[string]interface{})
	if dm["policy"] != "open" {
		t.Fatalf("dm.policy=%v", dm["policy"])
	}
	allowFrom, _ := dm["allowFrom"].([]interface{})
	if len(allowFrom) != 1 || allowFrom[0] != "*" {
		t.Fatalf("dm.allowFrom=%v", dm["allowFrom"])
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
	gateway := cfg["gateway"].(map[string]interface{})
	if gateway["mode"] != "local" {
		t.Fatalf("gateway.mode=%v", gateway["mode"])
	}
	if gateway["auth"].(map[string]interface{})["token"] != "agentorgs-local" {
		t.Fatalf("gateway.auth.token=%v", gateway["auth"])
	}
	network := matrix["network"].(map[string]interface{})
	if network["dangerouslyAllowPrivateNetwork"] != true {
		t.Fatalf("network=%v", network)
	}
	plugins := cfg["plugins"].(map[string]interface{})
	entries := plugins["entries"].(map[string]interface{})
	if entries["matrix"].(map[string]interface{})["enabled"] != true {
		t.Fatalf("plugins.entries.matrix=%v", entries["matrix"])
	}
}

func TestApplyWritesExactCollaborationRoomRequireMention(t *testing.T) {
	scheme := testScheme(t)
	const roomID = "!real-collab:matrix-local.agentorgs.io"
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "matrix-worker", Namespace: "agentorgs"},
		Data: map[string][]byte{
			"userId":      []byte("@worker:matrix-local.agentorgs.io"),
			"accessToken": []byte("tok-worker"),
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "work-channel", Namespace: "agentorgs"},
		Data:       map[string]string{"roomId": roomID},
	}
	member := &v1alpha1.Member{
		ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "agentorgs"},
		Spec: v1alpha1.MemberSpec{
			Channels: []v1alpha1.ProviderBinding{{Provider: "matrix"}},
		},
	}
	collab := &v1alpha1.Collaboration{
		ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: "agentorgs"},
		Spec: v1alpha1.CollaborationSpec{
			Participants: []v1alpha1.CollaborationParticipant{
				{Who: v1alpha1.ObjectRef{Kind: v1alpha1.MemberKind, Name: "worker"}},
			},
			Channel: v1alpha1.ProviderBinding{
				Provider: "matrix",
				Config:   v1alpha1.ConfigRef{Name: "work-channel", Key: "roomId"},
			},
		},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, cm, member, collab).Build()
	storage := &workspaceStorage{files: map[string][]byte{
		"agentorgs/worker/openclaw.json": []byte(`{"channels":{"matrix":{"enabled":false}}}`),
	}}
	adapter := openclawadapter.NewAdapter(config.Config{MatrixHomeserver: "http://hs"}, storage, k8s)

	if err := adapter.Apply(context.Background(), provider.MemberContext{
		Namespace: "agentorgs",
		Name:      "worker",
		Spec:      member.Spec,
	}); err != nil {
		t.Fatal(err)
	}

	raw := storage.files["agentorgs/worker/openclaw.json"]
	var cfg map[string]interface{}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	groups := cfg["channels"].(map[string]interface{})["matrix"].(map[string]interface{})["groups"].(map[string]interface{})
	if groups["*"].(map[string]interface{})["requireMention"] != true {
		t.Fatalf("missing wildcard requireMention: %v", groups)
	}
	room := groups[roomID].(map[string]interface{})
	if room["requireMention"] != true {
		t.Fatalf("exact room requireMention=%v groups=%v", room["requireMention"], groups)
	}
}

func TestApplyWaitsForCollaborationRoom(t *testing.T) {
	scheme := testScheme(t)
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "matrix-worker", Namespace: "agentorgs"},
		Data: map[string][]byte{
			"userId":      []byte("@worker:matrix-local.agentorgs.io"),
			"accessToken": []byte("tok-worker"),
		},
	}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "work-channel", Namespace: "agentorgs"},
		Data:       map[string]string{"roomId": "!placeholder:example.org"},
	}
	member := &v1alpha1.Member{
		ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "agentorgs"},
	}
	collab := &v1alpha1.Collaboration{
		ObjectMeta: metav1.ObjectMeta{Name: "work", Namespace: "agentorgs"},
		Spec: v1alpha1.CollaborationSpec{
			Participants: []v1alpha1.CollaborationParticipant{
				{Who: v1alpha1.ObjectRef{Kind: v1alpha1.MemberKind, Name: "worker"}},
			},
			Channel: v1alpha1.ProviderBinding{
				Provider: "matrix",
				Config:   v1alpha1.ConfigRef{Name: "work-channel", Key: "roomId"},
			},
		},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, cm, member, collab).Build()
	storage := &workspaceStorage{files: map[string][]byte{
		"agentorgs/worker/openclaw.json": []byte(`{}`),
	}}
	adapter := openclawadapter.NewAdapter(config.Config{}, storage, k8s)

	err := adapter.Apply(context.Background(), provider.MemberContext{
		Namespace: "agentorgs",
		Name:      "worker",
		Spec: v1alpha1.MemberSpec{
			Channels: []v1alpha1.ProviderBinding{{Provider: "matrix"}},
		},
	})
	if err == nil {
		t.Fatal("expected wait for room")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Fatalf("err=%v", err)
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
