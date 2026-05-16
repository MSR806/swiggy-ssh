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

func TestStartTerminalSessionLinksResolvedIdentity(t *testing.T) {
	repo := &testSessionRepo{}
	useCase := NewStartTerminalSessionUseCase(repo)

	userID := "user-1"
	sshIdentityID := "identity-1"
	fingerprint := "SHA256:abc"
	selectedAddressID := SelectedAddressIDUnsetPlaceholder

	created, err := useCase.Execute(context.Background(), StartTerminalSessionInput{
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

func TestStartTerminalSessionAllowsGuestIdentity(t *testing.T) {
	repo := &testSessionRepo{}
	useCase := NewStartTerminalSessionUseCase(repo)
	selectedAddressID := SelectedAddressIDUnsetPlaceholder

	_, err := useCase.Execute(context.Background(), StartTerminalSessionInput{
		Client:            ClientProtocolSSH,
		ClientSessionID:   "conn-guest",
		CurrentScreen:     ScreenSSHSessionPlaceholder,
		SelectedAddressID: &selectedAddressID,
	})
	if err != nil {
		t.Fatalf("start guest session: %v", err)
	}

	if repo.created.UserID != nil {
		t.Fatalf("guest session must not have user_id: %v", *repo.created.UserID)
	}
	if repo.created.SSHIdentityID != nil {
		t.Fatalf("guest session must not have ssh_identity_id: %v", *repo.created.SSHIdentityID)
	}
	if repo.created.SSHFingerprint != nil {
		t.Fatalf("no-key guest session must not have ssh fingerprint: %v", *repo.created.SSHFingerprint)
	}
}
