package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestPublicKeyPermissionsIncludesSafeMetadata(t *testing.T) {
	t.Parallel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}

	sshSigner, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new signer failed: %v", err)
	}

	permissions := publicKeyPermissions(sshSigner.PublicKey())
	if permissions == nil {
		t.Fatalf("permissions should not be nil")
	}

	gotType := permissions.Extensions["pubkey_type"]
	if gotType != sshSigner.PublicKey().Type() {
		t.Fatalf("unexpected key type: got %s want %s", gotType, sshSigner.PublicKey().Type())
	}

	gotFP := permissions.Extensions["pubkey_fingerprint"]
	wantFP := ssh.FingerprintSHA256(sshSigner.PublicKey())
	if gotFP != wantFP {
		t.Fatalf("unexpected fingerprint: got %s want %s", gotFP, wantFP)
	}

	if permissions.Extensions["pubkey_authorized"] == "" {
		t.Fatal("expected authorized public key extension")
	}
}

func TestParseViewportRequestFromPTY(t *testing.T) {
	t.Parallel()

	req := &ssh.Request{
		Type: "pty-req",
		Payload: ssh.Marshal(ptyRequestPayload{
			Term:          "xterm-256color",
			Columns:       120,
			Rows:          40,
			TerminalModes: "",
		}),
	}
	viewport, ok := parseViewportRequest(req)
	if !ok {
		t.Fatal("expected pty request to parse")
	}
	if viewport.Width != 120 || viewport.Height != 40 {
		t.Fatalf("unexpected viewport: got %+v", viewport)
	}
}

func TestParseViewportRequestFromWindowChange(t *testing.T) {
	t.Parallel()

	req := &ssh.Request{
		Type: "window-change",
		Payload: ssh.Marshal(windowChangePayload{
			Columns: 132,
			Rows:    48,
		}),
	}
	viewport, ok := parseViewportRequest(req)
	if !ok {
		t.Fatal("expected window-change request to parse")
	}
	if viewport.Width != 132 || viewport.Height != 48 {
		t.Fatalf("unexpected viewport: got %+v", viewport)
	}
}
