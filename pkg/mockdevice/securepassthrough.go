package mockdevice

import (
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
	"net/http"
	"strings"
	"time"
)

// SecurePassthroughState holds the SecurePassthrough session state.
type SecurePassthroughState struct {
	AESKey    []byte
	AESIV     []byte
	Token     string
	SessionID string
}

// handleSecurePassthrough handles SecurePassthrough protocol HTTP requests.
func (d *MockDevice) handleSecurePassthrough(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Set SHIP header to identify SecurePassthrough protocol
	w.Header().Set("Server", "SHIP")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	method, _ := req["method"].(string)

	switch method {
	case "handshake":
		d.handleSPHandshake(w, req)
	case "securePassthrough":
		d.handleSPRequest(w, r, req)
	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1,
		})
	}
}

// handleSPHandshake handles the SecurePassthrough handshake (RSA key exchange).
func (d *MockDevice) handleSPHandshake(w http.ResponseWriter, req map[string]interface{}) {
	params, _ := req["params"].(map[string]interface{})
	pubKeyPEM, _ := params["key"].(string)

	// Parse the client's public key
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	pubKey, ok := pubKeyInterface.(*rsa.PublicKey)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	// Generate AES key and IV
	aesKey := make([]byte, 16)
	aesIV := make([]byte, 16)
	rand.Read(aesKey)
	rand.Read(aesIV)

	// Combine key + IV
	combined := append(aesKey, aesIV...)

	// Encrypt with client's public key
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, combined)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	// Store session state
	d.mu.Lock()
	d.spState = &SecurePassthroughState{
		AESKey:    aesKey,
		AESIV:     aesIV,
		SessionID: "MOCK_SP_SESSION",
	}
	d.mu.Unlock()

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:  "TP_SESSIONID",
		Value: "MOCK_SP_SESSION",
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"key": base64.StdEncoding.EncodeToString(encrypted),
		},
	})
}

// handleSPRequest handles encrypted SecurePassthrough requests.
func (d *MockDevice) handleSPRequest(w http.ResponseWriter, r *http.Request, req map[string]interface{}) {
	d.mu.Lock()
	state := d.spState
	d.mu.Unlock()

	if state == nil || state.AESKey == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1010, // Invalid token/session
		})
		return
	}

	params, _ := req["params"].(map[string]interface{})
	encryptedReq, _ := params["request"].(string)

	// Decode and decrypt the request
	encryptedBytes, err := base64.StdEncoding.DecodeString(encryptedReq)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1003,
		})
		return
	}

	decrypted := d.spDecrypt(state, encryptedBytes)
	if decrypted == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1003,
		})
		return
	}

	var innerReq map[string]interface{}
	if err := json.Unmarshal(decrypted, &innerReq); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1003,
		})
		return
	}

	// Record command
	d.mu.Lock()
	d.ReceivedCommands = append(d.ReceivedCommands, CommandRecord{
		Timestamp: time.Now(),
		Command:   innerReq,
		RawBytes:  decrypted,
	})
	d.commandCount++
	d.mu.Unlock()

	method, _ := innerReq["method"].(string)

	var response map[string]interface{}

	switch method {
	case "login_device":
		response = d.handleSPLogin(innerReq)
	case "get_device_info":
		response = d.handleSPGetDeviceInfo(r)
	case "get_emeter_data":
		response = d.handleSPGetEmeterData(r)
	case "get_current_power":
		response = d.handleSPGetCurrentPower(r)
	case "set_device_info":
		response = d.handleSPSetDeviceInfo(r, innerReq)
	default:
		response = map[string]interface{}{"error_code": -1002}
	}

	// Encrypt response
	respJSON, _ := json.Marshal(response)
	encrypted := d.spEncrypt(state, respJSON)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"response": base64.StdEncoding.EncodeToString(encrypted),
		},
	})
}

// handleSPLogin handles TAPO login_device command.
func (d *MockDevice) handleSPLogin(req map[string]interface{}) map[string]interface{} {
	params, _ := req["params"].(map[string]interface{})
	usernameB64, _ := params["username"].(string)

	// Verify username is properly hashed (SHA1 hex encoded, then base64)
	// Default credentials: test@tp-link.net
	expectedUsername := "test@tp-link.net"
	expectedHash := sha1.Sum([]byte(expectedUsername))
	expectedHex := fmt.Sprintf("%x", expectedHash)
	expectedB64 := base64.StdEncoding.EncodeToString([]byte(expectedHex))

	if usernameB64 != expectedB64 {
		return map[string]interface{}{"error_code": -1501}
	}

	// Generate token
	token := "MOCK_TAPO_TOKEN_12345"

	d.mu.Lock()
	d.spState.Token = token
	d.mu.Unlock()

	return map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"token": token,
		},
	}
}

// handleSPGetDeviceInfo handles TAPO get_device_info command.
func (d *MockDevice) handleSPGetDeviceInfo(r *http.Request) map[string]interface{} {
	if !d.spCheckToken(r) {
		return map[string]interface{}{"error_code": -1010}
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	deviceType := "SMART.KASAPLUG"
	switch d.DeviceType {
	case DeviceTypeBulb:
		deviceType = "SMART.KASABULB"
	case DeviceTypeLightStrip:
		deviceType = "SMART.KASASTRIP"
	case DeviceTypePowerStrip:
		deviceType = "SMART.KASAPOWERSTRIP"
	}

	return map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"device_id":  d.DeviceID,
			"model":      d.Model,
			"type":       deviceType,
			"mac":        strings.ReplaceAll(d.MAC, ":", "-"),
			"hw_ver":     d.HWVersion,
			"fw_ver":     d.FWVersion,
			"nickname":   base64.StdEncoding.EncodeToString([]byte(d.Alias)),
			"device_on":  d.IsOn,
			"on_time":    12345,
			"rssi":       -50,
			"brightness": d.Brightness,
		},
	}
}

// handleSPGetEmeterData handles TAPO get_emeter_data command.
func (d *MockDevice) handleSPGetEmeterData(r *http.Request) map[string]interface{} {
	if !d.spCheckToken(r) {
		return map[string]interface{}{"error_code": -1010}
	}

	if !d.Capabilities.HasEmeter {
		return map[string]interface{}{"error_code": -1003}
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"current_ma": int(d.Current * 1000),
			"voltage_mv": int(d.Voltage * 1000),
			"power_mw":   int(d.Power * 1000),
			"energy_wh":  int(d.TotalWh),
		},
	}
}

// handleSPGetCurrentPower handles TAPO get_current_power command.
func (d *MockDevice) handleSPGetCurrentPower(r *http.Request) map[string]interface{} {
	if !d.spCheckToken(r) {
		return map[string]interface{}{"error_code": -1010}
	}

	if !d.Capabilities.HasEmeter {
		return map[string]interface{}{"error_code": -1003}
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	return map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"current_power": int(d.Power * 1000), // milliwatts
		},
	}
}

// handleSPSetDeviceInfo handles TAPO set_device_info command.
func (d *MockDevice) handleSPSetDeviceInfo(r *http.Request, req map[string]interface{}) map[string]interface{} {
	if !d.spCheckToken(r) {
		return map[string]interface{}{"error_code": -1010}
	}

	params, _ := req["params"].(map[string]interface{})

	d.mu.Lock()
	if deviceOn, ok := params["device_on"].(bool); ok {
		d.IsOn = deviceOn
	}
	if brightness, ok := params["brightness"].(float64); ok {
		d.Brightness = int(brightness)
	}
	d.mu.Unlock()

	return map[string]interface{}{"error_code": 0}
}

// spCheckToken verifies the token is present and valid.
func (d *MockDevice) spCheckToken(r *http.Request) bool {
	d.mu.RLock()
	state := d.spState
	d.mu.RUnlock()

	if state == nil || state.Token == "" {
		return false
	}

	query := r.URL.RawQuery
	return strings.Contains(query, "token="+state.Token)
}

// spDecrypt decrypts a SecurePassthrough payload using AES-CBC.
func (d *MockDevice) spDecrypt(state *SecurePassthroughState, ciphertext []byte) []byte {
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil
	}

	block, err := aes.NewCipher(state.AESKey)
	if err != nil {
		return nil
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, state.AESIV)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	if len(plaintext) > 0 {
		padLen := int(plaintext[len(plaintext)-1])
		if padLen <= 16 && padLen <= len(plaintext) {
			plaintext = plaintext[:len(plaintext)-padLen]
		}
	}

	return plaintext
}

// spEncrypt encrypts a SecurePassthrough payload using AES-CBC.
func (d *MockDevice) spEncrypt(state *SecurePassthroughState, plaintext []byte) []byte {
	block, err := aes.NewCipher(state.AESKey)
	if err != nil {
		return nil
	}

	// Add PKCS7 padding
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, state.AESIV)
	mode.CryptBlocks(ciphertext, padded)

	return ciphertext
}
