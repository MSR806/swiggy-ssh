package swiggy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"swiggy-ssh/internal/domain/auth"
)

func TestStartBrowserAuthBuildsOAuthPKCERedirect(t *testing.T) {
	client := NewBrowserAuthClient(BrowserAuthConfig{
		AuthorizeURL: "https://mcp.swiggy.com/auth/authorize",
		TokenURL:     "https://mcp.swiggy.com/auth/token",
		ClientID:     "client-1",
		Scopes:       []string{"mcp:tools"},
	})

	out, err := client.StartBrowserAuth(context.Background(), auth.BrowserAuthStartInput{
		State:        "state-1",
		CallbackURL:  "http://localhost:8080/auth/callback",
		CodeVerifier: "verifier-1",
	})
	if err != nil {
		t.Fatalf("start auth: %v", err)
	}
	u, err := url.Parse(out.RedirectURL)
	if err != nil {
		t.Fatalf("parse redirect: %v", err)
	}
	q := u.Query()
	assertQuery(t, q, "response_type", "code")
	assertQuery(t, q, "client_id", "client-1")
	assertQuery(t, q, "redirect_uri", "http://localhost:8080/auth/callback")
	assertQuery(t, q, "code_challenge", codeChallenge("verifier-1"))
	assertQuery(t, q, "code_challenge_method", "S256")
	assertQuery(t, q, "state", "state-1")
	assertQuery(t, q, "scope", "mcp:tools")
}

func TestExchangeBrowserAuthCallbackPostsVerifierAndReturnsCredentials(t *testing.T) {
	var got tokenRequest
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"token-1","token_type":"Bearer","expires_in":3600,"scope":"mcp:tools profile:read"}`))
	}))
	defer tokenServer.Close()

	now := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	client := NewBrowserAuthClient(BrowserAuthConfig{
		AuthorizeURL: "https://mcp.swiggy.com/auth/authorize",
		TokenURL:     tokenServer.URL,
		ClientID:     "client-1",
		Scopes:       []string{"mcp:tools"},
	})
	client.now = func() time.Time { return now }

	credentials, err := client.ExchangeBrowserAuthCallback(context.Background(), auth.BrowserAuthCallbackInput{
		Code:         "code-1",
		CallbackURL:  "http://localhost:8080/auth/callback",
		CodeVerifier: "verifier-1",
	})
	if err != nil {
		t.Fatalf("exchange callback: %v", err)
	}
	if got.GrantType != "authorization_code" || got.Code != "code-1" || got.CodeVerifier != "verifier-1" || got.ClientID != "client-1" || got.RedirectURI != "http://localhost:8080/auth/callback" {
		t.Fatalf("unexpected token request: %+v", got)
	}
	if credentials.AccessToken != "token-1" {
		t.Fatal("expected access token from token response")
	}
	if credentials.TokenExpiresAt == nil || !credentials.TokenExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected expires at: %v", credentials.TokenExpiresAt)
	}
	if len(credentials.Scopes) != 2 || credentials.Scopes[0] != "mcp:tools" || credentials.Scopes[1] != "profile:read" {
		t.Fatalf("unexpected scopes: %#v", credentials.Scopes)
	}
}

func assertQuery(t *testing.T, q url.Values, key, want string) {
	t.Helper()
	if got := q.Get(key); got != want {
		t.Fatalf("expected %s=%q, got %q", key, want, got)
	}
}
