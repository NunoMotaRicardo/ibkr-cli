package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client is a dynamically registered OAuth client.
type Client struct {
	ID          string
	Secret      string
	RedirectURI string
}

// Token is an OAuth token response.
type Token struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	Expiry       time.Time
	Scopes       []string
}

// PKCE holds a verifier/challenge pair (S256).
type PKCE struct{ Verifier, Challenge string }

func randB64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewPKCE creates a 256-bit verifier and its S256 challenge.
func NewPKCE() (PKCE, error) {
	v, err := randB64(32)
	if err != nil {
		return PKCE{}, err
	}
	return PKCE{Verifier: v, Challenge: Challenge(v)}, nil
}

// Challenge computes the S256 code challenge for a verifier.
func Challenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// NewState returns a random state value.
func NewState() (string, error) { return randB64(24) }

// Register performs RFC 7591 dynamic client registration.
func Register(ctx context.Context, hc *http.Client, ep *Endpoints, redirectURI string, scopes []string) (*Client, error) {
	return RegisterNamed(ctx, hc, ep, redirectURI, scopes, "ibkr-cli")
}

// RegisterNamed is Register with an explicit client_name, which IBKR shows on
// the consent page and under Manage Third-Party Consents.
func RegisterNamed(ctx context.Context, hc *http.Client, ep *Endpoints, redirectURI string, scopes []string, clientName string) (*Client, error) {
	if ep.Registration == "" {
		return nil, errors.New("authorization server offers no registration endpoint")
	}
	body, _ := json.Marshal(map[string]any{
		"client_name":                clientName,
		"redirect_uris":              []string{redirectURI},
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"scope":                      strings.Join(scopes, " "),
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.Registration, strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return nil, &HTTPError{Status: resp.StatusCode, URL: ep.Registration, Body: string(b)}
	}
	var r struct {
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
	}
	if err := json.Unmarshal(b, &r); err != nil || r.ClientID == "" {
		return nil, fmt.Errorf("registration response had no client_id: %s", strings.TrimSpace(string(b)))
	}
	return &Client{ID: r.ClientID, Secret: r.ClientSecret, RedirectURI: redirectURI}, nil
}

// AuthURL builds the authorization request URL (code + PKCE S256 + state + resource).
func AuthURL(ep *Endpoints, c *Client, scopes []string, state, challenge string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", c.ID)
	q.Set("redirect_uri", c.RedirectURI)
	q.Set("scope", strings.Join(scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("resource", ep.Resource)
	sep := "?"
	if strings.Contains(ep.Authorization, "?") {
		sep = "&"
	}
	return ep.Authorization + sep + q.Encode()
}

// Callback is the parsed redirect from the authorization server.
type Callback struct {
	Code, State, Iss, Error, ErrorDescription string
}

// ParseCallbackURL parses a full redirect URL, a query string, or a bare code.
func ParseCallbackURL(raw string) (Callback, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Callback{}, errors.New("empty input")
	}
	if !strings.Contains(raw, "=") {
		return Callback{Code: raw}, nil
	}
	if i := strings.Index(raw, "?"); i >= 0 {
		raw = raw[i+1:]
	}
	if i := strings.Index(raw, "#"); i >= 0 {
		raw = raw[:i]
	}
	q, err := url.ParseQuery(raw)
	if err != nil {
		return Callback{}, err
	}
	return Callback{Code: q.Get("code"), State: q.Get("state"), Iss: q.Get("iss"), Error: q.Get("error"), ErrorDescription: q.Get("error_description")}, nil
}

// ValidateCallback checks error, state and iss (RFC 9207).
func ValidateCallback(cb Callback, state string, ep *Endpoints, bareCodeOK bool) error {
	if cb.Error != "" {
		return fmt.Errorf("authorization denied: %s %s", cb.Error, cb.ErrorDescription)
	}
	if cb.Code == "" {
		return errors.New("no authorization code in callback")
	}
	if cb.State == "" && bareCodeOK {
		return nil
	}
	if cb.State != state {
		return errors.New("state mismatch: possible CSRF or stale browser tab; run connect again")
	}
	if cb.Iss != "" && strings.TrimRight(cb.Iss, "/") != strings.TrimRight(ep.Issuer, "/") {
		return fmt.Errorf("issuer mismatch: got %q, expected %q", cb.Iss, ep.Issuer)
	}
	if cb.Iss == "" && ep.IssParam {
		return errors.New("callback is missing the iss parameter the authorization server promised")
	}
	return nil
}

// Listener is the loopback redirect receiver.
type Listener struct {
	ln  net.Listener
	srv *http.Server
	ch  chan Callback
}

// Listen binds 127.0.0.1 on preferredPort (if non-zero and free) or a random port.
func Listen(preferredPort int) (*Listener, error) {
	var ln net.Listener
	var err error
	if preferredPort > 0 {
		ln, err = net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(preferredPort))
	}
	if ln == nil {
		ln, err = net.Listen("tcp", "127.0.0.1:0")
	}
	if err != nil {
		return nil, err
	}
	l := &Listener{ln: ln, ch: make(chan Callback, 1)}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		cb, _ := ParseCallbackURL(r.URL.RawQuery)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if cb.Error != "" || cb.Code == "" {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "<h3>ibkr-cli: authorization failed</h3><p>Return to the terminal for details.</p>")
		} else {
			fmt.Fprint(w, "<h3>ibkr-cli: connected</h3><p>You can close this tab and return to the terminal.</p>")
		}
		select {
		case l.ch <- cb:
		default:
		}
	})
	l.srv = &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = l.srv.Serve(ln) }()
	return l, nil
}

// Port is the bound port.
func (l *Listener) Port() int { return l.ln.Addr().(*net.TCPAddr).Port }

// RedirectURI is the loopback redirect URI for this listener.
func (l *Listener) RedirectURI() string {
	return "http://127.0.0.1:" + strconv.Itoa(l.Port()) + "/callback"
}

// Wait blocks for the callback, context cancellation or timeout.
func (l *Listener) Wait(ctx context.Context, timeout time.Duration) (Callback, error) {
	select {
	case cb := <-l.ch:
		return cb, nil
	case <-time.After(timeout):
		return Callback{}, fmt.Errorf("timed out after %s waiting for the browser login", timeout)
	case <-ctx.Done():
		return Callback{}, ctx.Err()
	}
}

// Close stops the listener.
func (l *Listener) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_ = l.srv.Shutdown(ctx)
}

type tokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Scope        string `json:"scope"`
}

func postForm(ctx context.Context, hc *http.Client, endpoint string, form url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return b, &HTTPError{Status: resp.StatusCode, URL: endpoint, Body: string(b)}
	}
	return b, nil
}

func parseToken(b []byte, now time.Time) (*Token, error) {
	var r tokenResp
	if err := json.Unmarshal(b, &r); err != nil || r.AccessToken == "" {
		return nil, fmt.Errorf("token response had no access_token: %.200s", string(b))
	}
	t := &Token{AccessToken: r.AccessToken, RefreshToken: r.RefreshToken, TokenType: r.TokenType}
	if r.ExpiresIn > 0 {
		t.Expiry = now.Add(time.Duration(r.ExpiresIn) * time.Second)
	}
	if r.Scope != "" {
		t.Scopes = strings.Fields(r.Scope)
	}
	return t, nil
}

func clientForm(c *Client, ep *Endpoints) url.Values {
	f := url.Values{}
	f.Set("client_id", c.ID)
	if c.Secret != "" {
		f.Set("client_secret", c.Secret)
	}
	if ep.Resource != "" {
		f.Set("resource", ep.Resource)
	}
	return f
}

// Exchange trades an authorization code for tokens.
func Exchange(ctx context.Context, hc *http.Client, ep *Endpoints, c *Client, code, verifier string) (*Token, error) {
	f := clientForm(c, ep)
	f.Set("grant_type", "authorization_code")
	f.Set("code", code)
	f.Set("redirect_uri", c.RedirectURI)
	f.Set("code_verifier", verifier)
	b, err := postForm(ctx, hc, ep.Token, f)
	if err != nil {
		return nil, err
	}
	return parseToken(b, time.Now())
}

// RefreshToken exchanges a refresh token. invalid_grant / 400 / 401 map to ErrAuthExpired.
func RefreshToken(ctx context.Context, hc *http.Client, ep *Endpoints, c *Client, refresh string) (*Token, error) {
	f := clientForm(c, ep)
	f.Set("grant_type", "refresh_token")
	f.Set("refresh_token", refresh)
	b, err := postForm(ctx, hc, ep.Token, f)
	if err != nil {
		var he *HTTPError
		if errors.As(err, &he) && (he.Status == 400 || he.Status == 401 || he.Status == 403) {
			return nil, fmt.Errorf("%w: %s", ErrAuthExpired, strings.TrimSpace(he.Body))
		}
		return nil, err
	}
	t, err := parseToken(b, time.Now())
	if err != nil {
		return nil, err
	}
	if t.RefreshToken == "" {
		t.RefreshToken = refresh
	}
	return t, nil
}

// Revoke revokes a token at the revocation endpoint (RFC 7009).
func Revoke(ctx context.Context, hc *http.Client, ep *Endpoints, c *Client, token, hint string) error {
	if ep.Revocation == "" {
		return errors.New("no revocation endpoint")
	}
	f := url.Values{}
	f.Set("token", token)
	f.Set("token_type_hint", hint)
	f.Set("client_id", c.ID)
	if c.Secret != "" {
		f.Set("client_secret", c.Secret)
	}
	_, err := postForm(ctx, hc, ep.Revocation, f)
	return err
}
