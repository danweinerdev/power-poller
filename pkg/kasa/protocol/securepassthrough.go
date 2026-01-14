package protocol

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

const (
	// SecurePassthroughPort is the HTTP port for SecurePassthrough devices.
	SecurePassthroughPort = 80

	// RSAKeySize is the key size for the handshake.
	RSAKeySize = 1024
)

// SecurePassthroughTransport handles HTTP/SecurePassthrough communication with TAPO devices.
type SecurePassthroughTransport struct {
	host    string
	port    int
	timeout time.Duration

	mu         sync.Mutex
	client     *http.Client
	privateKey *rsa.PrivateKey
	aesKey     []byte
	aesIV      []byte
	token      string
	connected  bool
}

// SecurePassthroughOption configures a SecurePassthroughTransport.
type SecurePassthroughOption func(*SecurePassthroughTransport)

// WithSecurePassthroughPort sets a custom port.
func WithSecurePassthroughPort(port int) SecurePassthroughOption {
	return func(t *SecurePassthroughTransport) {
		t.port = port
	}
}

// WithSecurePassthroughTimeout sets the HTTP timeout.
func WithSecurePassthroughTimeout(d time.Duration) SecurePassthroughOption {
	return func(t *SecurePassthroughTransport) {
		t.timeout = d
	}
}

// NewSecurePassthroughTransport creates a new SecurePassthrough transport for the given host.
func NewSecurePassthroughTransport(host string, opts ...SecurePassthroughOption) *SecurePassthroughTransport {
	t := &SecurePassthroughTransport{
		host:    host,
		port:    SecurePassthroughPort,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Host returns the device host address.
func (t *SecurePassthroughTransport) Host() string {
	return t.host
}

// Connect performs the SecurePassthrough handshake to establish an encrypted session.
func (t *SecurePassthroughTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.connected {
		return nil
	}

	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, RSAKeySize)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}
	t.privateKey = privateKey

	// Create HTTP client with cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return fmt.Errorf("failed to create cookie jar: %w", err)
	}

	// Custom transport to prevent "unsolicited response" warnings
	// TAPO devices send HTML on idle connections which triggers Go's HTTP client warnings
	t.client = &http.Client{
		Timeout: t.timeout,
		Jar:     jar,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			// Use a custom dialer that sets SO_LINGER to 0 for immediate RST on close
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				d := &net.Dialer{Timeout: t.timeout}
				conn, err := d.DialContext(ctx, network, addr)
				if err != nil {
					return nil, err
				}
				// Set SO_LINGER to 0 to send RST instead of FIN on close
				// This prevents the device from sending data after we're done
				if tcpConn, ok := conn.(*net.TCPConn); ok {
					tcpConn.SetLinger(0)
				}
				return conn, nil
			},
		},
	}

	// Perform handshake
	if err := t.handshake(ctx); err != nil {
		return err
	}

	t.connected = true
	return nil
}

// Close closes the HTTP client.
func (t *SecurePassthroughTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Close idle connections to prevent "unsolicited response" warnings
	if t.client != nil {
		if transport, ok := t.client.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	t.connected = false
	t.client = nil
	t.privateKey = nil
	t.aesKey = nil
	t.aesIV = nil
	t.token = ""

	return nil
}

// IsConnected returns true if a session is established.
func (t *SecurePassthroughTransport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.connected
}

// Send sends a command and receives the response.
func (t *SecurePassthroughTransport) Send(ctx context.Context, cmd interface{}) ([]byte, error) {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}
	return t.SendJSON(ctx, payload)
}

// SendJSON sends a raw JSON command and receives the response.
func (t *SecurePassthroughTransport) SendJSON(ctx context.Context, jsonCmd []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.connected {
		return nil, fmt.Errorf("not connected")
	}

	// Encrypt the payload
	encrypted, err := t.encrypt(jsonCmd)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt: %w", err)
	}

	// Wrap in securePassthrough request
	request := map[string]interface{}{
		"method": "securePassthrough",
		"params": map[string]interface{}{
			"request": base64.StdEncoding.EncodeToString(encrypted),
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request
	url := fmt.Sprintf("http://%s:%d/app?token=%s", t.host, t.port, t.token)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true // Force connection close after response

	resp, err := t.client.Do(req)
	if err != nil {
		t.connected = false
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.connected = false
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response
	var response struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			Response string `json:"response"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if response.ErrorCode != 0 {
		// Session might have expired
		if response.ErrorCode == -1010 || response.ErrorCode == 9999 {
			t.connected = false
			return nil, fmt.Errorf("session expired (error %d)", response.ErrorCode)
		}
		return nil, fmt.Errorf("device error: %d", response.ErrorCode)
	}

	// Decode and decrypt response
	encryptedResp, err := base64.StdEncoding.DecodeString(response.Result.Response)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	decrypted, err := t.decrypt(encryptedResp)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt response: %w", err)
	}

	return decrypted, nil
}

// handshake performs the SecurePassthrough handshake and login.
func (t *SecurePassthroughTransport) handshake(ctx context.Context) error {
	// Export public key as PEM
	pubKeyDER, err := x509.MarshalPKIXPublicKey(&t.privateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}

	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubKeyDER,
	})

	// Send handshake request
	request := map[string]interface{}{
		"method": "handshake",
		"params": map[string]interface{}{
			"key": string(pubKeyPEM),
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal handshake: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d/app", t.host, t.port)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true // Force connection close after response

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("handshake request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("handshake returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read handshake response: %w", err)
	}

	// Parse handshake response
	var response struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			Key string `json:"key"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("failed to parse handshake response: %w (body: %s)", err, string(body))
	}

	if response.ErrorCode != 0 {
		return fmt.Errorf("handshake error: %d", response.ErrorCode)
	}

	// Decode the encrypted key
	encryptedKey, err := base64.StdEncoding.DecodeString(response.Result.Key)
	if err != nil {
		return fmt.Errorf("failed to decode key: %w", err)
	}

	// Decrypt with our private key to get AES key + IV
	decryptedKey, err := rsa.DecryptPKCS1v15(rand.Reader, t.privateKey, encryptedKey)
	if err != nil {
		return fmt.Errorf("failed to decrypt key: %w", err)
	}

	// The decrypted data contains: AES key (16 bytes) + IV (16 bytes)
	if len(decryptedKey) < 32 {
		return fmt.Errorf("decrypted key too short: %d bytes", len(decryptedKey))
	}

	t.aesKey = decryptedKey[:16]
	t.aesIV = decryptedKey[16:32]

	// Perform login to get token
	if err := t.login(ctx); err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	return nil
}

// login performs the TAPO login to get a session token.
func (t *SecurePassthroughTransport) login(ctx context.Context) error {
	// Use default TAPO credentials
	username := "test@tp-link.net"
	password := "test"

	// Hash credentials: username is SHA1(username) as hex, then base64-encoded
	// Password is just base64-encoded (login v1) or SHA1+base64 (login v2)
	usernameHash := sha1.Sum([]byte(username))
	usernameHex := fmt.Sprintf("%x", usernameHash)
	usernameB64 := base64.StdEncoding.EncodeToString([]byte(usernameHex))
	passwordB64 := base64.StdEncoding.EncodeToString([]byte(password))

	// Create login request
	loginParams := map[string]interface{}{
		"username": usernameB64,
		"password": passwordB64,
	}

	loginReq := map[string]interface{}{
		"method": "login_device",
		"params": loginParams,
	}

	loginJSON, err := json.Marshal(loginReq)
	if err != nil {
		return fmt.Errorf("failed to marshal login: %w", err)
	}

	// Encrypt the login request
	encrypted, err := t.encrypt(loginJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt login: %w", err)
	}

	// Wrap in securePassthrough
	request := map[string]interface{}{
		"method": "securePassthrough",
		"params": map[string]interface{}{
			"request": base64.StdEncoding.EncodeToString(encrypted),
		},
	}

	reqBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("http://%s:%d/app", t.host, t.port)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Close = true // Force connection close after response

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read login response: %w", err)
	}

	// Parse outer response
	var outerResp struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			Response string `json:"response"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &outerResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w (body: %s)", err, string(body))
	}

	if outerResp.ErrorCode != 0 {
		return fmt.Errorf("login outer error: %d", outerResp.ErrorCode)
	}

	// Decrypt inner response
	encryptedResp, err := base64.StdEncoding.DecodeString(outerResp.Result.Response)
	if err != nil {
		return fmt.Errorf("failed to decode login response: %w", err)
	}

	decrypted, err := t.decrypt(encryptedResp)
	if err != nil {
		return fmt.Errorf("failed to decrypt login response: %w", err)
	}

	// Parse inner response to get token
	var innerResp struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			Token string `json:"token"`
		} `json:"result"`
	}

	if err := json.Unmarshal(decrypted, &innerResp); err != nil {
		return fmt.Errorf("failed to parse login inner response: %w", err)
	}

	if innerResp.ErrorCode != 0 {
		return fmt.Errorf("login error: %d", innerResp.ErrorCode)
	}

	t.token = innerResp.Result.Token
	return nil
}

// encrypt encrypts data using AES-128-CBC.
func (t *SecurePassthroughTransport) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(t.aesKey)
	if err != nil {
		return nil, err
	}

	// Pad plaintext to AES block size (PKCS7)
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	// Encrypt
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, t.aesIV)
	mode.CryptBlocks(ciphertext, padded)

	return ciphertext, nil
}

// decrypt decrypts data using AES-128-CBC.
func (t *SecurePassthroughTransport) decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid ciphertext length: %d", len(ciphertext))
	}

	block, err := aes.NewCipher(t.aesKey)
	if err != nil {
		return nil, err
	}

	// Decrypt
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, t.aesIV)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	if len(plaintext) == 0 {
		return plaintext, nil
	}
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen > len(plaintext) {
		return nil, fmt.Errorf("invalid padding length: %d", padLen)
	}
	// Verify padding
	for i := len(plaintext) - padLen; i < len(plaintext); i++ {
		if plaintext[i] != byte(padLen) {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}

	return plaintext[:len(plaintext)-padLen], nil
}

// Ensure SecurePassthroughTransport implements Transporter.
var _ Transporter = (*SecurePassthroughTransport)(nil)
