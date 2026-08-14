//go:build e2e

package e2e

import (
	"context"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// TestOpenClawMemberMatrixReply asserts:
// 1) worker workspace has SOUL/AGENTS/skills from controller templates
// 2) Matrix receives the mock-llm reply from the OpenClaw member
func TestOpenClawMemberMatrixReply(t *testing.T) {
	e := loadEnv()
	cs := kubeClient(t)
	ctx := context.Background()

	t.Cleanup(func() {
		if t.Failed() {
			dumpDiagnostics(t, e.Namespace)
		}
	})

	var roomID string
	waitUntil(t, stepTimeout, "collaboration roomId", func(ctx context.Context) (bool, error) {
		v, err := configMapString(ctx, cs, e.Namespace, "e2e-channel", "roomId")
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

	waitUntil(t, stepTimeout, "matrix-worker secret", func(ctx context.Context) (bool, error) {
		_, err := secretString(ctx, cs, e.Namespace, "matrix-worker", "accessToken")
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return err == nil, err
	})
	waitUntil(t, stepTimeout, "matrix-requester secret", func(ctx context.Context) (bool, error) {
		_, err := secretString(ctx, cs, e.Namespace, "matrix-requester", "accessToken")
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return err == nil, err
	})

	waitUntil(t, stepTimeout, "member-worker Running", func(ctx context.Context) (bool, error) {
		phase, err := podPhase(ctx, cs, e.Namespace, "member-worker")
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return phase == corev1.PodRunning, nil
	})

	t.Run("workspace_templates", func(t *testing.T) {
		waitUntil(t, stepTimeout, "worker workspace templates", func(ctx context.Context) (bool, error) {
			soul, err := kubectlExec(e.Namespace, "member-worker", "cat", "/workspace/SOUL.md")
			if err != nil {
				return false, err
			}
			agents, err := kubectlExec(e.Namespace, "member-worker", "cat", "/workspace/AGENTS.md")
			if err != nil {
				return false, err
			}
			skill, err := kubectlExec(e.Namespace, "member-worker", "cat", "/workspace/skills/workspace-sync/SKILL.md")
			if err != nil {
				return false, err
			}
			if _, err := kubectlExec(e.Namespace, "member-worker", "test", "-f", "/workspace/openclaw.json"); err != nil {
				return false, err
			}
			ok := strings.Contains(soul, "You are E2E OpenClaw Worker") &&
				strings.Contains(agents, "Read `SOUL.md`") &&
				strings.Contains(agents, "skills/") &&
				strings.Contains(skill, "workspace-sync")
			return ok, nil
		})
	})

	token, err := secretString(ctx, cs, e.Namespace, "matrix-requester", "accessToken")
	if err != nil {
		t.Fatalf("requester token: %v", err)
	}
	mx := &matrixClient{
		base:  e.MatrixURL,
		token: token,
		http:  &http.Client{Timeout: 20 * time.Second},
	}

	body := "Please reply with the model answer. " + e.WorkerMXID
	if err := mx.sendMention(ctx, roomID, body, []string{e.WorkerMXID}); err != nil {
		t.Fatalf("send mention: %v", err)
	}

	t.Run("matrix_mock_reply", func(t *testing.T) {
		waitUntil(t, stepTimeout, "worker matrix reply", func(ctx context.Context) (bool, error) {
			events, err := mx.recentMessages(ctx, roomID)
			if err != nil {
				return false, err
			}
			for _, ev := range events {
				if ev.Type != "m.room.message" {
					continue
				}
				if ev.Sender == e.WorkerMXID && strings.Contains(ev.Content.Body, e.ExpectedText) {
					return true, nil
				}
			}
			return false, nil
		})
	})
}
