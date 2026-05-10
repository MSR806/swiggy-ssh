package identity

import (
	"context"
	"testing"
	"time"
)

type testSessionRepo struct {
	created TerminalSession
	endedID string
	endedAt time.Time
}

func (r *testSessionRepo) CreateTerminalSession(_ context.Context, session TerminalSession) (TerminalSession, error) {
	session.ID = "session-1"
	now := time.Now().UTC()
	session.CreatedAt = now
	r.created = session
	return session, nil
}

func (r *testSessionRepo) MarkTerminalSessionEnded(_ context.Context, sessionID string, endedAt time.Time) error {
	r.endedID = sessionID
	r.endedAt = endedAt
	return nil
}

func TestSessionTrackerStartSessionLinksResolvedIdentity(t *testing.T) {
	repo := &testSessionRepo{}
	tracker := NewSessionTracker(repo)

	userID := "user-1"
	sshIdentityID := "identity-1"
	fingerprint := "SHA256:abc"
	selectedAddressID := SelectedAddressIDUnsetPlaceholder

	created, err := tracker.StartSession(context.Background(), TrackSessionInput{
		Client:            ClientProtocolSSH,
		ClientSessionID:   "conn-1",
		SSHFingerprint:    &fingerprint,
		CurrentScreen:     ScreenSSHSessionPlaceholder,
		SelectedAddressID: &selectedAddressID,
		ResolvedIdentity: &SessionIdentity{
			User:        User{ID: userID},
			SSHIdentity: SSHIdentity{ID: sshIdentityID},
		},
	})
	if err != nil {
		t.Fatalf("start session: %v", err)
	}

	if created.ID == "" {
		t.Fatal("expected created session id")
	}
	if repo.created.UserID == nil || *repo.created.UserID != userID {
		t.Fatalf("expected user_id %s", userID)
	}
	if repo.created.SSHIdentityID == nil || *repo.created.SSHIdentityID != sshIdentityID {
		t.Fatalf("expected ssh_identity_id %s", sshIdentityID)
	}
	if repo.created.CurrentScreen != ScreenSSHSessionPlaceholder {
		t.Fatalf("unexpected current screen: %s", repo.created.CurrentScreen)
	}
}

func TestSessionTrackerEndSessionMarksEndedAt(t *testing.T) {
	repo := &testSessionRepo{}
	tracker := NewSessionTracker(repo)
	fixedNow := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC)
	tracker.now = func() time.Time { return fixedNow }

	if err := tracker.EndSession(context.Background(), "session-1"); err != nil {
		t.Fatalf("end session: %v", err)
	}

	if repo.endedID != "session-1" {
		t.Fatalf("expected ended session id session-1, got %s", repo.endedID)
	}
	if !repo.endedAt.Equal(fixedNow) {
		t.Fatalf("expected ended at %v, got %v", fixedNow, repo.endedAt)
	}
}
