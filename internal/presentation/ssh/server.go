package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	"swiggy-ssh/internal/application/auth"
	"swiggy-ssh/internal/application/identity"
	"swiggy-ssh/internal/presentation/tui"

	"golang.org/x/crypto/ssh"
)

// Server is the SSH transport boundary for session handling.
type Server interface {
	Start(ctx context.Context) error
}

// SSHServer handles SSH listener and session mechanics.
type SSHServer struct {
	addr          string
	hostKeyPath   string
	logger        *slog.Logger
	resolver      *identity.ResolveSSHIdentityUseCase
	startSession  *identity.StartTerminalSessionUseCase
	endSession    *identity.EndTerminalSessionUseCase
	loginCodeSvc  auth.LoginCodeService
	publicBaseURL string
	authUseCase   *auth.EnsureValidAccountUseCase
}

type activeConnTracker struct {
	mu           sync.Mutex
	conns        map[net.Conn]struct{}
	shuttingDown bool
}

func newActiveConnTracker() *activeConnTracker {
	return &activeConnTracker{conns: make(map[net.Conn]struct{})}
}

func (t *activeConnTracker) add(conn net.Conn) {
	t.mu.Lock()
	if t.shuttingDown {
		t.mu.Unlock()
		_ = conn.Close()
		return
	}
	t.conns[conn] = struct{}{}
	t.mu.Unlock()
}

func (t *activeConnTracker) remove(conn net.Conn) {
	t.mu.Lock()
	delete(t.conns, conn)
	t.mu.Unlock()
}

func (t *activeConnTracker) closeAll() {
	t.mu.Lock()
	t.shuttingDown = true
	conns := make([]net.Conn, 0, len(t.conns))
	for conn := range t.conns {
		conns = append(conns, conn)
	}
	t.mu.Unlock()

	for _, conn := range conns {
		_ = conn.Close()
	}
}

func New(addr, hostKeyPath string, logger *slog.Logger, resolver *identity.ResolveSSHIdentityUseCase, startSession *identity.StartTerminalSessionUseCase, endSession *identity.EndTerminalSessionUseCase, loginCodeSvc auth.LoginCodeService, publicBaseURL string, authUseCase *auth.EnsureValidAccountUseCase) *SSHServer {
	return &SSHServer{
		addr:          addr,
		hostKeyPath:   hostKeyPath,
		logger:        logger,
		resolver:      resolver,
		startSession:  startSession,
		endSession:    endSession,
		loginCodeSvc:  loginCodeSvc,
		publicBaseURL: publicBaseURL,
		authUseCase:   authUseCase,
	}
}

func (s *SSHServer) Start(ctx context.Context) error {
	hostSigner, err := LoadOrCreateHostKey(s.hostKeyPath)
	if err != nil {
		return fmt.Errorf("load or create host key: %w", err)
	}

	serverConfig := newServerConfig()
	serverConfig.AddHostKey(hostSigner)

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.addr, err)
	}
	defer listener.Close()

	s.logger.InfoContext(ctx, "ssh server listening",
		"addr", s.addr,
		"host_key_fingerprint", ssh.FingerprintSHA256(hostSigner.PublicKey()),
	)

	var wg sync.WaitGroup
	tracker := newActiveConnTracker()
	go func() {
		<-ctx.Done()
		_ = listener.Close()
		tracker.closeAll()
	}()

	for {
		rawConn, acceptErr := listener.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil {
				wg.Wait()
				return nil
			}

			if errors.Is(acceptErr, net.ErrClosed) {
				wg.Wait()
				return nil
			}

			s.logger.ErrorContext(ctx, "ssh accept failed", "error", acceptErr)
			continue
		}

		tracker.add(rawConn)
		wg.Add(1)
		go func(conn net.Conn) {
			defer wg.Done()
			defer tracker.remove(conn)
			s.handleConn(ctx, conn, serverConfig)
		}(rawConn)
	}
}

func newServerConfig() *ssh.ServerConfig {
	return &ssh.ServerConfig{
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return publicKeyPermissions(key), nil
		},
		// Accept keyboard-interactive without prompts for clients that have no SSH key.
		// Public-key clients still use PublicKeyCallback, preserving fingerprint metadata.
		KeyboardInteractiveCallback: func(conn ssh.ConnMetadata, client ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			return nil, nil
		},
	}
}

func publicKeyPermissions(key ssh.PublicKey) *ssh.Permissions {
	return &ssh.Permissions{
		Extensions: map[string]string{
			"pubkey_fingerprint": ssh.FingerprintSHA256(key),
			"pubkey_type":        key.Type(),
			"pubkey_authorized":  strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))),
		},
	}
}

func (s *SSHServer) handleConn(ctx context.Context, netConn net.Conn, serverConfig *ssh.ServerConfig) {
	defer netConn.Close()

	sshConn, chans, reqs, err := ssh.NewServerConn(netConn, serverConfig)
	if err != nil {
		s.logger.WarnContext(ctx, "ssh handshake failed",
			"remote_addr", netConn.RemoteAddr().String(),
			"error", err,
		)
		return
	}
	defer sshConn.Close()

	fingerprint := ""
	pubKeyType := ""
	publicKeyAuthorized := ""
	if sshConn.Permissions != nil {
		fingerprint = sshConn.Permissions.Extensions["pubkey_fingerprint"]
		pubKeyType = sshConn.Permissions.Extensions["pubkey_type"]
		publicKeyAuthorized = sshConn.Permissions.Extensions["pubkey_authorized"]
	}

	var resolvedIdentity *identity.SessionIdentity
	if s.resolver != nil && publicKeyAuthorized != "" {
		publicKey, _, _, _, parseErr := ssh.ParseAuthorizedKey([]byte(publicKeyAuthorized))
		if parseErr != nil {
			s.logger.WarnContext(ctx, "ssh public key parse failed", "error", parseErr)
		} else {
			resolved, resolveErr := s.resolver.Execute(ctx, identity.ResolveSSHIdentityInput{
				Client: identity.ClientProtocolSSH,
				Key:    publicKey,
			})
			if errors.Is(resolveErr, identity.ErrNotFound) {
				s.logger.InfoContext(ctx, "ssh identity unknown; starting guest session",
					"remote_addr", sshConn.RemoteAddr().String(),
					"pubkey_fingerprint", fingerprint,
				)
			} else if errors.Is(resolveErr, identity.ErrSSHIdentityRevoked) {
				s.logger.InfoContext(ctx, "ssh identity revoked; starting guest session",
					"remote_addr", sshConn.RemoteAddr().String(),
					"pubkey_fingerprint", fingerprint,
				)
			} else if resolveErr != nil {
				s.logger.WarnContext(ctx, "ssh identity resolution failed",
					"remote_addr", sshConn.RemoteAddr().String(),
					"pubkey_fingerprint", fingerprint,
					"error", resolveErr,
				)
			} else {
				resolvedIdentity = &resolved
			}
		}
	}

	var terminalSessionID string
	if s.startSession != nil {
		selectedAddressID := identity.SelectedAddressIDUnsetPlaceholder
		var sshFingerprint *string
		if fingerprint != "" {
			sshFingerprint = &fingerprint
		}
		trackedSession, trackErr := s.startSession.Execute(ctx, identity.StartTerminalSessionInput{
			Client:            identity.ClientProtocolSSH,
			ClientSessionID:   fmt.Sprintf("%x", sshConn.SessionID()),
			SSHFingerprint:    sshFingerprint,
			CurrentScreen:     identity.ScreenSSHSessionPlaceholder,
			SelectedAddressID: &selectedAddressID,
			ResolvedIdentity:  resolvedIdentity,
		})
		if trackErr != nil {
			s.logger.WarnContext(ctx, "terminal session track start failed",
				"remote_addr", sshConn.RemoteAddr().String(),
				"pubkey_fingerprint", fingerprint,
				"error", trackErr,
			)
		} else {
			terminalSessionID = trackedSession.ID
			if s.endSession != nil {
				defer func() {
					endCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					if endErr := s.endSession.Execute(endCtx, identity.EndTerminalSessionInput{SessionID: trackedSession.ID}); endErr != nil {
						s.logger.WarnContext(ctx, "terminal session track end failed",
							"terminal_session_id", trackedSession.ID,
							"error", endErr,
						)
					}
				}()
			}
		}
	}

	go ssh.DiscardRequests(reqs)

	var resolvedUserID string
	if resolvedIdentity != nil {
		resolvedUserID = resolvedIdentity.User.ID
	}

	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			_ = newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		channel, requests, acceptErr := newChan.Accept()
		if acceptErr != nil {
			s.logger.WarnContext(ctx, "ssh session channel accept failed", "error", acceptErr)
			continue
		}

		go s.handleSessionChannel(ctx, sshConn, channel, requests, terminalSessionID, fingerprint, pubKeyType, resolvedUserID)
	}
}

func (s *SSHServer) handleSessionChannel(ctx context.Context, conn *ssh.ServerConn, ch ssh.Channel, reqs <-chan *ssh.Request, terminalSessionID, fingerprint, pubKeyType, resolvedUserID string) {
	defer ch.Close()

	started := false
	viewport := tui.Viewport{}
	for req := range reqs {
		switch req.Type {
		case "shell", "exec":
			started = true
			_ = req.Reply(true, nil)
		case "pty-req", "window-change":
			if parsed, ok := parseViewportRequest(req); ok {
				viewport = parsed
			}
			_ = req.Reply(true, nil)
		case "env":
			_ = req.Reply(true, nil)
		default:
			_ = req.Reply(false, nil)
		}

		if started {
			break
		}
	}

	if !started {
		return
	}
	go discardSessionRequests(reqs)

	fallbackMsg := fmt.Sprintf(
		"Welcome to swiggy-ssh (MVP skeleton).\nuser=%s\nremote=%s\nkey_type=%s\nkey_fingerprint=%s\n\n",
		conn.User(),
		conn.RemoteAddr().String(),
		pubKeyType,
		fingerprint,
	)

	s.runSession(tui.WithViewport(ctx, viewport), ch, fallbackMsg, terminalSessionID, fingerprint, resolvedUserID)

	_, _ = ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{Status: 0}))

	s.logger.InfoContext(ctx, "ssh session handled",
		"user", conn.User(),
		"remote_addr", conn.RemoteAddr().String(),
		"pubkey_type", pubKeyType,
		"pubkey_fingerprint", fingerprint,
		"terminal_session_id", terminalSessionID,
		"handled_at", time.Now().UTC().Format(time.RFC3339),
	)
}

type ptyRequestPayload struct {
	Term          string
	Columns       uint32
	Rows          uint32
	WidthPixels   uint32
	HeightPixels  uint32
	TerminalModes string
}

type windowChangePayload struct {
	Columns      uint32
	Rows         uint32
	WidthPixels  uint32
	HeightPixels uint32
}

func parseViewportRequest(req *ssh.Request) (tui.Viewport, bool) {
	switch req.Type {
	case "pty-req":
		var payload ptyRequestPayload
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			return tui.Viewport{}, false
		}
		return viewportFromCells(payload.Columns, payload.Rows)
	case "window-change":
		var payload windowChangePayload
		if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
			return tui.Viewport{}, false
		}
		return viewportFromCells(payload.Columns, payload.Rows)
	default:
		return tui.Viewport{}, false
	}
}

func viewportFromCells(columns, rows uint32) (tui.Viewport, bool) {
	if columns == 0 || rows == 0 {
		return tui.Viewport{}, false
	}
	return tui.Viewport{Width: int(columns), Height: int(rows)}, true
}

func discardSessionRequests(reqs <-chan *ssh.Request) {
	for req := range reqs {
		switch req.Type {
		case "window-change", "env":
			_ = req.Reply(true, nil)
		default:
			_ = req.Reply(false, nil)
		}
	}
}

// runSession drives the screen routing logic for an established SSH session channel.
// It is called after the request loop confirms a shell/exec was started.
func (s *SSHServer) runSession(ctx context.Context, ch ssh.Channel, fallbackMsg, terminalSessionID, fingerprint, resolvedUserID string) {
	if s.loginCodeSvc == nil || terminalSessionID == "" {
		_, _ = io.WriteString(ch, fallbackMsg)
		_, _ = io.WriteString(ch, "Session handler placeholder complete. Goodbye.\n")
		return
	}
	if err := tui.EnterFullscreen(ch); err != nil {
		s.logger.WarnContext(ctx, "failed to enter fullscreen tui", "error", err)
		return
	}
	defer func() { _ = tui.ExitFullscreen(ch) }()

	render := func(renderCtx context.Context, view tui.View) {
		_ = tui.ClearScreen(ch)
		_ = view.Render(renderCtx, ch)
	}

	// === HOME SCREEN — always shown first ===
	_ = tui.ClearScreen(ch)
	action, err := tui.HomeView{In: ch}.RenderWithAction(ctx, ch)
	if err != nil || action != tui.HomeActionInstamart {
		// User quit or selected a coming-soon item — end session.
		return
	}

	// === User selected Instamart — check/establish auth ===

	// RETURNING USER FAST-PATH: valid account already exists → go straight to Instamart.
	if s.authUseCase != nil && resolvedUserID != "" {
		_, fastErr := s.authUseCase.Execute(ctx, auth.EnsureValidAccountInput{
			UserID:         resolvedUserID,
			AllowFirstAuth: false,
		})
		if fastErr == nil {
			render(ctx, tui.InstamartPlaceholderView{UserID: resolvedUserID, In: ch})
			return
		}
		if errors.Is(fastErr, auth.ErrAccountRevoked) {
			render(ctx, tui.RevokedView{})
			return
		}
		// expired/reconnect_required/not-found — fall through to login flow.
	}

	// LOGIN CODE FLOW
	rawCode, _, issueErr := s.loginCodeSvc.IssueLoginCode(ctx, resolvedUserID, terminalSessionID)
	if issueErr != nil {
		s.logger.WarnContext(ctx, "failed to issue login code", "error", issueErr)
		render(ctx, tui.ErrorView{Message: "Login unavailable. Please try again later."})
		return
	}

	loginURL := s.publicBaseURL + "/login"
	render(ctx, tui.LoginWaitingView{LoginURL: loginURL, RawCode: rawCode})

	completed, pollErr := pollLoginCode(ctx, s.loginCodeSvc, rawCode, s.logger)
	if pollErr != nil {
		render(ctx, tui.ErrorView{Message: "Login polling error. Session ending."})
		return
	}
	if !completed {
		render(ctx, tui.ErrorView{Message: "Login expired or cancelled. Please reconnect."})
		return
	}

	// Login confirmed — check/establish account.
	if resolvedUserID == "" {
		render(ctx, tui.InstamartPlaceholderView{StatusMessage: "Guest session connected for this SSH session.", In: ch})
		return
	}
	if s.authUseCase == nil {
		render(ctx, tui.LoginSuccessView{})
		return
	}

	reauthFn := func(reauthCtx context.Context) error {
		newCode, _, reauthIssueErr := s.loginCodeSvc.IssueLoginCode(reauthCtx, resolvedUserID, terminalSessionID)
		if reauthIssueErr != nil {
			return fmt.Errorf("issue reauth login code: %w", reauthIssueErr)
		}
		render(reauthCtx, tui.ReconnectView{RawCode: newCode})
		reauthCompleted, reauthPollErr := pollLoginCode(reauthCtx, s.loginCodeSvc, newCode, s.logger)
		if reauthPollErr != nil {
			return fmt.Errorf("poll reauth: %w", reauthPollErr)
		}
		if !reauthCompleted {
			return errors.New("reauth login code expired or cancelled")
		}
		return nil
	}

	result, authErr := s.authUseCase.Execute(ctx, auth.EnsureValidAccountInput{
		UserID:         resolvedUserID,
		AllowFirstAuth: true,
		Reauth:         reauthFn,
	})
	switch {
	case errors.Is(authErr, auth.ErrAccountRevoked):
		render(ctx, tui.RevokedView{})
	case authErr != nil:
		s.logger.WarnContext(ctx, "ensure valid account failed", "error", authErr)
		render(ctx, tui.ErrorView{Message: "Auth check failed. Please reconnect."})
	default:
		render(ctx, tui.LoginSuccessView{
			IsFirstAuth: result.IsFirstAuth,
			WasReauth:   result.WasReauth,
			Account:     result.Account,
		})
		render(ctx, tui.InstamartPlaceholderView{UserID: resolvedUserID, In: ch})
	}
}

// pollLoginCode polls svc every 2 s until the code leaves the pending state.
// Returns (true, nil) on completion, (false, nil) on expiry/cancellation,
// (false, err) on unexpected error. Respects ctx cancellation.
func pollLoginCode(ctx context.Context, svc auth.LoginCodeService, rawCode string, logger *slog.Logger) (completed bool, err error) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			// Treat SSH disconnect / server shutdown the same as expiry at the
			// caller level — avoids "polling error" message on clean disconnect.
			return false, nil
		case <-ticker.C:
			record, getErr := svc.GetLoginCode(ctx, rawCode)
			if errors.Is(getErr, auth.ErrLoginCodeNotFound) {
				return false, nil // expired
			}
			if getErr != nil {
				logger.WarnContext(ctx, "poll login code error", "error", getErr)
				return false, getErr
			}
			switch record.Status {
			case auth.LoginCodeStatusCompleted:
				return true, nil
			case auth.LoginCodeStatusCancelled:
				return false, nil
				// LoginCodeStatusPending: keep polling
			}
		}
	}
}
