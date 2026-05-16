package ssh

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
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

func TestServerConfigAcceptsNoClientKey(t *testing.T) {
	t.Parallel()

	config := newServerConfig()
	config.AddHostKey(newTestSigner(t))

	listener := newTestListener(t)
	defer listener.Close()

	serverDone := make(chan error, 1)
	go func() {
		serverConn, err := listener.Accept()
		if err != nil {
			serverDone <- err
			return
		}
		defer serverConn.Close()
		conn, _, _, err := ssh.NewServerConn(serverConn, config)
		if err == nil {
			_ = conn.Close()
		}
		serverDone <- err
	}()

	clientConfig := &ssh.ClientConfig{
		User: "guest",
		Auth: []ssh.AuthMethod{ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
			return []string{}, nil
		})},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	client, _, _, err := ssh.NewClientConn(clientConn, "test", clientConfig)
	if err != nil {
		t.Fatalf("no-key client handshake: %v", err)
	}
	_ = client.Close()

	if err := <-serverDone; err != nil {
		t.Fatalf("server handshake: %v", err)
	}
}

func TestServerConfigPreservesProvidedClientKeyMetadata(t *testing.T) {
	t.Parallel()

	config := newServerConfig()
	config.AddHostKey(newTestSigner(t))
	clientSigner := newTestSigner(t)
	listener := newTestListener(t)
	defer listener.Close()

	serverDone := make(chan *ssh.Permissions, 1)
	serverErrs := make(chan error, 1)
	go func() {
		serverConn, err := listener.Accept()
		if err != nil {
			serverErrs <- err
			return
		}
		defer serverConn.Close()
		conn, _, _, err := ssh.NewServerConn(serverConn, config)
		if err != nil {
			serverErrs <- err
			return
		}
		serverDone <- conn.Permissions
		_ = conn.Close()
	}()

	clientConfig := &ssh.ClientConfig{
		User:            "known",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	clientConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	client, _, _, err := ssh.NewClientConn(clientConn, "test", clientConfig)
	if err != nil {
		t.Fatalf("public-key client handshake: %v", err)
	}
	_ = client.Close()

	select {
	case err := <-serverErrs:
		t.Fatalf("server handshake: %v", err)
	case permissions := <-serverDone:
		if permissions == nil {
			t.Fatal("expected public key permissions")
		}
		gotFP := permissions.Extensions["pubkey_fingerprint"]
		wantFP := ssh.FingerprintSHA256(clientSigner.PublicKey())
		if gotFP != wantFP {
			t.Fatalf("unexpected fingerprint: got %s want %s", gotFP, wantFP)
		}
	}
}

func TestServerConfigAllowsNoClientKey(t *testing.T) {
	t.Parallel()

	config := newServerConfig()
	if config == nil {
		t.Fatal("expected server config")
	}
	if config.PublicKeyCallback == nil {
		t.Fatal("server config must preserve public key metadata when a key is provided")
	}
	if config.KeyboardInteractiveCallback == nil {
		t.Fatal("server config must allow SSH clients without keys")
	}
}

func newTestSigner(t *testing.T) ssh.Signer {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key failed: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatalf("new signer failed: %v", err)
	}
	return signer
}

func newTestListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return listener
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
