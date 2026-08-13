package matrix

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/config"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Setup prepares Matrix for AgentOrgs: push registration, users, and collaboration rooms.
type Setup struct {
	Config config.Config
	K8s    client.Client
	API    *Client
}

func NewSetup(cfg config.Config, k8s client.Client, api *Client) *Setup {
	return &Setup{Config: cfg, K8s: k8s, API: api}
}

// EnsureReady runs first-time Matrix setup for the configured namespace.
func (s *Setup) EnsureReady(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("matrix-setup")
	if !s.Config.MatrixBootstrapEnabled {
		logger.Info("matrix setup disabled")
		return nil
	}
	if err := s.waitUntilHomeserverReady(ctx); err != nil {
		return err
	}
	if err := s.registerHomeserverPush(ctx); err != nil {
		logger.Info("homeserver push registration skipped or failed; continuing if tokens already work", "err", err.Error())
	}

	ns := s.Config.Namespace
	var members agentorgsv1alpha1.MemberList
	if err := s.K8s.List(ctx, &members, client.InNamespace(ns)); err != nil {
		return err
	}
	var groups agentorgsv1alpha1.GroupList
	if err := s.K8s.List(ctx, &groups, client.InNamespace(ns)); err != nil {
		return err
	}
	var collabs agentorgsv1alpha1.CollaborationList
	if err := s.K8s.List(ctx, &collabs, client.InNamespace(ns)); err != nil {
		return err
	}

	invite := []string{}
	for i := range members.Items {
		m := &members.Items[i]
		localpart, userID := s.readMatrixUserFromChannels(ctx, m.Name, m.Spec.Channels)
		if userID == "" {
			continue
		}
		token, err := s.ensureMatrixUser(ctx, localpart)
		if err != nil {
			logger.Error(err, "ensure member matrix user", "member", m.Name)
			continue
		}
		invite = append(invite, userID)
		_ = s.saveAccessTokenSecret(ctx, ns, m.Name, userID, token)
		if m.Status.MatrixUserID != userID {
			m.Status.MatrixUserID = userID
			_ = s.K8s.Status().Update(ctx, m)
		}
	}
	for i := range groups.Items {
		g := &groups.Items[i]
		localpart, userID := s.readMatrixUserFromChannels(ctx, g.Name, g.Spec.Channels)
		if userID == "" {
			continue
		}
		token, err := s.ensureMatrixUser(ctx, localpart)
		if err != nil {
			logger.Error(err, "ensure group matrix user", "group", g.Name)
			continue
		}
		invite = append(invite, userID)
		_ = s.saveAccessTokenSecret(ctx, ns, "group-"+g.Name, userID, token)
		if g.Status.MatrixUserID != userID {
			g.Status.MatrixUserID = userID
			_ = s.K8s.Status().Update(ctx, g)
		}
	}

	operatorToken, err := s.getRoomOperatorToken(ctx)
	if err != nil {
		return fmt.Errorf("room operator token: %w", err)
	}

	for _, c := range collabs.Items {
		if c.Spec.Channel.Provider != "" && c.Spec.Channel.Provider != providerName {
			continue
		}
		cmName := c.Spec.Channel.Config.Name
		cmKey := c.Spec.Channel.Config.Key
		if cmKey == "" {
			cmKey = "roomId"
		}
		existing, _ := ReadConfigMapKey(ctx, s.K8s, ns, cmName, cmKey)
		if isFakeOrEmptyRoomID(existing) {
			existing = ""
		}
		if existing != "" {
			continue
		}
		alias := fmt.Sprintf("agentorgs-%s", c.Name)
		roomID, err := s.API.CreateRoom(ctx, operatorToken, c.Name, alias, invite)
		if err != nil {
			logger.Error(err, "create collaboration room", "collaboration", c.Name)
			continue
		}
		for _, userID := range invite {
			_ = s.API.Invite(ctx, operatorToken, roomID, userID)
			localpart := strings.TrimPrefix(strings.Split(userID, ":")[0], "@")
			userToken, tokErr := s.ensureMatrixUser(ctx, localpart)
			if tokErr == nil {
				_ = s.API.JoinRoom(ctx, userToken, roomID)
			}
		}
		if err := s.upsertConfigMapKey(ctx, ns, cmName, cmKey, roomID); err != nil {
			logger.Error(err, "write roomId", "configmap", cmName)
		} else {
			logger.Info("collaboration room ready", "collaboration", c.Name, "roomID", roomID)
		}
	}
	return nil
}

func isFakeOrEmptyRoomID(roomID string) bool {
	if roomID == "" {
		return true
	}
	return strings.Contains(roomID, "example.org")
}

func (s *Setup) waitUntilHomeserverReady(ctx context.Context) error {
	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.API.Homeserver+"/_matrix/client/versions", nil)
		if err != nil {
			return err
		}
		resp, err := s.API.HTTP.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
	return fmt.Errorf("matrix homeserver not ready: %s", s.API.Homeserver)
}

// registerHomeserverPush tells the homeserver to push room events to this controller.
func (s *Setup) registerHomeserverPush(ctx context.Context) error {
	if !s.Config.MatrixAppServiceEnabled {
		return nil
	}
	if s.Config.MatrixAppServiceASToken == "" || s.Config.MatrixAppServiceHSToken == "" {
		return fmt.Errorf("appservice tokens missing")
	}
	if _, err := s.API.LoginAppServiceUser(ctx, s.Config.MatrixAppServiceSenderLocalpart); err == nil {
		return nil
	}
	if s.Config.MatrixAdminPassword == "" {
		return fmt.Errorf("admin password not set; cannot register appservice via #admins")
	}
	adminToken, err := s.API.RegisterPassword(ctx, s.Config.MatrixAdminUser, s.Config.MatrixAdminPassword)
	if err != nil {
		return err
	}
	yamlBytes, err := RegistrationYAML(s.Config)
	if err != nil {
		return err
	}
	cmd := fmt.Sprintf("!admin appservices register\n```yaml\n%s```", string(yamlBytes))
	if err := s.API.AdminCommand(ctx, adminToken, cmd); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	_, err = s.API.LoginAppServiceUser(ctx, s.Config.MatrixAppServiceSenderLocalpart)
	return err
}

// ensureMatrixUser creates the Matrix account if needed and returns an access token.
func (s *Setup) ensureMatrixUser(ctx context.Context, localpart string) (string, error) {
	if s.Config.MatrixAppServiceASToken != "" {
		_, token, err := s.API.EnsureAppServiceUser(ctx, localpart)
		return token, err
	}
	pass := "agentorgs-" + localpart
	return s.API.RegisterPassword(ctx, localpart, pass)
}

// getRoomOperatorToken returns a token that can create rooms and invite users.
func (s *Setup) getRoomOperatorToken(ctx context.Context) (string, error) {
	if s.Config.MatrixAccessToken != "" {
		return s.Config.MatrixAccessToken, nil
	}
	if s.Config.MatrixAppServiceASToken != "" {
		return s.API.LoginAppServiceUser(ctx, s.Config.MatrixAppServiceSenderLocalpart)
	}
	if s.Config.MatrixAdminPassword != "" {
		return s.API.RegisterPassword(ctx, s.Config.MatrixAdminUser, s.Config.MatrixAdminPassword)
	}
	return "", fmt.Errorf("no matrix token available for setup")
}

// readMatrixUserFromChannels reads @user:domain from the matrix channel ConfigMap.
// localpart is the part before ":" (without "@").
func (s *Setup) readMatrixUserFromChannels(ctx context.Context, resourceName string, channels []agentorgsv1alpha1.ProviderBinding) (localpart, userID string) {
	for _, ch := range channels {
		if ch.Provider != providerName {
			continue
		}
		key := ch.Config.Key
		if key == "" {
			key = "userId"
		}
		userID, _ = ReadConfigMapKey(ctx, s.K8s, s.Config.Namespace, ch.Config.Name, key)
		if userID == "" {
			localpart = resourceName
			userID = s.API.UserID(localpart)
			return localpart, userID
		}
		localpart = strings.TrimPrefix(strings.Split(userID, ":")[0], "@")
		return localpart, userID
	}
	return "", ""
}

// saveAccessTokenSecret stores userId + accessToken for OpenClaw Runtime.Apply.
func (s *Setup) saveAccessTokenSecret(ctx context.Context, ns, name, userID, token string) error {
	secretName := fmt.Sprintf("matrix-%s", name)
	var existing corev1.Secret
	err := s.K8s.Get(ctx, client.ObjectKey{Namespace: ns, Name: secretName}, &existing)
	data := map[string][]byte{
		"userId":      []byte(userID),
		"accessToken": []byte(token),
	}
	if apierrors.IsNotFound(err) {
		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: ns},
			Type:       corev1.SecretTypeOpaque,
			Data:       data,
		}
		return s.K8s.Create(ctx, sec)
	}
	if err != nil {
		return err
	}
	existing.Data = data
	return s.K8s.Update(ctx, &existing)
}

func (s *Setup) upsertConfigMapKey(ctx context.Context, ns, name, key, value string) error {
	var cm corev1.ConfigMap
	if err := s.K8s.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			cm = corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
				Data:       map[string]string{key: value},
			}
			return s.K8s.Create(ctx, &cm)
		}
		return err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[key] = value
	return s.K8s.Update(ctx, &cm)
}
