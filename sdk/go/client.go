// 这个文件实现 Go SDK 客户端，负责对外封装管理接口调用、WS 建连、命令发送和事件读取。

package gosdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopbx/internal/compat"
	"gopbx/pkg/wsproto"

	"github.com/gorilla/websocket"
)

type Client struct {
	Options    ClientOptions
	HTTPClient *http.Client
	Dialer     *websocket.Dialer
}

func NewClient(opts ClientOptions) *Client {
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	dialer := opts.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	return &Client{
		Options:    normalizeClientOptions(opts),
		HTTPClient: client,
		Dialer:     dialer,
	}
}

func (c *Client) DialCall(ctx context.Context, opts DialOptions) (*Session, error) {
	return c.dialWS(ctx, compat.RouteCall, "websocket", opts)
}

func (c *Client) DialWebRTC(ctx context.Context, opts DialOptions) (*Session, error) {
	return c.dialWS(ctx, compat.RouteCallWebRTC, "webrtc", opts)
}

func (c *Client) ListCalls(ctx context.Context) (*ListCallsResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.httpURL(compat.RouteCallLists), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readHTTPError(resp)
	}
	var payload ListCallsResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (c *Client) KillCall(ctx context.Context, sessionID string) (bool, error) {
	path := strings.Replace(compat.RouteCallKill, ":id", url.PathEscape(sessionID), 1)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.httpURL(path), nil)
	if err != nil {
		return false, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, readHTTPError(resp)
	}
	var ok bool
	if err := json.NewDecoder(resp.Body).Decode(&ok); err != nil {
		return false, err
	}
	return ok, nil
}

func (c *Client) GetICEServers(ctx context.Context) ([]ICEServer, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.httpURL(compat.RouteICEServers), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, readHTTPError(resp)
	}
	var payload []ICEServer
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	return payload, nil
}

// SendCommand 用于发送任意协议命令。
// 更高层的 Invite/TTS/Hangup 等方法只是对这个底层入口做轻量封装。
func (s *Session) SendCommand(cmd Command) error {
	return s.conn.WriteJSON(cmd)
}

func (s *Session) Invite(option *CallOption) error {
	return s.sendCallSetup(wsproto.CommandInvite, option)
}

func (s *Session) Accept(option *CallOption) error {
	return s.sendCallSetup(wsproto.CommandAccept, option)
}

func (s *Session) Candidate(candidates ...string) error {
	return s.SendCommand(Command{Command: wsproto.CommandCandidate, Candidates: candidates})
}

func (s *Session) TTS(text string, option *SynthesisOption, extras ...func(*Command)) error {
	cmd := Command{Command: wsproto.CommandTTS, Text: text}
	if option != nil {
		encoded, err := json.Marshal(option)
		if err != nil {
			return err
		}
		cmd.Option = encoded
	}
	for _, extra := range extras {
		extra(&cmd)
	}
	return s.SendCommand(cmd)
}

func (s *Session) Play(mediaURL string, autoHangup bool, playID string) error {
	cmd := Command{Command: wsproto.CommandPlay, URL: &mediaURL}
	if playID != "" {
		cmd.PlayID = &playID
	}
	if autoHangup {
		cmd.AutoHangup = wsproto.Bool(true)
	}
	return s.SendCommand(cmd)
}

func (s *Session) Interrupt() error {
	return s.SendCommand(Command{Command: wsproto.CommandInterrupt})
}

func (s *Session) History(speaker, text string) error {
	return s.SendCommand(Command{Command: wsproto.CommandHistory, Speaker: &speaker, Text: text})
}

func (s *Session) Hangup(reason, initiator string) error {
	cmd := Command{Command: wsproto.CommandHangup}
	if reason != "" {
		cmd.Reason = &reason
	}
	if initiator != "" {
		cmd.Initiator = &initiator
	}
	return s.SendCommand(cmd)
}

func (s *Session) SendAudio(payload []byte) error {
	return s.conn.WriteMessage(websocket.BinaryMessage, payload)
}

func (s *Session) ReadEvent(ctx context.Context, opts ...ReadEventOption) (Event, error) {
	option := ReadEventOption{}
	if len(opts) > 0 {
		option = opts[0]
	}
	if deadline, ok := deadlineFromContext(ctx, option.Timeout); ok {
		if err := s.conn.SetReadDeadline(deadline); err != nil {
			return Event{}, err
		}
		defer func() { _ = s.conn.SetReadDeadline(time.Time{}) }()
	}
	messageType, data, err := s.conn.ReadMessage()
	if err != nil {
		return Event{}, err
	}
	if messageType != websocket.TextMessage {
		return Event{}, fmt.Errorf("unexpected websocket message type: %d", messageType)
	}
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		return Event{}, err
	}
	return event, nil
}

func (s *Session) ReadBinary(ctx context.Context, opts ...ReadBinaryOption) ([]byte, error) {
	option := ReadBinaryOption{}
	if len(opts) > 0 {
		option = opts[0]
	}
	if deadline, ok := deadlineFromContext(ctx, option.Timeout); ok {
		if err := s.conn.SetReadDeadline(deadline); err != nil {
			return nil, err
		}
		defer func() { _ = s.conn.SetReadDeadline(time.Time{}) }()
	}
	messageType, data, err := s.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if messageType != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected websocket message type: %d", messageType)
	}
	return data, nil
}

func (s *Session) ReadLoop(ctx context.Context, handler ReadEventLoopHandler) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		event, err := s.ReadEvent(ctx)
		if err != nil {
			return err
		}
		if err := handler(ctx, event); err != nil {
			return err
		}
	}
}

func (s *Session) Close() error {
	return s.conn.Close()
}

func (s *Session) CallType() string { return s.callType }

func (c *Client) dialWS(ctx context.Context, path, callType string, opts DialOptions) (*Session, error) {
	parsed, err := url.Parse(c.wsURL(path))
	if err != nil {
		return nil, err
	}
	query := parsed.Query()
	if opts.SessionID != "" {
		query.Set("id", opts.SessionID)
	}
	if opts.Dump != nil {
		query.Set("dump", fmt.Sprintf("%t", *opts.Dump))
	}
	parsed.RawQuery = query.Encode()

	ctxDialer := *c.Dialer
	conn, _, err := ctxDialer.DialContext(ctx, parsed.String(), opts.Header)
	if err != nil {
		return nil, err
	}
	return &Session{callType: callType, conn: conn}, nil
}

func (s *Session) sendCallSetup(command wsproto.CommandName, option *CallOption) error {
	cmd := Command{Command: command}
	if option != nil {
		encoded, err := json.Marshal(option)
		if err != nil {
			return err
		}
		cmd.Option = encoded
	}
	return s.SendCommand(cmd)
}

func normalizeClientOptions(opts ClientOptions) ClientOptions {
	if opts.HTTPBaseURL != "" {
		opts.HTTPBaseURL = strings.TrimRight(opts.HTTPBaseURL, "/")
	}
	if opts.WSBaseURL != "" {
		opts.WSBaseURL = strings.TrimRight(opts.WSBaseURL, "/")
	}
	if opts.WSBaseURL == "" && opts.HTTPBaseURL != "" {
		opts.WSBaseURL = httpToWS(opts.HTTPBaseURL)
	}
	if opts.HTTPBaseURL == "" && opts.WSBaseURL != "" {
		opts.HTTPBaseURL = wsToHTTP(opts.WSBaseURL)
	}
	return opts
}

func httpToWS(raw string) string {
	switch {
	case strings.HasPrefix(raw, "https://"):
		return "wss://" + strings.TrimPrefix(raw, "https://")
	case strings.HasPrefix(raw, "http://"):
		return "ws://" + strings.TrimPrefix(raw, "http://")
	default:
		return raw
	}
}

func wsToHTTP(raw string) string {
	switch {
	case strings.HasPrefix(raw, "wss://"):
		return "https://" + strings.TrimPrefix(raw, "wss://")
	case strings.HasPrefix(raw, "ws://"):
		return "http://" + strings.TrimPrefix(raw, "ws://")
	default:
		return raw
	}
}

func (c *Client) httpURL(path string) string {
	return c.Options.HTTPBaseURL + path
}

func (c *Client) wsURL(path string) string {
	return c.Options.WSBaseURL + path
}

func readHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func deadlineFromContext(ctx context.Context, timeout time.Duration) (time.Time, bool) {
	if timeout > 0 {
		return time.Now().Add(timeout), true
	}
	if deadline, ok := ctx.Deadline(); ok {
		return deadline, true
	}
	return time.Time{}, false
}
