package mockdevice

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// KLAPState holds the KLAP session state.
type KLAPState struct {
	LocalSeed  []byte
	RemoteSeed []byte
	AuthHash   []byte
	Key        []byte
	IV         []byte
	Sig        []byte
	SeqNo      int32
}

// handleKLAP handles KLAP protocol HTTP requests.
func (d *MockDevice) handleKLAP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Path
	switch path {
	case "/app/handshake1":
		d.handleKLAPHandshake1(w, r)
	case "/app/handshake2":
		d.handleKLAPHandshake2(w, r)
	case "/app/request":
		d.handleKLAPRequest(w, r)
	default:
		http.Error(w, "Not found", http.StatusNotFound)
	}
}

// handleKLAPHandshake1 handles the first KLAP handshake step.
func (d *MockDevice) handleKLAPHandshake1(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) != 16 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// Client sends 16-byte local seed
	remoteSeed := body

	// Generate our local seed
	localSeed := make([]byte, 16)
	for i := range localSeed {
		localSeed[i] = byte(i + 0x10)
	}

	// Compute auth hash using default credentials: SHA256(SHA1(username) + SHA1(password))
	// These are the default KLAP credentials
	username := "test@tp-link.net"
	password := "test"
	usernameHash := sha1.Sum([]byte(username))
	passwordHash := sha1.Sum([]byte(password))
	authHash := sha256.Sum256(append(usernameHash[:], passwordHash[:]...))

	// Create server hash: SHA256(auth_hash + local_seed + remote_seed)
	// Client expects: SHA256(localHash + serverSeed + clientSeed)
	// Where localSeed = serverSeed, remoteSeed = clientSeed
	hashInput := append(authHash[:], localSeed...)
	hashInput = append(hashInput, remoteSeed...)
	serverHash := sha256.Sum256(hashInput)

	// Store state
	d.mu.Lock()
	d.klapState = &KLAPState{
		LocalSeed:  localSeed,
		RemoteSeed: remoteSeed,
		AuthHash:   authHash[:],
	}
	d.mu.Unlock()

	// Response: 16-byte local seed + 32-byte server hash
	response := append(localSeed, serverHash[:]...)

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:  "TP_SESSIONID",
		Value: "MOCK_KLAP_SESSION",
	})

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}

// handleKLAPHandshake2 handles the second KLAP handshake step.
func (d *MockDevice) handleKLAPHandshake2(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) != 32 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	d.mu.Lock()
	state := d.klapState
	d.mu.Unlock()

	if state == nil {
		http.Error(w, "No session", http.StatusForbidden)
		return
	}

	// Verify client hash: SHA256(auth_hash + remote_seed + local_seed)
	// Client sends: SHA256(localHash + clientSeed + serverSeed)
	// Where remoteSeed = clientSeed, localSeed = serverSeed
	expectedInput := append(state.AuthHash, state.RemoteSeed...)
	expectedInput = append(expectedInput, state.LocalSeed...)
	expectedHash := sha256.Sum256(expectedInput)

	if !bytes.Equal(body, expectedHash[:]) {
		http.Error(w, "Authentication failed", http.StatusForbidden)
		return
	}

	// Derive session keys - must match client's key derivation
	// Client encKey: SHA256("lsk" + localHash + clientSeed + serverSeed)[:16]
	// Client decKey: SHA256("lsk" + localHash + serverSeed + clientSeed)[:16]
	// Server needs to decrypt what client encrypts (use client's encKey as our decKey)
	// Server needs to encrypt what client decrypts (use client's decKey as our encKey)

	// Server's decryption key = client's encryption key
	decKeyInput := []byte("lsk")
	decKeyInput = append(decKeyInput, state.AuthHash...)
	decKeyInput = append(decKeyInput, state.RemoteSeed...) // clientSeed
	decKeyInput = append(decKeyInput, state.LocalSeed...)  // serverSeed
	decKeyHash := sha256.Sum256(decKeyInput)

	// Server's encryption key = client's decryption key
	encKeyInput := []byte("lsk")
	encKeyInput = append(encKeyInput, state.AuthHash...)
	encKeyInput = append(encKeyInput, state.LocalSeed...)  // serverSeed
	encKeyInput = append(encKeyInput, state.RemoteSeed...) // clientSeed
	encKeyHash := sha256.Sum256(encKeyInput)

	// Signature for IV: SHA256("iv" + localHash + clientSeed + serverSeed)[:12]
	sigInput := []byte("iv")
	sigInput = append(sigInput, state.AuthHash...)
	sigInput = append(sigInput, state.RemoteSeed...) // clientSeed
	sigInput = append(sigInput, state.LocalSeed...)  // serverSeed
	sigHash := sha256.Sum256(sigInput)

	d.mu.Lock()
	d.klapState.Key = decKeyHash[:16]    // For decrypting client requests
	d.klapState.IV = encKeyHash[:16]     // Store encKey for encrypting responses
	d.klapState.Sig = sigHash[:12]       // 12-byte signature for IV
	d.klapState.SeqNo = 0
	d.mu.Unlock()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
}

// handleKLAPRequest handles encrypted KLAP requests.
func (d *MockDevice) handleKLAPRequest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	d.mu.Lock()
	state := d.klapState
	d.mu.Unlock()

	if state == nil || state.Key == nil {
		http.Error(w, "No session", http.StatusForbidden)
		return
	}

	// Get sequence number from URL query param
	seqStr := r.URL.Query().Get("seq")
	var seqNo int32
	if seqStr != "" {
		var seq int
		fmt.Sscanf(seqStr, "%d", &seq)
		seqNo = int32(seq)
	}

	// Decrypt payload (body is just the encrypted data, no header)
	decrypted, err := d.klapDecrypt(state, seqNo, body)
	if err != nil {
		http.Error(w, "Decryption failed", http.StatusBadRequest)
		return
	}

	// Parse and handle command
	var cmd map[string]interface{}
	if err := json.Unmarshal(decrypted, &cmd); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Record command
	d.mu.Lock()
	d.ReceivedCommands = append(d.ReceivedCommands, CommandRecord{
		Timestamp: time.Now(),
		Command:   cmd,
		RawBytes:  decrypted,
	})
	d.commandCount++
	d.mu.Unlock()

	// Generate response
	resp := d.handleCommand(cmd)
	respJSON, _ := json.Marshal(resp)

	// Encrypt response using the same seq number
	// Server uses decKey (client's encKey) for decryption
	// and encKey (client's decKey, stored in state.IV) for encryption
	encrypted, err := d.klapEncrypt(state, seqNo, respJSON)
	if err != nil {
		http.Error(w, "Encryption failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	w.Write(encrypted)
}

// klapDecrypt decrypts a KLAP payload using the decryption key (client's encKey).
func (d *MockDevice) klapDecrypt(state *KLAPState, seqNo int32, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("invalid ciphertext length")
	}

	// IV = sig (12 bytes) + seq (4 bytes big-endian)
	iv := make([]byte, 16)
	copy(iv, state.Sig)
	binary.BigEndian.PutUint32(iv[12:], uint32(seqNo))

	// Use state.Key which is the decryption key (client's encKey)
	block, err := aes.NewCipher(state.Key)
	if err != nil {
		return nil, err
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	if len(plaintext) > 0 {
		padLen := int(plaintext[len(plaintext)-1])
		if padLen <= 16 && padLen <= len(plaintext) {
			plaintext = plaintext[:len(plaintext)-padLen]
		}
	}

	return plaintext, nil
}

// klapEncrypt encrypts a KLAP payload using the encryption key (client's decKey).
func (d *MockDevice) klapEncrypt(state *KLAPState, seqNo int32, plaintext []byte) ([]byte, error) {
	// IV = sig (12 bytes) + seq (4 bytes big-endian)
	iv := make([]byte, 16)
	copy(iv, state.Sig)
	binary.BigEndian.PutUint32(iv[12:], uint32(seqNo))

	// Use state.IV which stores the encryption key (client's decKey)
	block, err := aes.NewCipher(state.IV)
	if err != nil {
		return nil, err
	}

	// Add PKCS7 padding
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(padded))
	mode.CryptBlocks(ciphertext, padded)

	return ciphertext, nil
}
