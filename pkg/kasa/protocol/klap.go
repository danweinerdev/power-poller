package protocol

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

const (
	// KLAPPort is the HTTP port for KLAP devices.
	KLAPPort = 80

	// DefaultKLAPUsername is used for unprovisioned devices.
	DefaultKLAPUsername = "kasa@tp-link.net"

	// DefaultKLAPPassword is used for unprovisioned devices.
	DefaultKLAPPassword = ""
)

// Credentials holds authentication credentials for KLAP devices.
type Credentials struct {
	Username string
	Password string
}

// DefaultCredentials returns the default credentials for unprovisioned devices.
func DefaultCredentials() *Credentials {
	return &Credentials{
		Username: DefaultKLAPUsername,
		Password: DefaultKLAPPassword,
	}
}

// KLAPTransport handles HTTP/KLAP communication with newer KASA devices.
type KLAPTransport struct {
	host         string
	port         int
	timeout      time.Duration
	retries      int
	retryBackoff time.Duration
	credentials  *Credentials
	logger       *slog.Logger

	mu         sync.Mutex
	client     *http.Client
	localHash  []byte // SHA256(SHA1(username) + SHA1(password))
	sessionID  string
	seq        int32
	encKey     []byte // AES encryption key
	decKey     []byte // AES decryption key
	sig        []byte // Signature prefix for IV
	connected  bool
}

// KLAPOption configures a KLAPTransport.
type KLAPOption func(*KLAPTransport)

// WithKLAPPort sets a custom port.
func WithKLAPPort(port int) KLAPOption {
	return func(t *KLAPTransport) {
		t.port = port
	}
}

// WithKLAPTimeout sets the HTTP timeout.
func WithKLAPTimeout(d time.Duration) KLAPOption {
	return func(t *KLAPTransport) {
		t.timeout = d
	}
}

// WithCredentials sets the authentication credentials.
func WithCredentials(creds *Credentials) KLAPOption {
	return func(t *KLAPTransport) {
		if creds != nil {
			t.credentials = creds
		}
	}
}

// WithKLAPLogger sets a custom logger.
func WithKLAPLogger(logger *slog.Logger) KLAPOption {
	return func(t *KLAPTransport) {
		t.logger = logger
	}
}

// WithKLAPRetries sets the number of retry attempts for failed connections.
// Set to 0 to disable retries.
func WithKLAPRetries(n int) KLAPOption {
	return func(t *KLAPTransport) {
		t.retries = n
	}
}

// WithKLAPRetryBackoff sets the initial backoff duration for retries.
// Each subsequent retry doubles this duration (exponential backoff).
func WithKLAPRetryBackoff(d time.Duration) KLAPOption {
	return func(t *KLAPTransport) {
		t.retryBackoff = d
	}
}

// NewKLAPTransport creates a new KLAP transport for the given host.
func NewKLAPTransport(host string, opts ...KLAPOption) *KLAPTransport {
	t := &KLAPTransport{
		host:         host,
		port:         KLAPPort,
		timeout:      DefaultTimeout,
		retries:      DefaultRetries,
		retryBackoff: DefaultRetryBackoff,
		credentials:  DefaultCredentials(),
		logger:       slog.Default(),
	}
	for _, opt := range opts {
		opt(t)
	}

	// Compute local hash from credentials
	t.localHash = computeLocalHash(t.credentials.Username, t.credentials.Password)

	return t
}

// Host returns the device host address.
func (t *KLAPTransport) Host() string {
	return t.host
}

// Connect performs the KLAP handshake to establish an encrypted session.
func (t *KLAPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	// Create HTTP client with cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	t.client = &http.Client{
		Timeout: t.timeout,
		Jar:     jar,
		// Wrap transport to provide device context for HTTP-level logging
		Transport: newLoggingRoundTripper(http.DefaultTransport, t.host, t.logger),
	}

	// Perform handshake
	if err := t.handshake(ctx); err != nil {
		return err
	}

	t.connected = true
	return nil
}

// Close closes the HTTP client.
func (t *KLAPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.connected = false
	t.client = nil
	t.sessionID = ""
	t.seq = 0
	t.encKey = nil
	t.decKey = nil
	t.sig = nil

	return nil
}

// IsConnected returns true if a session is established.
func (t *KLAPTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// Send sends a command and receives the response.
func (t *KLAPTransport) Send(ctx context.Context, cmd interface{}) ([]byte, error) {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	return t.SendJSON(ctx, payload)
}

// SendJSON sends a raw JSON command and receives the response.
// It implements exponential backoff retries for connection failures.
func (t *KLAPTransport) SendJSON(ctx context.Context, jsonCmd []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil, fmt.Errorf("not connected")
	}

	var lastErr error
	backoff := t.retryBackoff

	// Attempt initial send plus configured retries
	maxAttempts := 1 + t.retries
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check context before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Wait before retry (skip on first attempt)
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}

			// Exponential backoff with cap
			backoff *= 2
			if backoff > MaxRetryBackoff {
				backoff = MaxRetryBackoff
			}

			// Reconnect if needed
			if !t.connected {
				if err := t.handshake(ctx); err != nil {
					lastErr = fmt.Errorf("reconnect failed (attempt %d/%d): %w", attempt+1, maxAttempts, err)
					continue
				}
				t.connected = true
			}
		}

		resp, err := t.sendRequestLocked(ctx, jsonCmd)
		if err != nil {
			lastErr = err
			continue
		}

		// Handle session expiration (403)
		if resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			t.connected = false

			// Try to reconnect and retry within this attempt
			if err := t.handshake(ctx); err != nil {
				lastErr = fmt.Errorf("session expired, reconnect failed: %w", err)
				continue
			}
			t.connected = true

			// Retry the request immediately after handshake
			resp, err = t.sendRequestLocked(ctx, jsonCmd)
			if err != nil {
				lastErr = err
				continue
			}

			if resp.StatusCode == http.StatusForbidden {
				resp.Body.Close()
				t.connected = false
				lastErr = fmt.Errorf("session expired after reconnect (403)")
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			t.connected = false
			lastErr = fmt.Errorf("unexpected status: %d", resp.StatusCode)
			continue
		}

		// Read and decrypt response
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		decrypted, err := t.decrypt(body)
		if err != nil {
			lastErr = fmt.Errorf("failed to decrypt: %w", err)
			continue
		}

		return decrypted, nil
	}

	return nil, fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// sendRequestLocked sends an encrypted request. Must be called with mu held.
func (t *KLAPTransport) sendRequestLocked(ctx context.Context, jsonCmd []byte) (*http.Response, error) {
	// Encrypt the payload
	encrypted, err := t.encrypt(jsonCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	// Send request
	url := fmt.Sprintf("http://%s:%d/app/request?seq=%d", t.host, t.port, t.seq)
	t.seq++

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(encrypted))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		t.connected = false
		return nil, fmt.Errorf("request failed: %w", err)
	}

	return resp, nil
}

// handshake performs the two-phase KLAP handshake.
func (t *KLAPTransport) handshake(ctx context.Context) error {
	// Generate client seed
	clientSeed := make([]byte, 16)
	if _, err := rand.Read(clientSeed); err != nil {
		return fmt.Errorf("failed to generate client seed: %w", err)
	}

	// Phase 1: Exchange seeds
	serverSeed, serverHash, err := t.handshake1(ctx, clientSeed)
	if err != nil {
		return fmt.Errorf("handshake1 failed: %w", err)
	}

	// Verify server hash: SHA256(localHash + serverSeed + clientSeed)
	expectedHash := computeHash(t.localHash, serverSeed, clientSeed)
	if !bytes.Equal(serverHash, expectedHash) {
		return fmt.Errorf("server hash mismatch - invalid credentials or unsupported device")
	}

	// Phase 2: Send auth hash: SHA256(localHash + clientSeed + serverSeed)
	authHash := computeHash(t.localHash, clientSeed, serverSeed)
	if err := t.handshake2(ctx, authHash); err != nil {
		return fmt.Errorf("handshake2 failed: %w", err)
	}

	// Derive encryption keys
	t.deriveKeys(clientSeed, serverSeed)

	return nil
}

// handshake1 performs the first phase of the KLAP handshake.
func (t *KLAPTransport) handshake1(ctx context.Context, clientSeed []byte) (serverSeed, serverHash []byte, err error) {
	url := fmt.Sprintf("http://%s:%d/app/handshake1", t.host, t.port)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(clientSeed))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, fmt.Errorf("handshake1 returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	// Response format: 16 bytes server_seed + 32 bytes server_hash
	if len(body) < 48 {
		return nil, nil, fmt.Errorf("handshake1 response too short: %d bytes", len(body))
	}

	serverSeed = body[:16]
	serverHash = body[16:48]

	return serverSeed, serverHash, nil
}

// handshake2 performs the second phase of the KLAP handshake.
func (t *KLAPTransport) handshake2(ctx context.Context, authHash []byte) error {
	url := fmt.Sprintf("http://%s:%d/app/handshake2", t.host, t.port)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(authHash))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("handshake2 returned status %d", resp.StatusCode)
	}

	return nil
}

// deriveKeys derives the encryption/decryption keys from the seeds.
func (t *KLAPTransport) deriveKeys(clientSeed, serverSeed []byte) {
	// Key derivation uses SHA256 with different prefixes
	// Encryption key: SHA256("lsk" + localHash + clientSeed + serverSeed)[:16]
	// Decryption key: SHA256("lsk" + localHash + serverSeed + clientSeed)[:16]
	// Signature: SHA256("iv" + localHash + clientSeed + serverSeed)[:12]

	encData := append([]byte("lsk"), t.localHash...)
	encData = append(encData, clientSeed...)
	encData = append(encData, serverSeed...)
	encHash := sha256.Sum256(encData)
	t.encKey = encHash[:16]

	decData := append([]byte("lsk"), t.localHash...)
	decData = append(decData, serverSeed...)
	decData = append(decData, clientSeed...)
	decHash := sha256.Sum256(decData)
	t.decKey = decHash[:16]

	sigData := append([]byte("iv"), t.localHash...)
	sigData = append(sigData, clientSeed...)
	sigData = append(sigData, serverSeed...)
	sigHash := sha256.Sum256(sigData)
	t.sig = sigHash[:12]
}

// encrypt encrypts data using AES-128-CBC.
func (t *KLAPTransport) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(t.encKey)
	if err != nil {
		return nil, err
	}

	// Build IV: sig (12 bytes) + seq (4 bytes big-endian)
	iv := make([]byte, 16)
	copy(iv, t.sig)
	binary.BigEndian.PutUint32(iv[12:], uint32(t.seq))

	// Pad plaintext to AES block size (PKCS7)
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	// Encrypt
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	return ciphertext, nil
}

// decrypt decrypts data using AES-128-CBC.
func (t *KLAPTransport) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid ciphertext length: %d", len(ciphertext))
	}

	block, err := aes.NewCipher(t.decKey)
	if err != nil {
		return nil, err
	}

	// Build IV: sig (12 bytes) + (seq-1) (4 bytes big-endian)
	// Response uses the same seq as the request
	iv := make([]byte, 16)
	copy(iv, t.sig)
	binary.BigEndian.PutUint32(iv[12:], uint32(t.seq-1))

	// Decrypt
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	if len(plaintext) == 0 {
		return plaintext, nil
	}
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen > len(plaintext) {
		return nil, fmt.Errorf("invalid padding")
	}
	// Verify padding
	for i := len(plaintext) - padLen; i < len(plaintext); i++ {
		if plaintext[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}

	return plaintext[:len(plaintext)-padLen], nil
}

// computeLocalHash computes SHA256(SHA1(username) + SHA1(password)).
func computeLocalHash(username, password string) []byte {
	userHash := sha1.Sum([]byte(username))
	passHash := sha1.Sum([]byte(password))

	combined := append(userHash[:], passHash[:]...)
	result := sha256.Sum256(combined)
	return result[:]
}

// computeHash computes SHA256(localHash + seed1 + seed2).
func computeHash(localHash, seed1, seed2 []byte) []byte {
	data := append(localHash, seed1...)
	data = append(data, seed2...)
	result := sha256.Sum256(data)
	return result[:]
}

// Ensure KLAPTransport implements Transporter.
var _ Transporter = (*KLAPTransport)(nil)
