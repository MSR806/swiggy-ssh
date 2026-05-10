package identity

import (
	"context"
	"time"
)

type EndTerminalSessionInput struct {
	SessionID string
}

type EndTerminalSessionUseCase struct {
	repo SessionRepository
	now  func() time.Time
}

func NewEndTerminalSessionUseCase(repo SessionRepository) *EndTerminalSessionUseCase {
	return &EndTerminalSessionUseCase{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (uc *EndTerminalSessionUseCase) Execute(ctx context.Context, input EndTerminalSessionInput) error {
	return uc.repo.MarkTerminalSessionEnded(ctx, input.SessionID, uc.now())
}
