package protocol

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestNewKLAPTransport(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		opts        []KLAPOption
		wantPort    int
		wantTimeout time.Duration
		wantUser    string
	}{
		{
			name:        "default options",
			host:        "192.168.1.100",
			wantPort:    KLAPPort,
			wantTimeout: DefaultTimeout,
			wantUser:    DefaultKLAPUsername,
		},
		{
			name:        "custom port",
			host:        "192.168.1.100",
			opts:        []KLAPOption{WithKLAPPort(8080)},
			wantPort:    8080,
			wantTimeout: DefaultTimeout,
			wantUser:    DefaultKLAPUsername,
		},
		{
			name:        "custom timeout",
			host:        "192.168.1.100",
			opts:        []KLAPOption{WithKLAPTimeout(30 * time.Second)},
			wantPort:    KLAPPort,
			wantTimeout: 30 * time.Second,
			wantUser:    DefaultKLAPUsername,
		},
		{
			name: "custom credentials",
			host: "192.168.1.100",
			opts: []KLAPOption{WithCredentials(&Credentials{
				Username: "test@example.com",
				Password: "secret",
			})},
			wantPort:    KLAPPort,
			wantTimeout: DefaultTimeout,
			wantUser:    "test@example.com",
		},
		{
			name:        "nil credentials uses defaults",
			host:        "192.168.1.100",
			opts:        []KLAPOption{WithCredentials(nil)},
			wantPort:    KLAPPort,
			wantTimeout: DefaultTimeout,
			wantUser:    DefaultKLAPUsername,
		},
		{
			name: "multiple options",
			host: "192.168.1.100",
			opts: []KLAPOption{
				WithKLAPPort(9000),
				WithKLAPTimeout(60 * time.Second),
				WithCredentials(&Credentials{Username: "user", Password: "pass"}),
			},
			wantPort:    9000,
			wantTimeout: 60 * time.Second,
			wantUser:    "user",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewKLAPTransport(tc.host, tc.opts...)

			if tr.Host() != tc.host {
				t.Errorf("Host() = %q, want %q", tr.Host(), tc.host)
			}
			if tr.port != tc.wantPort {
				t.Errorf("port = %d, want %d", tr.port, tc.wantPort)
			}
			if tr.timeout != tc.wantTimeout {
				t.Errorf("timeout = %v, want %v", tr.timeout, tc.wantTimeout)
			}
			if tr.credentials.Username != tc.wantUser {
				t.Errorf("username = %q, want %q", tr.credentials.Username, tc.wantUser)
			}
			if tr.localHash == nil {
				t.Error("localHash should be computed")
			}
		})
	}
}

func TestDefaultCredentials(t *testing.T) {
	creds := DefaultCredentials()
	if creds.Username != DefaultKLAPUsername {
		t.Errorf("Username = %q, want %q", creds.Username, DefaultKLAPUsername)
	}
	if creds.Password != DefaultKLAPPassword {
		t.Errorf("Password = %q, want %q", creds.Password, DefaultKLAPPassword)
	}
}

func TestComputeLocalHash(t *testing.T) {
	// Test with known values
	username := "kasa@tp-link.net"
	password := ""

	hash := computeLocalHash(username, password)

	// Verify hash length is SHA256 output (32 bytes)
	if len(hash) != 32 {
		t.Errorf("hash length = %d, want 32", len(hash))
	}

	// Compute expected hash manually
	userHash := sha1.Sum([]byte(username))
	passHash := sha1.Sum([]byte(password))
	combined := append(userHash[:], passHash[:]...)
	expectedHash := sha256.Sum256(combined)

	if !bytes.Equal(hash, expectedHash[:]) {
		t.Errorf("hash mismatch: got %x, want %x", hash, expectedHash)
	}
}

func TestComputeHash(t *testing.T) {
	localHash := make([]byte, 32)
	for i := range localHash {
		localHash[i] = byte(i)
	}

	seed1 := make([]byte, 16)
	seed2 := make([]byte, 16)
	for i := range seed1 {
		seed1[i] = byte(i + 100)
		seed2[i] = byte(i + 200)
	}

	hash := computeHash(localHash, seed1, seed2)

	// Verify hash length
	if len(hash) != 32 {
		t.Errorf("hash length = %d, want 32", len(hash))
	}

	// Compute expected hash
	data := append(localHash, seed1...)
	data = append(data, seed2...)
	expected := sha256.Sum256(data)

	if !bytes.Equal(hash, expected[:]) {
		t.Errorf("hash mismatch")
	}
}

func TestKLAPTransport_IsConnected(t *testing.T) {
	tr := NewKLAPTransport("192.168.1.100")

	if tr.IsConnected() {
		t.Error("should not be connected initially")
	}
}

func TestKLAPTransport_Close(t *testing.T) {
	tr := NewKLAPTransport("192.168.1.100")

	// Close without connect should be no-op
	if err := tr.Close(); err != nil {
		t.Errorf("Close() on unconnected transport error = %v", err)
	}

	if tr.IsConnected() {
		t.Error("should not be connected after Close()")
	}
}

func TestKLAPTransport_SendJSON_NotConnected(t *testing.T) {
	tr := NewKLAPTransport("192.168.1.100")

	_, err := tr.SendJSON(context.Background(), []byte(`{}`))
	if err == nil {
		t.Error("expected error when not connected, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error should mention 'not connected', got: %v", err)
	}
}

func TestKLAPTransport_Send_NotConnected(t *testing.T) {
	tr := NewKLAPTransport("192.168.1.100")

	_, err := tr.Send(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error when not connected, got nil")
	}
}

// mockKLAPServer creates a test server that simulates KLAP handshake
type mockKLAPServer struct {
	server     *httptest.Server
	localHash  []byte
	clientSeed []byte
	serverSeed []byte
	encKey     []byte
	decKey     []byte
	sig        []byte
	seq        int32
	failHS1    bool
	failHS2    bool
	fail403    bool
	fail403Once bool
	requestCount int
}

func newMockKLAPServer(username, password string) *mockKLAPServer {
	m := &mockKLAPServer{
		localHash:  computeLocalHash(username, password),
		serverSeed: make([]byte, 16),
	}
	// Use fixed server seed for testing
	for i := range m.serverSeed {
		m.serverSeed[i] = byte(i + 50)
	}
	return m
}

func (m *mockKLAPServer) start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/app/handshake1", m.handleHandshake1)
	mux.HandleFunc("/app/handshake2", m.handleHandshake2)
	mux.HandleFunc("/app/request", m.handleRequest)
	m.server = httptest.NewServer(mux)
}

func (m *mockKLAPServer) stop() {
	if m.server != nil {
		m.server.Close()
	}
}

func (m *mockKLAPServer) url() string {
	return m.server.URL
}

func (m *mockKLAPServer) handleHandshake1(w http.ResponseWriter, r *http.Request) {
	if m.failHS1 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	clientSeed, err := io.ReadAll(r.Body)
	if err != nil || len(clientSeed) != 16 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	m.clientSeed = clientSeed

	// Compute server hash: SHA256(localHash + serverSeed + clientSeed)
	serverHash := computeHash(m.localHash, m.serverSeed, clientSeed)

	// Derive keys for later request handling
	m.deriveKeys()

	// Response: serverSeed (16) + serverHash (32)
	resp := append(m.serverSeed, serverHash...)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(resp)
}

func (m *mockKLAPServer) handleHandshake2(w http.ResponseWriter, r *http.Request) {
	if m.failHS2 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	authHash, err := io.ReadAll(r.Body)
	if err != nil || len(authHash) != 32 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Verify auth hash: SHA256(localHash + clientSeed + serverSeed)
	expectedHash := computeHash(m.localHash, m.clientSeed, m.serverSeed)
	if !bytes.Equal(authHash, expectedHash) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (m *mockKLAPServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	m.requestCount++

	if m.fail403 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	if m.fail403Once && m.requestCount == 1 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// Read encrypted request
	ciphertext, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Decrypt (we need to decrypt with our encKey since client encrypts with their encKey which equals our decKey)
	plaintext, err := m.decryptRequest(ciphertext)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Create a simple response
	response := []byte(`{"system":{"get_sysinfo":{"err_code":0}}}`)
	_ = plaintext // Would process the command in a real implementation

	// Encrypt response
	encrypted, err := m.encryptResponse(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	m.seq++
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(encrypted)
}

func (m *mockKLAPServer) deriveKeys() {
	// Server's decrypt key = client's encrypt key
	encData := append([]byte("lsk"), m.localHash...)
	encData = append(encData, m.clientSeed...)
	encData = append(encData, m.serverSeed...)
	encHash := sha256.Sum256(encData)
	m.decKey = encHash[:16] // We decrypt what client encrypts

	// Server's encrypt key = client's decrypt key
	decData := append([]byte("lsk"), m.localHash...)
	decData = append(decData, m.serverSeed...)
	decData = append(decData, m.clientSeed...)
	decHash := sha256.Sum256(decData)
	m.encKey = decHash[:16] // We encrypt for client to decrypt

	sigData := append([]byte("iv"), m.localHash...)
	sigData = append(sigData, m.clientSeed...)
	sigData = append(sigData, m.serverSeed...)
	sigHash := sha256.Sum256(sigData)
	m.sig = sigHash[:12]
}

func (m *mockKLAPServer) decryptRequest(ciphertext []byte) ([]byte, error) {
	tr := &KLAPTransport{
		decKey: m.decKey,
		sig:    m.sig,
		seq:    m.seq + 1, // Server receives at seq N, client sent at seq N
	}
	return tr.decrypt(ciphertext)
}

func (m *mockKLAPServer) encryptResponse(plaintext []byte) ([]byte, error) {
	tr := &KLAPTransport{
		encKey: m.encKey,
		sig:    m.sig,
		seq:    m.seq,
	}
	return tr.encrypt(plaintext)
}

// parseServerAddr extracts host and port from an httptest server URL
func parseServerAddr(serverURL string) (host string, port int) {
	addr := strings.TrimPrefix(serverURL, "http://")
	parts := strings.Split(addr, ":")
	host = parts[0]
	port, _ = strconv.Atoi(parts[1])
	return
}

func TestKLAPTransport_Connect(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())

	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	t.Run("successful connection", func(t *testing.T) {
		if tr.IsConnected() {
			t.Error("should not be connected before Connect()")
		}

		if err := tr.Connect(ctx); err != nil {
			t.Errorf("Connect() error = %v", err)
		}
		defer tr.Close()

		if !tr.IsConnected() {
			t.Error("should be connected after Connect()")
		}
	})
}

func TestKLAPTransport_Connect_AlreadyConnected(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}
	defer tr.Close()

	// Second connect should be a no-op
	if err := tr.Connect(ctx); err != nil {
		t.Errorf("second Connect() error = %v", err)
	}
}

func TestKLAPTransport_Connect_Handshake1Failure(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.failHS1 = true
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	err := tr.Connect(ctx)
	if err == nil {
		tr.Close()
		t.Error("expected error for handshake1 failure, got nil")
	}
	if !strings.Contains(err.Error(), "handshake1") {
		t.Errorf("error should mention handshake1, got: %v", err)
	}
}

func TestKLAPTransport_Connect_Handshake2Failure(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.failHS2 = true
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	err := tr.Connect(ctx)
	if err == nil {
		tr.Close()
		t.Error("expected error for handshake2 failure, got nil")
	}
	if !strings.Contains(err.Error(), "handshake2") {
		t.Errorf("error should mention handshake2, got: %v", err)
	}
}

func TestKLAPTransport_Connect_InvalidCredentials(t *testing.T) {
	// Server expects default credentials
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()

	// Client uses wrong credentials
	tr := NewKLAPTransport(host, WithKLAPPort(port), WithCredentials(&Credentials{
		Username: "wrong@example.com",
		Password: "wrongpass",
	}))

	err := tr.Connect(ctx)
	if err == nil {
		tr.Close()
		t.Error("expected error for invalid credentials, got nil")
	}
	if !strings.Contains(err.Error(), "hash mismatch") && !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("error should mention credential issue, got: %v", err)
	}
}

func TestKLAPTransport_SendJSON(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	jsonCmd := []byte(`{"system":{"get_sysinfo":{}}}`)

	resp, err := tr.SendJSON(ctx, jsonCmd)
	if err != nil {
		t.Fatalf("SendJSON() error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestKLAPTransport_Send(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	resp, err := tr.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestKLAPTransport_SessionExpiredReconnect(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.fail403Once = true // First request returns 403, then succeeds
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	// This should trigger 403, reconnect, and retry
	jsonCmd := []byte(`{"system":{"get_sysinfo":{}}}`)

	resp, err := tr.SendJSON(ctx, jsonCmd)
	if err != nil {
		t.Fatalf("SendJSON() after session expiry should auto-reconnect, got error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("expected non-empty response after reconnect")
	}
}

func TestKLAPTransport_SessionExpiredPermanent(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	// Now set permanent 403
	mock.fail403 = true

	jsonCmd := []byte(`{"system":{"get_sysinfo":{}}}`)

	_, err := tr.SendJSON(ctx, jsonCmd)
	if err == nil {
		t.Error("expected error when server permanently returns 403")
	}
}

func TestKLAPTransport_Reconnection(t *testing.T) {
	mock := newMockKLAPServer(DefaultKLAPUsername, DefaultKLAPPassword)
	mock.start()
	defer mock.stop()

	host, port := parseServerAddr(mock.url())
	ctx := context.Background()
	tr := NewKLAPTransport(host, WithKLAPPort(port))

	// First connection
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}

	jsonCmd := []byte(`{"system":{"get_sysinfo":{}}}`)

	if _, err := tr.SendJSON(ctx, jsonCmd); err != nil {
		t.Errorf("first SendJSON() error = %v", err)
	}

	// Close connection
	if err := tr.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Reset mock state for new handshake
	mock.seq = 0
	mock.clientSeed = nil

	// Reconnect
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	defer tr.Close()

	if _, err := tr.SendJSON(ctx, jsonCmd); err != nil {
		t.Errorf("second SendJSON() after reconnect error = %v", err)
	}
}

func TestKLAPTransport_EncryptDecrypt(t *testing.T) {
	// Test encryption/decryption roundtrip
	tr := &KLAPTransport{
		encKey: make([]byte, 16),
		decKey: make([]byte, 16),
		sig:    make([]byte, 12),
		seq:    1,
	}
	// Use same key for enc/dec in this test
	for i := range tr.encKey {
		tr.encKey[i] = byte(i)
		tr.decKey[i] = byte(i)
	}
	for i := range tr.sig {
		tr.sig[i] = byte(i + 100)
	}

	testCases := []string{
		`{"test": "data"}`,
		`{"system":{"get_sysinfo":{}}}`,
		`short`,
		`{"longer":"payload with more data to test block alignment properly across multiple blocks"}`,
	}

	for _, original := range testCases {
		t.Run(original[:min(10, len(original))], func(t *testing.T) {
			encrypted, err := tr.encrypt([]byte(original))
			if err != nil {
				t.Fatalf("encrypt() error = %v", err)
			}

			// Increment seq to match decrypt expectation
			tr.seq++

			decrypted, err := tr.decrypt(encrypted)
			if err != nil {
				t.Fatalf("decrypt() error = %v", err)
			}

			if string(decrypted) != original {
				t.Errorf("roundtrip failed: got %q, want %q", decrypted, original)
			}
		})
	}
}

func TestKLAPTransport_DecryptInvalidCiphertext(t *testing.T) {
	tr := &KLAPTransport{
		decKey: make([]byte, 16),
		sig:    make([]byte, 12),
		seq:    1,
	}

	tests := []struct {
		name       string
		ciphertext []byte
		wantErr    string
	}{
		{
			name:       "empty ciphertext",
			ciphertext: []byte{},
			wantErr:    "invalid ciphertext length",
		},
		{
			name:       "wrong block size",
			ciphertext: []byte{1, 2, 3, 4, 5},
			wantErr:    "invalid ciphertext length",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tr.decrypt(tc.ciphertext)
			if err == nil {
				t.Error("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error should contain %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestKLAPTransport_Connect_Timeout(t *testing.T) {
	// Use a non-routable IP to force timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tr := NewKLAPTransport("10.255.255.1", WithKLAPTimeout(100*time.Millisecond))

	err := tr.Connect(ctx)
	if err == nil {
		tr.Close()
		t.Error("expected timeout error, got nil")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
