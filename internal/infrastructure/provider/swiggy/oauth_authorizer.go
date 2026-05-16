package swiggy

import (
	"context"
	"net/http"
	"strings"
	"time"

	domainauth "swiggy-ssh/internal/domain/auth"
)

const oauthProvider = "swiggy"

type OAuthAccountAuthorizer struct {
	repo domainauth.Repository
	now  func() time.Time
}

func NewOAuthAccountAuthorizer(repo domainauth.Repository) *OAuthAccountAuthorizer {
	return &OAuthAccountAuthorizer{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (a *OAuthAccountAuthorizer) AuthorizeMCPRequest(ctx context.Context, req *http.Request) error {
	userID, ok := domainauth.UserIDFromContext(ctx)
	if !ok {
		return domainauth.ErrOAuthAccountUserRequired
	}
	account, err := a.repo.FindOAuthAccountByUserAndProvider(ctx, userID, oauthProvider)
	if err != nil {
		return err
	}
	if strings.TrimSpace(account.AccessToken) == "" {
		return domainauth.ErrTokenReconnectRequired
	}
	if err := domainauth.ValidateTokenForUse(account, a.now()); err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+account.AccessToken)
	return nil
}
