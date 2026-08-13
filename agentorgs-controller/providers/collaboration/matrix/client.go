package matrix

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client is a small Matrix Client-Server + AppService helper.
// It only implements what AgentOrgs needs for bootstrap and Deliver.
type Client struct {
	Homeserver string
	Domain     string
	ASToken    string
	HTTP       *http.Client
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
	if err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/register", "", body, &resp); err != nil {
		return "", err
	}
	if resp.AccessToken != "" {
		return resp.AccessToken, nil
	}
	// Already registered → login.
	return c.LoginPassword(ctx, localpart, password)
}

func (c *Client) LoginPassword(ctx context.Context, localpart, password string) (string, error) {
	body := map[string]interface{}{
		"type":     "m.login.password",
		"user":     localpart,
		"password": password,
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		ErrCode     string `json:"errcode"`
		Error       string `json:"error"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/login", "", body, &resp); err != nil {
		return "", err
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("matrix login failed: %s %s", resp.ErrCode, resp.Error)
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
	if err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/login", c.ASToken, body, &resp); err != nil {
		return "", err
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("appservice login failed: %s %s", resp.ErrCode, resp.Error)
	}
	return resp.AccessToken, nil
}

func (c *Client) EnsureAppServiceUser(ctx context.Context, localpart string) (userID, token string, err error) {
	userID = c.UserID(localpart)
	path := "/_matrix/client/v3/register?kind=user"
	body := map[string]interface{}{
		"username":                 localpart,
		"type":                     "m.login.application_service",
		"inhibit_login":            false,
		"auth":                     map[string]string{"type": "m.login.application_service"},
	}
	var resp struct {
		AccessToken string `json:"access_token"`
		UserID      string `json:"user_id"`
		ErrCode     string `json:"errcode"`
	}
	if err := c.doJSON(ctx, http.MethodPost, path, c.ASToken, body, &resp); err != nil {
		return "", "", err
	}
	if resp.AccessToken != "" {
		if resp.UserID != "" {
			userID = resp.UserID
		}
		return userID, resp.AccessToken, nil
	}
	token, err = c.LoginAppServiceUser(ctx, localpart)
	return userID, token, err
}

func (c *Client) CreateRoom(ctx context.Context, token, name, aliasLocalpart string, invite []string) (string, error) {
	body := map[string]interface{}{
		"name":          name,
		"preset":        "private_chat",
		"invite":        invite,
		"is_direct":     false,
		"room_alias_name": aliasLocalpart,
	}
	var resp struct {
		RoomID  string `json:"room_id"`
		ErrCode string `json:"errcode"`
		Error   string `json:"error"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/_matrix/client/v3/createRoom", token, body, &resp); err != nil {
		return "", err
	}
	if resp.RoomID != "" {
		return resp.RoomID, nil
	}
	// Alias may already exist — resolve it.
	alias := fmt.Sprintf("#%s:%s", aliasLocalpart, c.Domain)
	roomID, err := c.ResolveAlias(ctx, token, alias)
	if err != nil {
		return "", fmt.Errorf("create room failed: %s %s (%v)", resp.ErrCode, resp.Error, err)
	}
	return roomID, nil
}

func (c *Client) ResolveAlias(ctx context.Context, token, alias string) (string, error) {
	path := "/_matrix/client/v3/directory/room/" + url.PathEscape(alias)
	var resp struct {
		RoomID string `json:"room_id"`
	}
	if err := c.doJSON(ctx, http.MethodGet, path, token, nil, &resp); err != nil {
		return "", err
	}
	if resp.RoomID == "" {
		return "", fmt.Errorf("alias %s not found", alias)
	}
	return resp.RoomID, nil
}

func (c *Client) Invite(ctx context.Context, token, roomID, userID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/invite", url.PathEscape(roomID))
	body := map[string]string{"user_id": userID}
	var resp map[string]interface{}
	return c.doJSON(ctx, http.MethodPost, path, token, body, &resp)
}

func (c *Client) JoinRoom(ctx context.Context, token, roomID string) error {
	path := fmt.Sprintf("/_matrix/client/v3/join/%s", url.PathEscape(roomID))
	var resp map[string]interface{}
	return c.doJSON(ctx, http.MethodPost, path, token, map[string]interface{}{}, &resp)
}

func (c *Client) SendMessage(ctx context.Context, token, roomID, text string) error {
	return c.SendMessageWithMentions(ctx, token, roomID, text, nil)
}

// SendMessageWithMentions posts m.room.message and optionally sets m.mentions.user_ids.
func (c *Client) SendMessageWithMentions(ctx context.Context, token, roomID, text string, mentionUserIDs []string) error {
	path := fmt.Sprintf("/_matrix/client/v3/rooms/%s/send/m.room.message", url.PathEscape(roomID))
	body := map[string]interface{}{
		"msgtype": "m.text",
		"body":    text,
	}
	if len(mentionUserIDs) > 0 {
		body["m.mentions"] = map[string]interface{}{"user_ids": mentionUserIDs}
	}
	var resp map[string]interface{}
	return c.doJSON(ctx, http.MethodPost, path, token, body, &resp)
}

func (c *Client) AdminCommand(ctx context.Context, adminToken, command string) error {
	alias := fmt.Sprintf("#admins:%s", c.Domain)
	roomID, err := c.ResolveAlias(ctx, adminToken, alias)
	if err != nil {
		return fmt.Errorf("resolve admin room: %w", err)
	}
	return c.SendMessage(ctx, adminToken, roomID, command)
}

func (c *Client) doJSON(ctx context.Context, method, path, bearer string, body interface{}, out interface{}) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Homeserver+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if out != nil && len(raw) > 0 {
		_ = json.Unmarshal(raw, out)
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("matrix %s %s: %s (%s)", method, path, resp.Status, string(raw))
	}
	return nil
}
