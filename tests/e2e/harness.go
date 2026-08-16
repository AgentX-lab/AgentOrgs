//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type env struct {
	Namespace    string
	MatrixURL    string
	LeadMXID     string
	WorkerMXID   string
	GroupMXID    string
	ExpectedText string
	PeerOKText   string
}

// swarmEnv is the flat All-strategy e2e org (peer-a / peer-b / swarm-team).
type swarmEnv struct {
	Namespace    string
	MatrixURL    string
	PeerAMXID    string
	PeerBMXID    string
	GroupMXID    string
	ExpectedText string
}

const stepTimeout = 5 * time.Minute

func loadEnv() env {
	ns := getenv("AGENTORGS_NAMESPACE", "agentorgs")
	domain := getenv("AGENTORGS_MATRIX_DOMAIN", "matrix-local.agentorgs.io")
	return env{
		Namespace:    ns,
		MatrixURL:    strings.TrimRight(getenv("AGENTORGS_MATRIX_URL", "http://127.0.0.1:18080"), "/"),
		LeadMXID:     "@lead:" + domain,
		WorkerMXID:   "@worker:" + domain,
		GroupMXID:    "@e2e-team:" + domain,
		ExpectedText: getenv("AGENTORGS_E2E_EXPECTED_TEXT", "agentorgs-e2e-ok"),
		PeerOKText:   getenv("AGENTORGS_E2E_PEER_OK_TEXT", "agentorgs-e2e-peer-ok"),
	}
}

func loadSwarmEnv() swarmEnv {
	ns := getenv("AGENTORGS_NAMESPACE", "agentorgs")
	domain := getenv("AGENTORGS_MATRIX_DOMAIN", "matrix-local.agentorgs.io")
	return swarmEnv{
		Namespace:    ns,
		MatrixURL:    strings.TrimRight(getenv("AGENTORGS_MATRIX_URL", "http://127.0.0.1:18080"), "/"),
		PeerAMXID:    "@peer-a:" + domain,
		PeerBMXID:    "@peer-b:" + domain,
		GroupMXID:    "@swarm-team:" + domain,
		ExpectedText: getenv("AGENTORGS_E2E_EXPECTED_TEXT", "agentorgs-e2e-ok"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func kubeClient(t *testing.T) *kubernetes.Clientset {
	t.Helper()
	loading := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		// In-cluster fallback (unlikely for kind e2e host runner).
		cfg, err = rest.InClusterConfig()
	}
	if err != nil {
		t.Fatalf("kubeconfig: %v", err)
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kubernetes client: %v", err)
	}
	return cs
}

func waitUntil(t *testing.T, timeout time.Duration, desc string, fn func(context.Context) (bool, error)) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	var lastErr error
	for {
		ok, err := fn(ctx)
		if err == nil && ok {
			return
		}
		if err != nil {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			if lastErr != nil {
				t.Fatalf("timeout waiting for %s: %v", desc, lastErr)
			}
			t.Fatalf("timeout waiting for %s", desc)
		case <-ticker.C:
		}
	}
}

func secretString(ctx context.Context, cs *kubernetes.Clientset, ns, name, key string) (string, error) {
	sec, err := cs.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	raw, ok := sec.Data[key]
	if !ok || len(raw) == 0 {
		return "", fmt.Errorf("secret %s missing key %s", name, key)
	}
	return string(raw), nil
}

func configMapString(ctx context.Context, cs *kubernetes.Clientset, ns, name, key string) (string, error) {
	cm, err := cs.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	v := cm.Data[key]
	if v == "" {
		return "", fmt.Errorf("configmap %s missing key %s", name, key)
	}
	return v, nil
}

func memberPhase(ns, name string) (string, error) {
	cmd := exec.Command("kubectl", "-n", ns, "get", "member", name, "-o", "jsonpath={.status.phase}")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("kubectl get member %s: %w (%s)", name, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func kubectlExec(ns, pod string, args ...string) (string, error) {
	cmdArgs := append([]string{"-n", ns, "exec", pod, "--"}, args...)
	cmd := exec.Command("kubectl", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("kubectl exec %s: %w (%s)", pod, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func kubectlPatchCollaborationStrategy(ns, name, strategy string) error {
	patch := fmt.Sprintf(`{"spec":{"whenTargetIsGroup":{"strategy":%q}}}`, strategy)
	cmd := exec.Command("kubectl", "-n", ns, "patch", "collaboration", name, "--type=merge", "-p", patch)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("kubectl patch collaboration %s: %w (%s)", name, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

type matrixClient struct {
	base  string
	token string
	http  *http.Client
}

func (c *matrixClient) sendMention(ctx context.Context, roomID, body string, mentionUserIDs []string) error {
	payload := map[string]interface{}{
		"msgtype": "m.text",
		"body":    body,
	}
	if len(mentionUserIDs) > 0 {
		var plain, html strings.Builder
		for i, user := range mentionUserIDs {
			if i > 0 {
				plain.WriteByte(' ')
				html.WriteByte(' ')
			}
			plain.WriteString(user)
			html.WriteString(`<a href="https://matrix.to/#/`)
			html.WriteString(url.PathEscape(user))
			html.WriteString(`">`)
			html.WriteString(user)
			html.WriteString(`</a>`)
		}
		plain.WriteByte(' ')
		plain.WriteString(body)
		html.WriteByte(' ')
		html.WriteString(body)
		payload["body"] = plain.String()
		payload["format"] = "org.matrix.custom.html"
		payload["formatted_body"] = html.String()
		payload["m.mentions"] = map[string]interface{}{"user_ids": mentionUserIDs}
	}
	txn := fmt.Sprintf("e2e-%d", time.Now().UnixNano())
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", url.PathEscape(roomID), url.PathEscape(txn))
	return c.doJSON(ctx, http.MethodPut, path, payload, nil)
}

func (c *matrixClient) sendPlain(ctx context.Context, roomID, body string) error {
	return c.sendMention(ctx, roomID, body, nil)
}

func senderHasText(events []matrixEvent, sender, text string) bool {
	for _, ev := range events {
		if ev.Type != "m.room.message" {
			continue
		}
		if ev.Sender == sender && strings.Contains(ev.Content.Body, text) {
			return true
		}
	}
	return false
}

func senderMessageCount(events []matrixEvent, sender string) int {
	n := 0
	for _, ev := range events {
		if ev.Type == "m.room.message" && ev.Sender == sender {
			n++
		}
	}
	return n
}

func (c *matrixClient) recentMessages(ctx context.Context, roomID string) ([]matrixEvent, error) {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/messages?dir=b&limit=50", url.PathEscape(roomID))
	var resp struct {
		Chunk []matrixEvent `json:"chunk"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Chunk, nil
}

type matrixEvent struct {
	Type    string `json:"type"`
	Sender  string `json:"sender"`
	Content struct {
		Body string `json:"body"`
	} `json:"content"`
}

func (c *matrixClient) doJSON(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("matrix %s %s: %s (%s)", method, path, resp.Status, string(raw))
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return err
		}
	}
	return nil
}
