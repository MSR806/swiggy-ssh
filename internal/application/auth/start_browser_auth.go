package auth

import (
	"context"
	"fmt"
)

type StartBrowserAuthInput struct {
	AttemptToken string
	CallbackURL  string
}

type StartBrowserAuthOutput struct {
	RedirectURL string
}

type StartBrowserAuthUseCase struct {
	attemptSvc BrowserAuthAttemptService
	provider   BrowserAuthProvider
}

func NewStartBrowserAuthUseCase(attemptSvc BrowserAuthAttemptService, provider BrowserAuthProvider) *StartBrowserAuthUseCase {
	return &StartBrowserAuthUseCase{attemptSvc: attemptSvc, provider: provider}
}

func (s *StartBrowserAuthUseCase) Execute(ctx context.Context, input StartBrowserAuthInput) (StartBrowserAuthOutput, error) {
	attempt, err := s.attemptSvc.GetAuthAttempt(ctx, input.AttemptToken)
	if err != nil {
		return StartBrowserAuthOutput{}, err
	}
	if attempt.Status != AuthAttemptStatusPending {
		return StartBrowserAuthOutput{}, ErrAuthAttemptAlreadyUsed
	}
	if s.provider == nil {
		return StartBrowserAuthOutput{}, ErrBrowserAuthProviderUnavailable
	}
	started, err := s.provider.StartBrowserAuth(ctx, BrowserAuthStartInput{
		State:        input.AttemptToken,
		CallbackURL:  input.CallbackURL,
		CodeVerifier: attempt.CodeVerifier,
	})
	if err != nil {
		return StartBrowserAuthOutput{}, fmt.Errorf("start provider browser auth: %w", err)
	}
	return StartBrowserAuthOutput{RedirectURL: started.RedirectURL}, nil
}
