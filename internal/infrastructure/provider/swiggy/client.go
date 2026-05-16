package swiggy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"swiggy-ssh/internal/domain/auth"
)

// Client is the provider adapter contract for Swiggy APIs.
type Client interface {
	Ping(ctx context.Context) error
}

type BrowserAuthConfig struct {
	AuthorizeURL string
	TokenURL     string
	ClientID     string
	Scopes       []string
}

// BrowserAuthClient implements Swiggy OAuth 2.1 + PKCE browser authentication.
type BrowserAuthClient struct {
	authorizeURL string
	tokenURL     string
	clientID     string
	scopes       []string
	httpClient   *http.Client
	now          func() time.Time
}

func NewBrowserAuthClient(cfg BrowserAuthConfig) *BrowserAuthClient {
	return &BrowserAuthClient{
		authorizeURL: cfg.AuthorizeURL,
		tokenURL:     cfg.TokenURL,
		clientID:     cfg.ClientID,
		scopes:       cfg.Scopes,
		httpClient:   http.DefaultClient,
		now:          func() time.Time { return time.Now().UTC() },
	}
}

func (c *BrowserAuthClient) StartBrowserAuth(ctx context.Context, input auth.BrowserAuthStartInput) (auth.BrowserAuthStartOutput, error) {
	if c.authorizeURL == "" || c.clientID == "" || input.CodeVerifier == "" {
		return auth.BrowserAuthStartOutput{}, auth.ErrBrowserAuthProviderUnavailable
	}
	u, err := url.Parse(c.authorizeURL)
	if err != nil {
		return auth.BrowserAuthStartOutput{}, errors.Join(auth.ErrBrowserAuthProviderUnavailable, err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", c.clientID)
	q.Set("redirect_uri", input.CallbackURL)
	q.Set("code_challenge", codeChallenge(input.CodeVerifier))
	q.Set("code_challenge_method", "S256")
	q.Set("state", input.State)
	q.Set("scope", strings.Join(c.scopes, " "))
	u.RawQuery = q.Encode()
	return auth.BrowserAuthStartOutput{RedirectURL: u.String()}, nil
}

func (c *BrowserAuthClient) ExchangeBrowserAuthCallback(ctx context.Context, input auth.BrowserAuthCallbackInput) (auth.BrowserAuthCredentials, error) {
	if c.tokenURL == "" || c.clientID == "" || input.CodeVerifier == "" {
		return auth.BrowserAuthCredentials{}, auth.ErrBrowserAuthProviderUnavailable
	}
	body, err := json.Marshal(tokenRequest{
		GrantType:    "authorization_code",
		Code:         input.Code,
		CodeVerifier: input.CodeVerifier,
		ClientID:     c.clientID,
		RedirectURI:  input.CallbackURL,
	})
	if err != nil {
		return auth.BrowserAuthCredentials{}, fmt.Errorf("marshal token request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, bytes.NewReader(body))
	if err != nil {
		return auth.BrowserAuthCredentials{}, errors.Join(auth.ErrBrowserAuthProviderCallback, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return auth.BrowserAuthCredentials{}, errors.Join(auth.ErrBrowserAuthProviderCallback, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, resp.Body)
		return auth.BrowserAuthCredentials{}, errors.Join(auth.ErrBrowserAuthProviderCallback, fmt.Errorf("token endpoint status %d", resp.StatusCode))
	}

	var token tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return auth.BrowserAuthCredentials{}, errors.Join(auth.ErrBrowserAuthProviderCallback, err)
	}
	if token.AccessToken == "" || token.ExpiresIn <= 0 {
		return auth.BrowserAuthCredentials{}, errors.Join(auth.ErrBrowserAuthProviderCallback, errors.New("invalid token response"))
	}
	expiresAt := c.now().Add(time.Duration(token.ExpiresIn) * time.Second)
	scopes := c.scopes
	if token.Scope != "" {
		scopes = strings.Fields(strings.ReplaceAll(token.Scope, ",", " "))
	}

	return auth.BrowserAuthCredentials{
		AccessToken:    token.AccessToken,
		TokenExpiresAt: &expiresAt,
		Scopes:         scopes,
	}, nil
}

type tokenRequest struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
