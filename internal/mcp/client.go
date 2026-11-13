// Package mcp is a small MCP client over streamable HTTP: initialize,
// tools/list (paginated) and tools/call, with bearer-token refresh on 401.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/auth"
)

// ProtocolVersion is the MCP protocol version this client requests.
const ProtocolVersion = "2025-06-18"

// TokenProvider supplies bearer tokens.
type TokenProvider interface {
	AccessToken(ctx context.Context) (string, error)
	Refresh(ctx context.Context) (string, error)
}

// Client talks to one MCP server.
type Client struct {
	URL    string
	HC     *http.Client
	Tokens TokenProvider

	UserAgent string

	sessionID string
	proto     string
	nextID    atomic.Int64
	inited    bool
}

// ServerInfo is returned by Initialize.
type ServerInfo struct {
	Name            string
	Version         string
	ProtocolVersion string
}

// RPCError is a JSON-RPC error object.
type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message) }

// HTTPStatusError is a non-2xx MCP HTTP response.
type HTTPStatusError struct {
	Status int
	Body   string
}

func (e *HTTPStatusError) Error() string {
	b := strings.TrimSpace(e.Body)
	if len(b) > 300 {
		b = b[:300] + "..."
	}
	return fmt.Sprintf("MCP server returned HTTP %d: %s", e.Status, b)
}

// TransportError means the request may have reached the server but no usable
// response arrived (timeout, connection reset). The outcome is unknown.
type TransportError struct{ Err error }

func (e *TransportError) Error() string { return "no response from MCP server: " + e.Err.Error() }
func (e *TransportError) Unwrap() error { return e.Err }

// Tool is an MCP tool definition.
type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"inputSchema,omitempty"`
	Annotations *Annotations   `json:"annotations,omitempty"`
}

// Annotations are the optional MCP tool hints.
type Annotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
}

// Content is one content block of a tool result.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallResult is a tools/call result.
type CallResult struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

// Text joins all text blocks.
func (r *CallResult) Text() string {
	var parts []string
	for _, c := range r.Content {
		if c.Text != "" {
			parts = append(parts, c.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// Data returns structured content, else the text parsed as JSON, else the raw text.
func (r *CallResult) Data() any {
	if r.StructuredContent != nil {
		return r.StructuredContent
	}
	t := strings.TrimSpace(r.Text())
	if t != "" && (t[0] == '{' || t[0] == '[') {
		var v any
		d := json.NewDecoder(strings.NewReader(t))
		d.UseNumber()
		if d.Decode(&v) == nil {
			return v
		}
	}
	return t
}

type rpcResponse struct {
	ID     json.RawMessage `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *RPCError       `json:"error"`
}

// Initialize performs the MCP handshake.
func (c *Client) Initialize(ctx context.Context) (*ServerInfo, error) {
	c.sessionID, c.proto, c.inited = "", "", false
	params := map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "ibkr-cli", "version": "1"},
	}
	raw, err := c.rpc(ctx, "initialize", params, true)
	if err != nil {
		return nil, err
	}
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
		ServerInfo      struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	_ = json.Unmarshal(raw, &res)
	c.proto = res.ProtocolVersion
	if c.proto == "" {
		c.proto = ProtocolVersion
	}
	c.inited = true
	if err := c.notify(ctx, "notifications/initialized"); err != nil {
		return nil, err
	}
	return &ServerInfo{Name: res.ServerInfo.Name, Version: res.ServerInfo.Version, ProtocolVersion: c.proto}, nil
}

func (c *Client) ensureInit(ctx context.Context) error {
	if c.inited {
		return nil
	}
	_, err := c.Initialize(ctx)
	return err
}

// ListTools returns every tool, following pagination.
func (c *Client) ListTools(ctx context.Context) ([]Tool, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	var all []Tool
	cursor := ""
	for page := 0; page < 100; page++ {
		params := map[string]any{}
		if cursor != "" {
			params["cursor"] = cursor
		}
		raw, err := c.rpc(ctx, "tools/list", params, false)
		if err != nil {
			return nil, err
		}
		var res struct {
			Tools      []Tool `json:"tools"`
			NextCursor string `json:"nextCursor"`
		}
		if err := json.Unmarshal(raw, &res); err != nil {
			return nil, fmt.Errorf("bad tools/list result: %w", err)
		}
		all = append(all, res.Tools...)
		if res.NextCursor == "" {
			return all, nil
		}
		cursor = res.NextCursor
	}
	return nil, errors.New("tools/list pagination did not terminate")
}

// CallTool invokes a tool. A *TransportError means the outcome is unknown.
func (c *Client) CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error) {
	if err := c.ensureInit(ctx); err != nil {
		return nil, err
	}
	if args == nil {
		args = map[string]any{}
	}
	raw, err := c.rpc(ctx, "tools/call", map[string]any{"name": name, "arguments": args}, false)
	if err != nil {
		return nil, err
	}
	var res CallResult
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	if err := d.Decode(&res); err != nil {
		return nil, fmt.Errorf("bad tools/call result: %w", err)
	}
	return &res, nil
}

func (c *Client) notify(ctx context.Context, method string) error {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method})
	resp, err := c.post(ctx, body, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
	if resp.StatusCode/100 != 2 {
		return &HTTPStatusError{Status: resp.StatusCode}
	}
	return nil
}

func (c *Client) rpc(ctx context.Context, method string, params any, isInit bool) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	if err != nil {
		return nil, err
	}
	resp, err := c.post(ctx, body, isInit)
	if err != nil {
		return nil, err
	}
	// Session expired: re-initialize once and retry.
	if resp.StatusCode == http.StatusNotFound && c.sessionID != "" && !isInit {
		resp.Body.Close()
		if _, err := c.Initialize(ctx); err != nil {
			return nil, err
		}
		if resp, err = c.post(ctx, body, false); err != nil {
			return nil, err
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: server rejected the token", auth.ErrAuthExpired)
		}
		return nil, &HTTPStatusError{Status: resp.StatusCode, Body: string(b)}
	}
	if sid := resp.Header.Get("Mcp-Session-Id"); sid != "" {
		c.sessionID = sid
	}
	rr, err := readResponse(resp, id)
	if err != nil {
		var te *TransportError
		if errors.As(err, &te) {
			return nil, err
		}
		return nil, err
	}
	if rr.Error != nil {
		return nil, rr.Error
	}
	return rr.Result, nil
}

// post sends the body, handling bearer auth and a single refresh-and-retry on 401.
func (c *Client) post(ctx context.Context, body []byte, isInit bool) (*http.Response, error) {
	do := func(tok string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		req.Header.Set("Authorization", "Bearer "+tok)
		if c.UserAgent != "" {
			req.Header.Set("User-Agent", c.UserAgent)
		}
		if c.sessionID != "" && !isInit {
			req.Header.Set("Mcp-Session-Id", c.sessionID)
		}
		if c.proto != "" {
			req.Header.Set("MCP-Protocol-Version", c.proto)
		}
		resp, err := c.HC.Do(req)
		if err != nil {
			if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
				return nil, ctx.Err()
			}
			return nil, &TransportError{Err: err}
		}
		return resp, nil
	}
	tok, err := c.Tokens.AccessToken(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := do(tok)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}
	resp.Body.Close()
	tok, err = c.Tokens.Refresh(ctx)
	if err != nil {
		return nil, err
	}
	return do(tok)
}

func readResponse(resp *http.Response, id int64) (*rpcResponse, error) {
	ct := resp.Header.Get("Content-Type")
	if strings.HasPrefix(ct, "text/event-stream") {
		sc := bufio.NewScanner(resp.Body)
		sc.Buffer(make([]byte, 1<<16), 32<<20)
		var data []string
		flush := func() *rpcResponse {
			if len(data) == 0 {
				return nil
			}
			payload := strings.Join(data, "\n")
			data = nil
			var rr rpcResponse
			if json.Unmarshal([]byte(payload), &rr) != nil || len(rr.ID) == 0 {
				return nil // notification or junk
			}
			if rr.Result == nil && rr.Error == nil {
				return nil
			}
			if string(bytes.TrimSpace(rr.ID)) != fmt.Sprint(id) {
				return nil
			}
			return &rr
		}
		for sc.Scan() {
			line := sc.Text()
			switch {
			case line == "":
				if rr := flush(); rr != nil {
					return rr, nil
				}
			case strings.HasPrefix(line, "data:"):
				data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
			}
		}
		if rr := flush(); rr != nil {
			return rr, nil
		}
		if err := sc.Err(); err != nil {
			return nil, &TransportError{Err: err}
		}
		return nil, &TransportError{Err: errors.New("event stream ended without a response")}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, &TransportError{Err: err}
	}
	var rr rpcResponse
	if err := json.Unmarshal(b, &rr); err != nil {
		return nil, fmt.Errorf("unparseable MCP response: %.200s", string(b))
	}
	return &rr, nil
}
