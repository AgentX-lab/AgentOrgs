package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

// Client is a small Matrix Client-Server + AppService helper.
// It only implements what AgentOrgs needs for bootstrap and Deliver.
type Client struct {
	Homeserver string
	Domain     string
	ASToken    string
	HTTP       *http.Client
	txnSeq     atomic.Int64 // per-client Matrix send transaction IDs
}

func NewClient(homeserver, domain, asToken string) *Client {
	return &Client{
		Homeserver: strings.TrimRight(homeserver, "/"),
		Domain:     domain,
		ASToken:    asToken,
		HTTP:       &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) UserID(localpart string) string {
	return fmt.Sprintf("@%s:%s", localpart, c.Domain)
}

func (c *Client) RegisterPassword(ctx context.Context, localpart, password string) (string, error) {
	body := map[string]interface{}{
		"username": localpart,
		"password": password,
		"auth":     map[string]string{"type": "m.login.dummy"},
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		ErrCode     string `json:"errcode"`
		Error       string `json:"error"`
	}
	status, _, err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/register", "", body, &resp)
	if err != nil {
		return "", err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		if resp.AccessToken != "" {
			return resp.AccessToken, nil
		}
	}
	// Already registered → login (same as AgentTeams EnsureUser).
	if resp.ErrCode != "" && resp.ErrCode != "M_USER_IN_USE" && status >= 300 {
		return "", fmt.Errorf("matrix register %s: %s %s", localpart, resp.ErrCode, resp.Error)
	}
	return c.LoginPassword(ctx, localpart, password)
}

func (c *Client) LoginPassword(ctx context.Context, localpart, password string) (string, error) {
	body := map[string]interface{}{
		"type": "m.login.password",
		"identifier": map[string]string{
			"type": "m.id.user",
			"user": localpart,
		},
		"password": password,
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		ErrCode     string `json:"errcode"`
		Error       string `json:"error"`
	}
	status, _, err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/login", "", body, &resp)
	if err != nil {
		return "", err
	}
	if status >= 300 || resp.AccessToken == "" {
		return "", fmt.Errorf("matrix login failed: HTTP %d %s %s", status, resp.ErrCode, resp.Error)
	}
	return resp.AccessToken, nil
}

func (c *Client) LoginAppServiceUser(ctx context.Context, localpart string) (string, error) {
	if c.ASToken == "" {
		return "", fmt.Errorf("appservice as_token is empty")
	}
	body := map[string]interface{}{
		"type": "m.login.application_service",
		"identifier": map[string]string{
			"type": "m.id.user",
			"user": localpart,
		},
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		ErrCode     string `json:"errcode"`
		Error       string `json:"error"`
	}
	status, _, err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/login", c.ASToken, body, &resp)
	if err != nil {
		return "", err
	}
	if status >= 300 || resp.AccessToken == "" {
		return "", fmt.Errorf("appservice login failed: HTTP %d %s %s", status, resp.ErrCode, resp.Error)
	}
	return resp.AccessToken, nil
}

func (c *Client) EnsureAppServiceUser(ctx context.Context, localpart string) (userID, token string, err error) {
	userID = c.UserID(localpart)
	// Match AgentTeams: AS register body without ?kind=user; as_token authenticates.
	body := map[string]interface{}{
		"type":     "m.login.application_service",
		"username": localpart,
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		UserID      string `json:"user_id"`
		ErrCode     string `json:"errcode"`
		Error       string `json:"error"`
	}
	status, _, err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/register", c.ASToken, body, &resp)
	if err != nil {
		return "", "", err
	}
	if status == http.StatusOK || status == http.StatusCreated {
		if resp.UserID != "" {
			userID = resp.UserID
		}
		if resp.AccessToken != "" {
			return userID, resp.AccessToken, nil
		}
	}
	if resp.ErrCode == "M_USER_IN_USE" || status == http.StatusOK || status == http.StatusCreated {
		token, err = c.LoginAppServiceUser(ctx, localpart)
		return userID, token, err
	}
	return "", "", fmt.Errorf("AS register %s: HTTP %d %s %s", localpart, status, resp.ErrCode, resp.Error)
}

func (c *Client) CreateRoom(ctx context.Context, token, name, aliasLocalpart string, invite []string) (string, error) {
	body := map[string]interface{}{
		"name":            name,
		"preset":          "private_chat",
		"invite":          invite,
		"is_direct":       false,
		"room_alias_name": aliasLocalpart,
	}
	var resp struct {
		RoomID  string `json:"room_id"`
		ErrCode string `json:"errcode"`
		Error   string `json:"error"`
	}
	status, _, err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/createRoom", token, body, &resp)
	if err != nil {
		return "", err
	}
	if (status == http.StatusOK || status == http.StatusCreated) && resp.RoomID != "" {
		return resp.RoomID, nil
	}
	// Alias may already exist — resolve it.
	alias := fmt.Sprintf("#%s:%s", aliasLocalpart, c.Domain)
	roomID, resolveErr := c.ResolveAlias(ctx, token, alias)
	if resolveErr != nil {
		return "", fmt.Errorf("create room failed: HTTP %d %s %s (%v)", status, resp.ErrCode, resp.Error, resolveErr)
	}
	return roomID, nil
}

func (c *Client) ResolveAlias(ctx context.Context, token, alias string) (string, error) {
	path := "/_matrix/client/v3/directory/room/" + encodeAlias(alias)
	var resp struct {
		RoomID string `json:"room_id"`
	}
	status, _, err := c.doJSON(ctx, http.MethodGet, path, token, nil, &resp)
	if err != nil {
		return "", err
	}
	if status >= 300 || resp.RoomID == "" {
		return "", fmt.Errorf("alias %s not found", alias)
	}
	return resp.RoomID, nil
}

func (c *Client) Invite(ctx context.Context, token, roomID, userID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/invite", encodeRoomID(roomID))
	body := map[string]string{"user_id": userID}
	status, raw, err := c.doJSON(ctx, http.MethodPost, path, token, body, nil)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("matrix invite: HTTP %d (%s)", status, string(raw))
	}
	return nil
}

func (c *Client) JoinRoom(ctx context.Context, token, roomID string) error {
	// Align with AgentTeams: POST /rooms/{roomId}/join
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/join", encodeRoomID(roomID))
	status, raw, err := c.doJSON(ctx, http.MethodPost, path, token, map[string]interface{}{}, nil)
	if err != nil {
		return err
	}
	if status >= 300 {
		return fmt.Errorf("matrix join: HTTP %d (%s)", status, string(raw))
	}
	return nil
}

func (c *Client) SendMessage(ctx context.Context, token, roomID, text string) error {
	return c.SendMessageWithMentions(ctx, token, roomID, text, nil)
}

// SendMessageWithMentions posts m.room.message and optionally sets m.mentions.user_ids.
// Matrix CS API requires PUT .../send/{eventType}/{txnId}.
func (c *Client) SendMessageWithMentions(ctx context.Context, token, roomID, text string, mentionUserIDs []string) error {
	txnID := fmt.Sprintf("ao-%d", c.txnSeq.Add(1))
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message/%s", encodeRoomID(roomID), txnID)
	body := map[string]interface{}{
		"msgtype": "m.text",
		"body":    text,
	}
	if len(mentionUserIDs) > 0 {
		body["m.mentions"] = map[string]interface{}{"user_ids": mentionUserIDs}
	}
	status, raw, err := c.doJSON(ctx, http.MethodPut, path, token, body, nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return fmt.Errorf("matrix send message: HTTP %d (%s)", status, string(raw))
	}
	return nil
}

func (c *Client) AdminCommand(ctx context.Context, adminToken, command string) error {
	alias := fmt.Sprintf("#admins:%s", c.Domain)
	roomID, err := c.ResolveAlias(ctx, adminToken, alias)
	if err != nil {
		return fmt.Errorf("resolve admin room: %w", err)
	}
	return c.SendMessage(ctx, adminToken, roomID, command)
}

// doJSON performs a Matrix HTTP call. Transport failures return err.
// HTTP 4xx/5xx still unmarshal into out and return (status, body, nil) so
// callers can handle Matrix errcodes (e.g. M_USER_IN_USE) like AgentTeams.
func (c *Client) doJSON(ctx context.Context, method, path, bearer string, body interface{}, out interface{}) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Homeserver+path, reader)
	if err != nil {
		return 0, nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	if out != nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, out)
	}
	return resp.StatusCode, raw, nil
}

// encodeRoomID percent-encodes "!" in room IDs for URL paths (AgentTeams).
func encodeRoomID(roomID string) string {
	return strings.ReplaceAll(roomID, "!", "%21")
}

// encodeAlias percent-encodes "#" and ":" in room aliases for URL paths (AgentTeams).
func encodeAlias(alias string) string {
	s := strings.ReplaceAll(alias, "#", "%23")
	return strings.ReplaceAll(s, ":", "%3A")
}
