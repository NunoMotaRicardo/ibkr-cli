package auth

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/NunoMotaRicardo/ibkr-cli/internal/secrets"
)

// Manager hands out valid access tokens, refreshing proactively.
type Manager struct {
	Store *secrets.Store
	HC    *http.Client
	Now   func() time.Time
	// Leeway is how long before expiry a token is refreshed (default 60s).
	Leeway time.Duration
	mu     sync.Mutex
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m *Manager) hc() *http.Client {
	if m.HC != nil {
		return m.HC
	}
	return DefaultHTTPClient()
}

// AccessToken returns a usable access token or ErrNotConnected / ErrAuthExpired.
func (m *Manager) AccessToken(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, src, err := m.Store.Load()
	if err != nil {
		return "", err
	}
	if c == nil || !c.HasToken() {
		return "", ErrNotConnected
	}
	if src == secrets.SourceEnv {
		return c.AccessToken, nil
	}
	leeway := m.Leeway
	if leeway == 0 {
		leeway = 60 * time.Second
	}
	if c.AccessToken != "" && (c.Expiry.IsZero() || m.now().Add(leeway).Before(c.Expiry)) {
		return c.AccessToken, nil
	}
	return m.refreshLocked(ctx, c)
}

// Refresh forces a refresh (used after a 401).
func (m *Manager) Refresh(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, src, err := m.Store.Load()
	if err != nil {
		return "", err
	}
	if c == nil || !c.HasToken() {
		return "", ErrNotConnected
	}
	if src == secrets.SourceEnv {
		return "", fmt.Errorf("%w: IBKR_ACCESS_TOKEN was rejected by the server", ErrAuthExpired)
	}
	return m.refreshLocked(ctx, c)
}

func (m *Manager) refreshLocked(ctx context.Context, c *secrets.Credentials) (string, error) {
	if c.RefreshToken == "" || c.TokenEndpoint == "" || c.ClientID == "" {
		return "", fmt.Errorf("%w: no refresh token stored", ErrAuthExpired)
	}
	ep := &Endpoints{Token: c.TokenEndpoint, Resource: c.Server}
	cl := &Client{ID: c.ClientID, Secret: c.ClientSecret, RedirectURI: c.RedirectURI}
	t, err := RefreshToken(ctx, m.hc(), ep, cl, c.RefreshToken)
	if err != nil {
		return "", err
	}
	c.AccessToken, c.RefreshToken, c.Expiry = t.AccessToken, t.RefreshToken, t.Expiry
	if t.TokenType != "" {
		c.TokenType = t.TokenType
	}
	if len(t.Scopes) > 0 {
		c.Scopes = t.Scopes
	}
	if _, err := m.Store.Save(c); err != nil {
		return "", fmt.Errorf("refreshed token but could not store it: %w", err)
	}
	return t.AccessToken, nil
}
