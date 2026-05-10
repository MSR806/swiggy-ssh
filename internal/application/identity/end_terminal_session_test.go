package identity

import (
	"context"
	"testing"
	"time"
)

func TestEndTerminalSessionMarksEndedAt(t *testing.T) {
	repo := &testSessionRepo{}
	useCase := NewEndTerminalSessionUseCase(repo)
	fixedNow := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC)
	useCase.now = func() time.Time { return fixedNow }

	if err := useCase.Execute(context.Background(), EndTerminalSessionInput{SessionID: "session-1"}); err != nil {
		t.Fatalf("end session: %v", err)
	}

	if repo.endedID != "session-1" {
		t.Fatalf("expected ended session id session-1, got %s", repo.endedID)
	}
	if !repo.endedAt.Equal(fixedNow) {
		t.Fatalf("expected ended at %v, got %v", fixedNow, repo.endedAt)
	}
}
