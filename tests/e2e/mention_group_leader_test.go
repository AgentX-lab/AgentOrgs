//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// TestMentionGroupLeader covers Leader-mode @ semantics and craft workspaces:
//  0. lead/worker SOUL differ; skill packs differ (coordination vs backend)
//  1. human @Group → only lead replies (worker silent)
//  2. test sends visible @worker as lead → worker replies
//  3. lead plain text (no @) → worker stays silent
//  4. human @Group with E2E_GROUP_TASK → mock-llm makes lead @worker → worker peer-ok
//  5. All + @Group -@lead → dispatch intent keeps worker only
func TestMentionGroupLeader(t *testing.T) {
	e := loadEnv()
	cs := kubeClient(t)
	ctx := context.Background()
	roomID := waitMentionGroupLeaderReady(t, e)

	if !t.Run("workspace_soul_and_skills_differ", func(t *testing.T) {
		assertDistinctWorkspaces(t, e)
	}) {
		return
	}

	requesterTok, err := secretString(ctx, cs, e.Namespace, "matrix-requester", "accessToken")
	if err != nil {
		t.Fatalf("requester token: %v", err)
	}
	leadTok, err := secretString(ctx, cs, e.Namespace, "matrix-lead", "accessToken")
	if err != nil {
		t.Fatalf("lead token: %v", err)
	}

	reqMX := &matrixClient{base: e.MatrixURL, token: requesterTok, http: &http.Client{Timeout: 20 * time.Second}}
	leadMX := &matrixClient{base: e.MatrixURL, token: leadTok, http: &http.Client{Timeout: 20 * time.Second}}

	beforeEvents, err := reqMX.recentMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	workerBeforeGroup := senderMessageCount(beforeEvents, e.WorkerMXID)

	marker := "E2E_TOKEN_GROUP_WAKE"
	if err := reqMX.sendMention(ctx, roomID, marker+" please reply", []string{e.GroupMXID}); err != nil {
		t.Fatalf("human @Group: %v", err)
	}

	if !t.Run("only_lead_after_at_group", func(t *testing.T) {
		waitUntil(t, stepTimeout, "lead reply after @Group", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			return senderHasText(events, e.LeadMXID, e.ExpectedText), nil
		})
		events, err := reqMX.recentMessages(ctx, roomID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		if senderMessageCount(events, e.WorkerMXID) > workerBeforeGroup {
			t.Fatal("worker replied after @Group; want lead only")
		}
	}) {
		return
	}

	eventsAfterLead, err := reqMX.recentMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	workerCountBefore := senderMessageCount(eventsAfterLead, e.WorkerMXID)

	peerMarker := "E2E_TOKEN_PEER_WAKE"
	if err := leadMX.sendMention(ctx, roomID, peerMarker+" please reply", []string{e.WorkerMXID}); err != nil {
		t.Fatalf("lead @worker: %v", err)
	}

	if !t.Run("worker_after_lead_mention", func(t *testing.T) {
		waitUntil(t, stepTimeout, "worker reply after lead @", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			return senderMessageCount(events, e.WorkerMXID) > workerCountBefore &&
				senderHasText(events, e.WorkerMXID, e.ExpectedText), nil
		})
	}) {
		return
	}

	workerMid, err := reqMX.recentMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	workerCountMid := senderMessageCount(workerMid, e.WorkerMXID)

	if err := leadMX.sendPlain(ctx, roomID, "E2E_PLAIN_NO_MENTION status only"); err != nil {
		t.Fatalf("lead plain: %v", err)
	}

	if !t.Run("worker_silent_without_mention", func(t *testing.T) {
		deadline := time.Now().Add(45 * time.Second)
		for time.Now().Before(deadline) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				t.Fatalf("list messages: %v", err)
			}
			if senderMessageCount(events, e.WorkerMXID) > workerCountMid {
				t.Fatal("worker replied to plain text without @")
			}
			time.Sleep(3 * time.Second)
		}
	}) {
		return
	}

	beforePeer, err := reqMX.recentMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("list messages: %v", err)
	}
	peerOKBefore := 0
	for _, ev := range beforePeer {
		if ev.Type == "m.room.message" && ev.Sender == e.WorkerMXID && strings.Contains(ev.Content.Body, e.PeerOKText) {
			peerOKBefore++
		}
	}

	if err := reqMX.sendMention(ctx, roomID, "E2E_GROUP_TASK please coordinate", []string{e.GroupMXID}); err != nil {
		t.Fatalf("human @Group for mock peer: %v", err)
	}

	if !t.Run("lead_posts_worker_mention", func(t *testing.T) {
		waitUntil(t, stepTimeout, "lead message mentions worker", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			for _, ev := range events {
				if ev.Type != "m.room.message" || ev.Sender != e.LeadMXID {
					continue
				}
				if strings.Contains(ev.Content.Body, e.WorkerMXID) && strings.Contains(ev.Content.Body, "E2E_PEER_TASK") {
					return true, nil
				}
			}
			return false, nil
		})
	}) {
		return
	}

	if !t.Run("worker_peer_ok", func(t *testing.T) {
		waitUntil(t, stepTimeout, "worker peer-ok reply", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			n := 0
			for _, ev := range events {
				if ev.Type == "m.room.message" && ev.Sender == e.WorkerMXID && strings.Contains(ev.Content.Body, e.PeerOKText) {
					n++
				}
			}
			return n > peerOKBefore, nil
		})
	}) {
		return
	}

	if !t.Run("dispatch_intent_exclude_lead", func(t *testing.T) {
		if err := kubectlPatchCollaborationStrategy(e.Namespace, "mention-group-leader", "All"); err != nil {
			t.Fatalf("switch to All: %v", err)
		}
		t.Cleanup(func() {
			_ = kubectlPatchCollaborationStrategy(e.Namespace, "mention-group-leader", "Leader")
		})

		before, err := reqMX.recentMessages(ctx, roomID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		leadBefore := senderMessageCount(before, e.LeadMXID)
		workerBefore := senderMessageCount(before, e.WorkerMXID)

		// All would wake both; -@lead keeps only worker (proves exclude ≠ Leader strategy).
		body := "E2E_INTENT_EXCLUDE_WAKE -@lead please reply"
		if err := reqMX.sendMention(ctx, roomID, body, []string{e.GroupMXID}); err != nil {
			t.Fatalf("human @Group -@lead: %v", err)
		}

		waitUntil(t, stepTimeout, "worker reply after @Group -@lead", func(ctx context.Context) (bool, error) {
			events, err := reqMX.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			return senderMessageCount(events, e.WorkerMXID) > workerBefore &&
				senderHasText(events, e.WorkerMXID, e.ExpectedText), nil
		})

		events, err := reqMX.recentMessages(ctx, roomID)
		if err != nil {
			t.Fatalf("list messages: %v", err)
		}
		if senderMessageCount(events, e.LeadMXID) > leadBefore {
			t.Fatal("lead replied after @Group -@lead; want worker only (dispatch intent)")
		}
	}) {
		return
	}
}

func assertDistinctWorkspaces(t *testing.T, e env) {
	t.Helper()
	leadSoul, err := kubectlExec(e.Namespace, "member-lead", "cat", "/workspace/SOUL.md")
	if err != nil {
		t.Fatalf("lead SOUL: %v", err)
	}
	workerSoul, err := kubectlExec(e.Namespace, "member-worker", "cat", "/workspace/SOUL.md")
	if err != nil {
		t.Fatalf("worker SOUL: %v", err)
	}
	if !strings.Contains(leadSoul, "E2E Lead") {
		t.Fatalf("lead SOUL missing display name:\n%s", leadSoul)
	}
	if !strings.Contains(workerSoul, "E2E Backend Worker") {
		t.Fatalf("worker SOUL missing display name:\n%s", workerSoul)
	}
	if leadSoul == workerSoul {
		t.Fatal("lead and worker SOUL.md must differ")
	}

	leadSkills, err := kubectlExec(e.Namespace, "member-lead", "ls", "/workspace/skills")
	if err != nil {
		t.Fatalf("lead skills: %v", err)
	}
	workerSkills, err := kubectlExec(e.Namespace, "member-worker", "ls", "/workspace/skills")
	if err != nil {
		t.Fatalf("worker skills: %v", err)
	}
	if !strings.Contains(leadSkills, "team-coordination") {
		t.Fatalf("lead missing team-coordination skill, got: %s", leadSkills)
	}
	if !strings.Contains(workerSkills, "backend-dev") {
		t.Fatalf("worker missing backend-dev skill, got: %s", workerSkills)
	}
	if strings.Contains(leadSkills, "backend-dev") {
		t.Fatalf("lead should not have backend-dev, got: %s", leadSkills)
	}
	if strings.Contains(workerSkills, "team-coordination") {
		t.Fatalf("worker should not have team-coordination, got: %s", workerSkills)
	}
}

func waitMentionGroupLeaderReady(t *testing.T, e env) string {
	t.Helper()
	cs := kubeClient(t)

	var roomID string
	waitUntil(t, stepTimeout, "group collaboration roomId", func(ctx context.Context) (bool, error) {
		v, err := configMapString(ctx, cs, e.Namespace, "mention-group-leader-channel", "roomId")
		if err != nil {
			return false, err
		}
		if strings.Contains(v, "example.org") {
			return false, nil
		}
		roomID = v
		return true, nil
	})
	t.Logf("roomId=%s", roomID)

	for _, sec := range []string{"matrix-requester", "matrix-lead", "matrix-worker"} {
		name := sec
		waitUntil(t, stepTimeout, name+" secret", func(ctx context.Context) (bool, error) {
			_, err := secretString(ctx, cs, e.Namespace, name, "accessToken")
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return err == nil, err
		})
	}

	for _, member := range []string{"lead", "worker"} {
		name := member
		waitUntil(t, stepTimeout, "member "+name+" Ready", func(ctx context.Context) (bool, error) {
			phase, err := memberPhase(e.Namespace, name)
			if err != nil {
				return false, err
			}
			return phase == "Ready", nil
		})
	}

	return roomID
}
