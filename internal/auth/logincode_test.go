package auth_test

import (
	"context"
	"testing"
	"time"

	"swiggy-ssh/internal/auth"
	"swiggy-ssh/internal/provider/mock"
)

func newMock(ttl time.Duration) *mock.MockLoginCodeService {
	return mock.NewMockLoginCodeService(ttl)
}

func TestLoginCodePending(t *testing.T) {
	svc := newMock(10 * time.Minute)
	rawCode, record, err := svc.IssueLoginCode(context.Background(), "user-1", "sess-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if rawCode == "" {
		t.Fatal("expected raw code")
	}
	if record.Status != auth.LoginCodeStatusPending {
		t.Fatalf("expected pending, got %s", record.Status)
	}
	// raw code must not equal the hash
	if rawCode == record.CodeHash {
		t.Fatal("raw code must not equal code hash")
	}

	got, err := svc.GetLoginCode(context.Background(), rawCode)
	if err != nil {
		t.Fatalf("get pending: %v", err)
	}
	if got.Status != auth.LoginCodeStatusPending {
		t.Fatalf("expected pending on get, got %s", got.Status)
	}
}

func TestLoginCodeCompleted(t *testing.T) {
	svc := newMock(10 * time.Minute)
	rawCode, _, err := svc.IssueLoginCode(context.Background(), "user-1", "sess-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if err := svc.CompleteLoginCode(context.Background(), rawCode); err != nil {
		t.Fatalf("complete: %v", err)
	}

	got, err := svc.GetLoginCode(context.Background(), rawCode)
	if err != nil {
		t.Fatalf("get completed: %v", err)
	}
	if got.Status != auth.LoginCodeStatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}

	// Completed code cannot be reused.
	if err := svc.CompleteLoginCode(context.Background(), rawCode); err != auth.ErrLoginCodeAlreadyUsed {
		t.Fatalf("expected ErrLoginCodeAlreadyUsed on second complete, got %v", err)
	}
}

func TestLoginCodeExpired(t *testing.T) {
	svc := mock.NewMockLoginCodeService(1 * time.Millisecond)
	// Inject a clock that starts at now but can be advanced.
	base := time.Now().UTC()
	svc.SetNow(func() time.Time { return base })

	rawCode, _, err := svc.IssueLoginCode(context.Background(), "user-1", "sess-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	// Advance clock past TTL.
	svc.SetNow(func() time.Time { return base.Add(2 * time.Millisecond) })

	_, err = svc.GetLoginCode(context.Background(), rawCode)
	if err != auth.ErrLoginCodeNotFound {
		t.Fatalf("expected ErrLoginCodeNotFound after expiry, got %v", err)
	}

	// Cannot complete expired code.
	if err := svc.CompleteLoginCode(context.Background(), rawCode); err != auth.ErrLoginCodeNotFound {
		t.Fatalf("expected ErrLoginCodeNotFound on complete after expiry, got %v", err)
	}
}

func TestLoginCodeCancelled(t *testing.T) {
	svc := newMock(10 * time.Minute)
	rawCode, _, err := svc.IssueLoginCode(context.Background(), "user-1", "sess-1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	if err := svc.CancelLoginCode(context.Background(), rawCode); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	got, err := svc.GetLoginCode(context.Background(), rawCode)
	if err != nil {
		t.Fatalf("get cancelled: %v", err)
	}
	if got.Status != auth.LoginCodeStatusCancelled {
		t.Fatalf("expected cancelled, got %s", got.Status)
	}

	// Cancelled code cannot be completed.
	if err := svc.CompleteLoginCode(context.Background(), rawCode); err != auth.ErrLoginCodeAlreadyUsed {
		t.Fatalf("expected ErrLoginCodeAlreadyUsed on complete of cancelled, got %v", err)
	}
}

func TestLoginCodeRawNeverEqualsHash(t *testing.T) {
	svc := newMock(10 * time.Minute)
	for i := 0; i < 5; i++ {
		rawCode, record, err := svc.IssueLoginCode(context.Background(), "user-x", "sess-x")
		if err != nil {
			t.Fatalf("issue %d: %v", i, err)
		}
		if rawCode == record.CodeHash {
			t.Fatalf("iteration %d: raw code equals hash — raw code is being stored", i)
		}
		if record.CodeHash == "" {
			t.Fatalf("iteration %d: code hash is empty", i)
		}
	}
}
