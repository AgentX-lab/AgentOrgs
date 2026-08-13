package collaboration_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/collaboration"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/provider"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// --- fakes ---

type fakeReader struct {
	members []agentorgsv1alpha1.Member
	groups  []agentorgsv1alpha1.Group
	collab  agentorgsv1alpha1.Collaboration
	pols    []agentorgsv1alpha1.Policy
}

func (f *fakeReader) ListMembers(context.Context, string) ([]agentorgsv1alpha1.Member, error) {
	return f.members, nil
}
func (f *fakeReader) ListGroups(context.Context, string) ([]agentorgsv1alpha1.Group, error) {
	return f.groups, nil
}
func (f *fakeReader) ListCollaborations(context.Context, string) ([]agentorgsv1alpha1.Collaboration, error) {
	return []agentorgsv1alpha1.Collaboration{f.collab}, nil
}
func (f *fakeReader) ListPolicies(context.Context, string) ([]agentorgsv1alpha1.Policy, error) {
	return f.pols, nil
}
func (f *fakeReader) GetCollaboration(_ context.Context, _, _ string) (agentorgsv1alpha1.Collaboration, error) {
	return f.collab, nil
}
func (f *fakeReader) GetMember(_ context.Context, _, name string) (agentorgsv1alpha1.Member, error) {
	for _, m := range f.members {
		if m.Name == name {
			return m, nil
		}
	}
	return agentorgsv1alpha1.Member{}, fmt.Errorf("member %q not found", name)
}

type memStorage struct {
	mu     sync.Mutex
	runs   map[string]protocol.CollaborationRun
	events []protocol.CollaborationEvent
}

func newMemStorage() *memStorage {
	return &memStorage{runs: map[string]protocol.CollaborationRun{}}
}
func (s *memStorage) Name() string { return "memory" }
func (s *memStorage) WriteRun(_ context.Context, run protocol.CollaborationRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.RunID] = run
	return nil
}
func (s *memStorage) ReadRun(_ context.Context, _, runID string) (protocol.CollaborationRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[runID]
	if !ok {
		return protocol.CollaborationRun{}, fmt.Errorf("run not found")
	}
	return run, nil
}
func (s *memStorage) WriteEvent(_ context.Context, event protocol.CollaborationEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}
func (s *memStorage) ListEvents(_ context.Context, _, runID string) ([]protocol.CollaborationEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []protocol.CollaborationEvent
	for _, e := range s.events {
		if e.RunID == runID {
			out = append(out, e)
		}
	}
	return out, nil
}
func (s *memStorage) EnsureMemberWorkspace(context.Context, string, string, string) error {
	return nil
}
func (s *memStorage) GetWorkspaceFile(context.Context, string, string, string) ([]byte, error) {
	return nil, fmt.Errorf("not implemented")
}
func (s *memStorage) PutWorkspaceFile(context.Context, string, string, string, []byte) error {
	return nil
}

type fakeRuntime struct {
	mu    sync.Mutex
	calls []string
}

func (r *fakeRuntime) Name() string { return "openclaw" }
func (r *fakeRuntime) Apply(context.Context, provider.MemberContext) error {
	return nil
}
func (r *fakeRuntime) Delete(context.Context, provider.MemberContext) error {
	return nil
}
func (r *fakeRuntime) SendRequest(_ context.Context, member provider.MemberContext, _ protocol.CollaborationEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, member.Name)
	return nil
}

type fakeChannel struct {
	delivered int
}

func (c *fakeChannel) Name() string { return "matrix" }
func (c *fakeChannel) Deliver(context.Context, protocol.CollaborationEvent) error {
	c.delivered++
	return nil
}
func (c *fakeChannel) Subscribe(context.Context, provider.EventHandler) error {
	return nil
}

func agentMember(name string) agentorgsv1alpha1.Member {
	return agentorgsv1alpha1.Member{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "agentorgs"},
		Spec: agentorgsv1alpha1.MemberSpec{
			Type: agentorgsv1alpha1.MemberTypeAgent,
			Runtime: &agentorgsv1alpha1.ProviderBinding{
				Provider: "openclaw",
			},
		},
	}
}

func fixture() (*fakeReader, *fakeRuntime, *fakeChannel, *collaboration.Engine) {
	reader := &fakeReader{
		members: []agentorgsv1alpha1.Member{
			{ObjectMeta: metav1.ObjectMeta{Name: "product-owner"}, Spec: agentorgsv1alpha1.MemberSpec{Type: agentorgsv1alpha1.MemberTypeHuman}},
			agentMember("be-1"),
			agentMember("be-2"),
			agentMember("qa-1"),
			agentMember("qa-2"),
			agentMember("architect"),
		},
		groups: []agentorgsv1alpha1.Group{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "backend-team"},
				Spec: agentorgsv1alpha1.GroupSpec{
					Members: []agentorgsv1alpha1.GroupMember{
						{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "be-1"}, Role: "Developer"},
						{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "be-2"}, Role: "Developer"},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "qa-team"},
				Spec: agentorgsv1alpha1.GroupSpec{
					Members: []agentorgsv1alpha1.GroupMember{
						{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "qa-1"}, Role: "Tester"},
						{Who: agentorgsv1alpha1.ObjectRef{Kind: agentorgsv1alpha1.MemberKind, Name: "qa-2"}, Role: "Tester"},
					},
				},
			},
		},
		collab: agentorgsv1alpha1.Collaboration{
			ObjectMeta: metav1.ObjectMeta{Name: "cross-team-work", Namespace: "agentorgs"},
			Spec: agentorgsv1alpha1.CollaborationSpec{
				Channel: agentorgsv1alpha1.ProviderBinding{Provider: "matrix"},
				WhenTargetIsGroup: agentorgsv1alpha1.WhenTargetIsGroup{
					Strategy: agentorgsv1alpha1.GroupTargetAll,
				},
				Limits: agentorgsv1alpha1.CollaborationLimits{MaxReplyRounds: 5, TimeoutMinutes: 10},
			},
		},
		pols: []agentorgsv1alpha1.Policy{{
			ObjectMeta: metav1.ObjectMeta{Name: "allow"},
			Spec: agentorgsv1alpha1.PolicySpec{
				Effect: agentorgsv1alpha1.PolicyEffectAllow,
				From:   []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.MemberKind, Name: "product-owner"}},
				To: []agentorgsv1alpha1.ObjectRef{
					{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"},
					{Kind: agentorgsv1alpha1.GroupKind, Name: "qa-team"},
					{Kind: agentorgsv1alpha1.MemberKind, Name: "architect"},
				},
				Actions: []agentorgsv1alpha1.PolicyAction{agentorgsv1alpha1.PolicyActionStart, agentorgsv1alpha1.PolicyActionContinue},
			},
		}},
	}

	rt := &fakeRuntime{}
	ch := &fakeChannel{}
	reg := provider.NewRegistry()
	reg.RegisterRuntime(rt)
	reg.RegisterCollaboration(ch)
	reg.RegisterStorage(newMemStorage())

	engine := &collaboration.Engine{Registry: reg, Client: reader}
	return reader, rt, ch, engine
}

func TestStartCollaborationTwoGroupsAndMember(t *testing.T) {
	_, rt, ch, engine := fixture()
	run, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{
			{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"},
			{Kind: agentorgsv1alpha1.GroupKind, Name: "qa-team"},
			{Kind: agentorgsv1alpha1.MemberKind, Name: "architect"},
		},
		map[string]interface{}{"text": "联调登录接口"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != protocol.RunStatusRunning {
		t.Fatalf("status=%s", run.Status)
	}
	if len(run.ResolvedTargets) != 5 {
		t.Fatalf("ResolvedTargets=%v want 5", run.ResolvedTargets)
	}
	want := map[string]bool{"architect": true, "be-1": true, "be-2": true, "qa-1": true, "qa-2": true}
	for _, name := range run.ResolvedTargets {
		if !want[name] {
			t.Fatalf("unexpected target %q", name)
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("missing targets %v", want)
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.calls) != 5 {
		t.Fatalf("runtime calls=%v want 5", rt.calls)
	}
	if ch.delivered != 1 {
		t.Fatalf("channel delivered=%d want 1", ch.delivered)
	}
}

func TestStartCollaborationPolicyDeny(t *testing.T) {
	reader, _, _, engine := fixture()
	reader.pols = []agentorgsv1alpha1.Policy{{
		ObjectMeta: metav1.ObjectMeta{Name: "deny-architect"},
		Spec: agentorgsv1alpha1.PolicySpec{
			Effect:  agentorgsv1alpha1.PolicyEffectAllow,
			From:    []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.MemberKind, Name: "product-owner"}},
			To:      []agentorgsv1alpha1.ObjectRef{{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"}},
			Actions: []agentorgsv1alpha1.PolicyAction{agentorgsv1alpha1.PolicyActionStart},
		},
	}}
	_, err := engine.StartCollaboration(context.Background(), "agentorgs", "cross-team-work", "product-owner",
		[]protocol.ObjectTarget{
			{Kind: agentorgsv1alpha1.GroupKind, Name: "backend-team"},
			{Kind: agentorgsv1alpha1.MemberKind, Name: "architect"},
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected policy deny")
	}
}
