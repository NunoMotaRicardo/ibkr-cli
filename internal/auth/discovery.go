// Package auth implements the MCP authorization flow: discovery, dynamic client
// registration, authorization code + PKCE, refresh and revocation.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Errors mapped to exit code 3 by the CLI.
var (
	ErrNotConnected = errors.New("not connected")
	ErrAuthExpired  = errors.New("authorization expired or revoked")
)

// HTTPError is a non-2xx response from an OAuth endpoint.
type HTTPError struct {
	Status int
	URL    string
	Body   string
}

func (e *HTTPError) Error() string {
	b := strings.TrimSpace(e.Body)
	if len(b) > 400 {
		b = b[:400] + "..."
	}
	return fmt.Sprintf("HTTP %d from %s: %s", e.Status, e.URL, b)
}

// Endpoints are the discovered OAuth endpoints for an MCP server.
type Endpoints struct {
	Resource        string
	Issuer          string
	Authorization   string
	Token           string
	Revocation      string
	Registration    string
	ScopesSupported []string
	IssParam        bool     // authorization_response_iss_parameter_supported
	Warnings        []string // discovery fallbacks used
}

// IBKR defaults, used only when discovery fails for the default server.
const (
	defaultServer = "https://api.ibkr.com/v1/api/mcp-public"
	defaultIssuer = "https://api.ibkr.com"
)

func defaultEndpoints() *Endpoints {
	return &Endpoints{
		Resource:        defaultServer,
		Issuer:          defaultIssuer,
		Authorization:   "https://api.ibkr.com/oauth2/authorize",
		Token:           "https://api.ibkr.com/oauth2/api/v1/token",
		Revocation:      "https://api.ibkr.com/oauth2/api/v1/token/revoke",
		Registration:    "https://api.ibkr.com/oauth2/register",
		ScopesSupported: []string{"mcp.read", "mcp.write", "mcp.orders.submit"},
		IssParam:        true,
	}
}

var resourceMetaRe = regexp.MustCompile(`resource_metadata="([^"]+)"`)

// Discover finds the OAuth endpoints for an MCP server URL following the MCP
// authorization spec (401 + WWW-Authenticate -> protected resource metadata ->
// authorization server metadata). For IBKR's default server, built-in defaults
// fill in anything discovery could not provide.
func Discover(ctx context.Context, hc *http.Client, server string) (*Endpoints, error) {
	ep := &Endpoints{Resource: server}
	var prmURLs []string

	// 1. Probe the MCP endpoint unauthenticated.
	if req, err := http.NewRequestWithContext(ctx, http.MethodPost, server, strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"ping"}`)); err == nil {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if resp, err := hc.Do(req); err == nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
			resp.Body.Close()
			if resp.StatusCode == http.StatusUnauthorized {
				if m := resourceMetaRe.FindStringSubmatch(resp.Header.Get("WWW-Authenticate")); m != nil {
					prmURLs = append(prmURLs, m[1])
				}
			}
		} else {
			ep.Warnings = append(ep.Warnings, "probe failed: "+err.Error())
		}
	}
	prmURLs = append(prmURLs, server+"/.well-known/oauth-protected-resource")
	if u, err := url.Parse(server); err == nil && u.Path != "" && u.Path != "/" {
		prmURLs = append(prmURLs, u.Scheme+"://"+u.Host+"/.well-known/oauth-protected-resource"+u.Path)
	}

	// 2. Protected resource metadata.
	var prm struct {
		Resource             string   `json:"resource"`
		AuthorizationServers []string `json:"authorization_servers"`
		ScopesSupported      []string `json:"scopes_supported"`
	}
	issuer := ""
	for _, u := range prmURLs {
		if err := getJSON(ctx, hc, u, &prm); err == nil && len(prm.AuthorizationServers) > 0 {
			issuer = strings.TrimRight(prm.AuthorizationServers[0], "/")
			if prm.Resource != "" {
				ep.Resource = prm.Resource
			}
			ep.ScopesSupported = prm.ScopesSupported
			break
		}
	}
	if issuer == "" {
		ep.Warnings = append(ep.Warnings, "protected resource metadata not found")
		if server == defaultServer {
			issuer = defaultIssuer
		} else {
			u, err := url.Parse(server)
			if err != nil {
				return nil, err
			}
			issuer = u.Scheme + "://" + u.Host
		}
	}
	ep.Issuer = issuer

	// 3. Authorization server metadata.
	var as struct {
		Issuer                string   `json:"issuer"`
		AuthorizationEndpoint string   `json:"authorization_endpoint"`
		TokenEndpoint         string   `json:"token_endpoint"`
		RevocationEndpoint    string   `json:"revocation_endpoint"`
		RegistrationEndpoint  string   `json:"registration_endpoint"`
		ScopesSupported       []string `json:"scopes_supported"`
		IssParam              bool     `json:"authorization_response_iss_parameter_supported"`
	}
	found := false
	iu, _ := url.Parse(issuer)
	cands := []string{issuer + "/.well-known/oauth-authorization-server"}
	if iu != nil && iu.Path != "" && iu.Path != "/" {
		cands = append(cands, iu.Scheme+"://"+iu.Host+"/.well-known/oauth-authorization-server"+iu.Path)
	}
	cands = append(cands, issuer+"/.well-known/openid-configuration")
	for _, u := range cands {
		if err := getJSON(ctx, hc, u, &as); err == nil && as.TokenEndpoint != "" {
			found = true
			break
		}
	}
	if found {
		if as.Issuer != "" {
			ep.Issuer = strings.TrimRight(as.Issuer, "/")
		}
		ep.Authorization, ep.Token, ep.Revocation, ep.Registration = as.AuthorizationEndpoint, as.TokenEndpoint, as.RevocationEndpoint, as.RegistrationEndpoint
		ep.IssParam = as.IssParam
		if len(ep.ScopesSupported) == 0 {
			ep.ScopesSupported = as.ScopesSupported
		}
		return ep, nil
	}
	ep.Warnings = append(ep.Warnings, "authorization server metadata not found")
	if server == defaultServer {
		d := defaultEndpoints()
		d.Warnings = ep.Warnings
		return d, nil
	}
	return nil, fmt.Errorf("could not discover OAuth endpoints for %s: %s", server, strings.Join(ep.Warnings, "; "))
}

func getJSON(ctx context.Context, hc *http.Client, u string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode/100 != 2 {
		return &HTTPError{Status: resp.StatusCode, URL: u, Body: string(b)}
	}
	return json.Unmarshal(b, v)
}

// DefaultHTTPClient returns a client with sane timeouts for OAuth calls.
func DefaultHTTPClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }
