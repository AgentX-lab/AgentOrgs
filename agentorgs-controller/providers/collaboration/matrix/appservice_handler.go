package matrix

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	agentorgsv1alpha1 "github.com/agentscope-ai/AgentOrgs/agentorgs-controller/api/v1alpha1"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/internal/organization"
	"github.com/agentscope-ai/AgentOrgs/agentorgs-controller/pkg/protocol"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// AppServiceHandler receives homeserver transaction pushes.
//
// One Matrix message with several @mentions becomes ONE collaboration start:
//   @backend-team @qa-team @architect hello
//     → targets = [Group/backend-team, Group/qa-team, Member/architect]
type AppServiceHandler struct {
	HSToken   string
	Namespace string
	K8s       client.Client
	Provider  *Provider

	mu   sync.Mutex
	seen map[string]struct{}
}

func NewAppServiceHandler(hsToken, namespace string, k8s client.Client, provider *Provider) *AppServiceHandler {
	return &AppServiceHandler{
		HSToken:   hsToken,
		Namespace: namespace,
		K8s:       k8s,
		Provider:  provider,
		seen:      map[string]struct{}{},
	}
}

type matrixEvent struct {
	Type    string `json:"type"`
	RoomID  string `json:"room_id"`
	EventID string `json:"event_id"`
	Sender  string `json:"sender"`
	Content struct {
		Body     string `json:"body"`
		Mentions *struct {
			UserIDs []string `json:"user_ids"`
		} `json:"m.mentions"`
	} `json:"content"`
}

type transactionBody struct {
	Events []matrixEvent `json:"events"`
}

// HandleTransactions implements PUT /_matrix/app/v1/transactions/{txnId}.
func (h *AppServiceHandler) HandleTransactions(w http.ResponseWriter, r *http.Request) {
	logger := log.FromContext(r.Context()).WithName("matrix-appservice")
	if !h.verifyHSToken(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errcode":"M_FORBIDDEN","error":"invalid hs_token"}`))
		return
	}

	var body transactionBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logger.Error(err, "decode transaction")
		writeEmptyOK(w)
		return
	}

	for _, ev := range body.Events {
		if ev.Type != "m.room.message" {
			continue
		}
		if ev.Content.Mentions == nil || len(ev.Content.Mentions.UserIDs) == 0 {
			continue
		}
		if h.alreadySeen(ev.RoomID, ev.EventID) {
			continue
		}
		if err := h.handleMentionMessage(r.Context(), ev); err != nil {
			logger.Error(err, "handle mention message", "room", ev.RoomID, "event", ev.EventID)
		}
	}
	writeEmptyOK(w)
}

func (h *AppServiceHandler) handleMentionMessage(ctx context.Context, ev matrixEvent) error {
	collabName, err := h.resolveCollaboration(ctx, ev.RoomID)
	if err != nil {
		return err
	}

	members, err := h.listMembers(ctx)
	if err != nil {
		return err
	}
	groups, err := h.listGroups(ctx)
	if err != nil {
		return err
	}
	index := organization.NewMentionIndex(members, groups)

	senderTarget, err := index.Resolve(ev.Sender)
	if err != nil {
		return fmt.Errorf("unknown sender %s: %w", ev.Sender, err)
	}
	if senderTarget.Kind != agentorgsv1alpha1.MemberKind {
		return fmt.Errorf("sender %s is not a Member", ev.Sender)
	}

	targets, err := index.ResolveMany(ev.Content.Mentions.UserIDs)
	if err != nil {
		return err
	}

	intent, err := parseDispatchIntentFromBody(ev.Content.Body, index)
	if err != nil {
		return err
	}
	targets = dropSkippedTargets(targets, intent)
	if len(targets) == 0 {
		return fmt.Errorf("no positive @targets left after applying -@ excludes")
	}

	return h.Provider.emit(ctx, protocol.CollaborationEvent{
		Namespace:      h.Namespace,
		Collaboration:  collabName,
		Type:           protocol.EventTypeMemberRequest,
		Source:         protocol.EventSource{Member: senderTarget.Name},
		Targets:        targets,
		Payload:        map[string]interface{}{"text": ev.Content.Body},
		DispatchIntent: intent,
		CreatedAt:      time.Now().UTC(),
	})
}

func (h *AppServiceHandler) resolveCollaboration(ctx context.Context, roomID string) (string, error) {
	var list agentorgsv1alpha1.CollaborationList
	if err := h.K8s.List(ctx, &list, client.InNamespace(h.Namespace)); err != nil {
		return "", err
	}
	for _, c := range list.Items {
		if c.Spec.Channel.Provider != "" && c.Spec.Channel.Provider != providerName {
			continue
		}
		cmName := c.Spec.Channel.Config.Name
		cmKey := c.Spec.Channel.Config.Key
		if cmKey == "" {
			cmKey = "roomId"
		}
		id, err := ReadConfigMapKey(ctx, h.K8s, h.Namespace, cmName, cmKey)
		if err != nil || id == "" {
			continue
		}
		if id == roomID {
			return c.Name, nil
		}
	}
	return "", fmt.Errorf("no Collaboration bound to room %s", roomID)
}

func (h *AppServiceHandler) listMembers(ctx context.Context) ([]agentorgsv1alpha1.Member, error) {
	var list agentorgsv1alpha1.MemberList
	if err := h.K8s.List(ctx, &list, client.InNamespace(h.Namespace)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (h *AppServiceHandler) listGroups(ctx context.Context) ([]agentorgsv1alpha1.Group, error) {
	var list agentorgsv1alpha1.GroupList
	if err := h.K8s.List(ctx, &list, client.InNamespace(h.Namespace)); err != nil {
		return nil, err
	}
	return list.Items, nil
}

func (h *AppServiceHandler) alreadySeen(roomID, eventID string) bool {
	if eventID == "" {
		return false
	}
	key := roomID + "/" + eventID
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.seen[key]; ok {
		return true
	}
	h.seen[key] = struct{}{}
	return false
}

func (h *AppServiceHandler) verifyHSToken(r *http.Request) bool {
	if h.HSToken == "" {
		return false
	}
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ") == h.HSToken
	}
	return r.URL.Query().Get("access_token") == h.HSToken
}

func writeEmptyOK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("{}"))
}
