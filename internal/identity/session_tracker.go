package identity

import (
	"context"
	"time"
)

const (
	ClientProtocolSSH                 = "ssh"
	ScreenSSHSessionPlaceholder       = "ssh_session_placeholder"
	SelectedAddressIDUnsetPlaceholder = "unselected"
)

type TerminalSession struct {
	ID                string
	Client            string
	ClientSessionID   string
	UserID            *string
	SSHIdentityID     *string
	SSHFingerprint    *string
	CurrentScreen     string
	SelectedAddressID *string
	CreatedAt         time.Time
	LastSeenAt        *time.Time
	EndedAt           *time.Time
}

type SessionRepository interface {
	CreateTerminalSession(ctx context.Context, session TerminalSession) (TerminalSession, error)
	MarkTerminalSessionEnded(ctx context.Context, sessionID string, endedAt time.Time) error
}

type TrackSessionInput struct {
	Client            string
	ClientSessionID   string
	SSHFingerprint    *string
	CurrentScreen     string
	SelectedAddressID *string
	ResolvedIdentity  *SessionIdentity
}

type SessionTracker struct {
	repo SessionRepository
	now  func() time.Time
}

func NewSessionTracker(repo SessionRepository) *SessionTracker {
	return &SessionTracker{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (t *SessionTracker) StartSession(ctx context.Context, input TrackSessionInput) (TerminalSession, error) {
	var userID *string
	var sshIdentityID *string
	if input.ResolvedIdentity != nil {
		userID = &input.ResolvedIdentity.User.ID
		sshIdentityID = &input.ResolvedIdentity.SSHIdentity.ID
	}

	return t.repo.CreateTerminalSession(ctx, TerminalSession{
		Client:            input.Client,
		ClientSessionID:   input.ClientSessionID,
		UserID:            userID,
		SSHIdentityID:     sshIdentityID,
		SSHFingerprint:    input.SSHFingerprint,
		CurrentScreen:     input.CurrentScreen,
		SelectedAddressID: input.SelectedAddressID,
	})
}

func (t *SessionTracker) EndSession(ctx context.Context, sessionID string) error {
	return t.repo.MarkTerminalSessionEnded(ctx, sessionID, t.now())
}
