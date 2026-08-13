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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type env struct {
	Namespace     string
	MatrixURL     string
	MatrixDomain  string
	WorkerMXID    string
	RequesterMXID string
	ExpectedText  string
	Timeout       time.Duration
}

func loadEnv() env {
	ns := getenv("AGENTORGS_NAMESPACE", "agentorgs")
	domain := getenv("AGENTORGS_MATRIX_DOMAIN", "matrix-local.agentorgs.io")
	timeoutSecs := 600
	if v := os.Getenv("AGENTORGS_E2E_TIMEOUT_SECS"); v != "" {
		fmt.Sscanf(v, "%d", &timeoutSecs)
	}
	return env{
		Namespace:     ns,
		MatrixURL:     strings.TrimRight(getenv("AGENTORGS_MATRIX_URL", "http://127.0.0.1:18080"), "/"),
		MatrixDomain:  domain,
		WorkerMXID:    "@worker:" + domain,
		RequesterMXID: "@requester:" + domain,
		ExpectedText:  getenv("AGENTORGS_E2E_EXPECTED_TEXT", "agentorgs-e2e-ok"),
		Timeout:       time.Duration(timeoutSecs) * time.Second,
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

func deletePod(ctx context.Context, cs *kubernetes.Clientset, ns, name string) {
	err := cs.CoreV1().Pods(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		// Best-effort restart; test will wait for Running.
		fmt.Fprintf(os.Stderr, "delete pod %s: %v\n", name, err)
	}
}

func podPhase(ctx context.Context, cs *kubernetes.Clientset, ns, name string) (corev1.PodPhase, error) {
	pod, err := cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return pod.Status.Phase, nil
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

type matrixClient struct {
	base   string
	token  string
	http   *http.Client
}

func (c *matrixClient) sendMention(ctx context.Context, roomID, body string, mentionUserIDs []string) error {
	payload := map[string]interface{}{
		"msgtype": "m.text",
		"body":    body,
	}
	if len(mentionUserIDs) > 0 {
		payload["m.mentions"] = map[string]interface{}{"user_ids": mentionUserIDs}
	}
	txn := fmt.Sprintf("e2e-%d", time.Now().UnixNano())
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", url.PathEscape(roomID), url.PathEscape(txn))
	return c.doJSON(ctx, http.MethodPut, path, payload, nil)
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

func dumpDiagnostics(t *testing.T, ns string) {
	t.Helper()
	cmds := [][]string{
		{"get", "pods", "-o", "wide"},
		{"logs", "deploy/agentorgs-controller", "--tail=80"},
		{"logs", "member-worker", "--tail=120"},
		{"logs", "deploy/mock-llm", "--tail=40"},
	}
	for _, args := range cmds {
		cmd := exec.Command("kubectl", append([]string{"-n", ns}, args...)...)
		out, _ := cmd.CombinedOutput()
		t.Logf("kubectl %s:\n%s", strings.Join(args, " "), string(out))
	}
}
