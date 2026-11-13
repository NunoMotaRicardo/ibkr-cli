// Package testfake provides an in-process fake of IBKR's MCP server and its
// OAuth authorization server, for tests. No network is used.
package testfake

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Handler produces the text content of a tools/call result.
type Handler func(args map[string]any) (text string, isError bool)

// Fake is one server hosting both the MCP endpoint (/mcp) and the AS.
type Fake struct {
	*httptest.Server
	mu sync.Mutex

	Tools    []map[string]any // tools/list entries
	PageSize int              // paginate tools/list when > 0
	Handlers map[string]Handler
	SSE      bool // answer tools/call with text/event-stream
	// Hang makes tools/call for these names never answer (simulates a timeout).
	Hang map[string]bool

	ValidTokens  map[string]bool
	RefreshCount int
	RefreshFails bool
	Registered   []map[string]any
	RejectDCR    bool
	Revoked      []string
	Calls        []Call // every tools/call received
	Initializes  int
	Unauthorized int
	ExpiresIn    int
	LastAuthURL  string
	SessionID    string
}

// Call is a recorded tools/call.
type Call struct {
	Name string
	Args map[string]any
}

// New starts a fake server.
func New() *Fake {
	f := &Fake{Handlers: map[string]Handler{}, Hang: map[string]bool{}, ValidTokens: map[string]bool{}, ExpiresIn: 3600, SessionID: "sess-1"}
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", f.mcp)
	mux.HandleFunc("/mcp/.well-known/oauth-protected-resource", f.prm)
	mux.HandleFunc("/.well-known/oauth-authorization-server", f.asMeta)
	mux.HandleFunc("/register", f.register)
	mux.HandleFunc("/authorize", f.authorize)
	mux.HandleFunc("/token", f.token)
	mux.HandleFunc("/revoke", f.revoke)
	f.Server = httptest.NewServer(mux)
	return f
}

// MCPURL is the MCP endpoint URL.
func (f *Fake) MCPURL() string { return f.URL + "/mcp" }

// Tool builds a tools/list entry.
func Tool(name, desc string, schema map[string]any, readOnly *bool) map[string]any {
	if schema == nil {
		schema = map[string]any{"type": "object", "properties": map[string]any{}}
	}
	t := map[string]any{"name": name, "description": desc, "inputSchema": schema}
	if readOnly != nil {
		t["annotations"] = map[string]any{"readOnlyHint": *readOnly}
	}
	return t
}

// CallsTo returns recorded calls for a tool name.
func (f *Fake) CallsTo(name string) []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []Call
	for _, c := range f.Calls {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}

// AllowToken marks a token as valid.
func (f *Fake) AllowToken(t string) {
	f.mu.Lock()
	f.ValidTokens[t] = true
	f.mu.Unlock()
}

func (f *Fake) prm(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{"resource": f.MCPURL(), "authorization_servers": []string{f.URL}, "scopes_supported": []string{"mcp.read", "mcp.write", "mcp.orders.submit"}})
}

func (f *Fake) asMeta(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]any{
		"issuer": f.URL, "authorization_endpoint": f.URL + "/authorize", "token_endpoint": f.URL + "/token",
		"revocation_endpoint": f.URL + "/revoke", "registration_endpoint": f.URL + "/register",
		"code_challenge_methods_supported": []string{"S256"}, "authorization_response_iss_parameter_supported": true,
		"scopes_supported": []string{"mcp.read", "mcp.write", "mcp.orders.submit"},
	})
}

func (f *Fake) register(w http.ResponseWriter, r *http.Request) {
	var body map[string]any
	_ = json.NewDecoder(r.Body).Decode(&body)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.RejectDCR {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":"invalid_redirect_uri","error_description":"loopback not allowed"}`)
		return
	}
	f.Registered = append(f.Registered, body)
	w.WriteHeader(201)
	json.NewEncoder(w).Encode(map[string]any{"client_id": fmt.Sprintf("client-%d", len(f.Registered))})
}

// authorize simulates login+consent by redirecting straight to the redirect_uri.
func (f *Fake) authorize(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.LastAuthURL = r.URL.String()
	f.mu.Unlock()
	q := r.URL.Query()
	u, _ := url.Parse(q.Get("redirect_uri"))
	v := u.Query()
	v.Set("code", "code-123")
	v.Set("state", q.Get("state"))
	v.Set("iss", f.URL)
	u.RawQuery = v.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

func (f *Fake) token(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Form.Get("grant_type") {
	case "authorization_code":
		if r.Form.Get("code") != "code-123" || r.Form.Get("code_verifier") == "" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":"invalid_grant"}`)
			return
		}
		tok := "access-1"
		f.ValidTokens[tok] = true
		json.NewEncoder(w).Encode(map[string]any{"access_token": tok, "refresh_token": "refresh-1", "token_type": "Bearer", "expires_in": f.ExpiresIn, "scope": "mcp.read mcp.write"})
	case "refresh_token":
		if f.RefreshFails || r.Form.Get("refresh_token") == "" {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"error":"invalid_grant","error_description":"revoked"}`)
			return
		}
		f.RefreshCount++
		tok := fmt.Sprintf("access-r%d", f.RefreshCount)
		f.ValidTokens[tok] = true
		json.NewEncoder(w).Encode(map[string]any{"access_token": tok, "token_type": "Bearer", "expires_in": f.ExpiresIn})
	default:
		w.WriteHeader(400)
		fmt.Fprint(w, `{"error":"unsupported_grant_type"}`)
	}
}

func (f *Fake) revoke(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	f.Revoked = append(f.Revoked, r.Form.Get("token_type_hint")+":"+r.Form.Get("token"))
	delete(f.ValidTokens, r.Form.Get("token"))
	f.mu.Unlock()
	w.WriteHeader(200)
}

type rpcReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

func (f *Fake) mcp(w http.ResponseWriter, r *http.Request) {
	auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	f.mu.Lock()
	ok := auth != "" && f.ValidTokens[auth]
	if !ok {
		f.Unauthorized++
	}
	f.mu.Unlock()
	if !ok {
		w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata="%s/mcp/.well-known/oauth-protected-resource"`, f.URL))
		w.WriteHeader(401)
		return
	}
	b, _ := io.ReadAll(r.Body)
	var req rpcReq
	if err := json.Unmarshal(b, &req); err != nil {
		w.WriteHeader(400)
		return
	}
	if len(req.ID) == 0 { // notification
		w.WriteHeader(202)
		return
	}
	write := func(result any, rpcErr map[string]any) {
		resp := map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(req.ID)}
		if rpcErr != nil {
			resp["error"] = rpcErr
		} else {
			resp["result"] = result
		}
		out, _ := json.Marshal(resp)
		if f.SSE && req.Method == "tools/call" {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\",\"params\":{}}\n\nevent: message\ndata: %s\n\n", out)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(out)
	}
	switch req.Method {
	case "initialize":
		f.mu.Lock()
		f.Initializes++
		f.mu.Unlock()
		w.Header().Set("Mcp-Session-Id", f.SessionID)
		write(map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "fake-ibkr", "version": "1.2.1"}}, nil)
	case "tools/list":
		var p struct {
			Cursor string `json:"cursor"`
		}
		_ = json.Unmarshal(req.Params, &p)
		tools := f.Tools
		next := ""
		if f.PageSize > 0 {
			start := 0
			if p.Cursor != "" {
				fmt.Sscanf(p.Cursor, "c%d", &start)
			}
			end := start + f.PageSize
			if end < len(tools) {
				next = fmt.Sprintf("c%d", end)
			} else {
				end = len(tools)
			}
			tools = tools[start:end]
		}
		res := map[string]any{"tools": tools}
		if next != "" {
			res["nextCursor"] = next
		}
		write(res, nil)
	case "tools/call":
		var p struct {
			Name string         `json:"name"`
			Args map[string]any `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &p)
		f.mu.Lock()
		f.Calls = append(f.Calls, Call{p.Name, p.Args})
		hang := f.Hang[p.Name]
		h := f.Handlers[p.Name]
		f.mu.Unlock()
		if hang {
			select {
			case <-r.Context().Done():
			case <-time.After(10 * time.Second):
			}
			return
		}
		if h == nil {
			write(nil, map[string]any{"code": -32602, "message": "unknown tool " + p.Name})
			return
		}
		text, isErr := h(p.Args)
		write(map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isErr}, nil)
	default:
		write(nil, map[string]any{"code": -32601, "message": "method not found"})
	}
}
