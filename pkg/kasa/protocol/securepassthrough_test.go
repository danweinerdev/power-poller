package protocol

import (
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
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// mockSecurePassthroughServer creates a mock SecurePassthrough HTTP server for testing.
type mockSecurePassthroughServer struct {
	server     *httptest.Server
	aesKey     []byte
	aesIV      []byte
	token      string
	deviceInfo map[string]interface{}
}

func newMockSecurePassthroughServer() *mockSecurePassthroughServer {
	m := &mockSecurePassthroughServer{
		token: "TEST_TOKEN_12345",
		deviceInfo: map[string]interface{}{
			"device_id": "TEST123",
			"model":     "EP25",
			"type":      "SMART.KASAPLUG",
			"device_on": true,
			"on_time":   12345,
		},
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]interface{}
		json.Unmarshal(body, &req)

		method, _ := req["method"].(string)

		switch method {
		case "handshake":
			m.handleHandshake(w, req)
		case "securePassthrough":
			m.handleSecurePassthrough(w, r, req)
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error_code": -1,
			})
		}
	})

	m.server = httptest.NewServer(handler)
	return m
}

func (m *mockSecurePassthroughServer) handleHandshake(w http.ResponseWriter, req map[string]interface{}) {
	params, _ := req["params"].(map[string]interface{})
	pubKeyPEM, _ := params["key"].(string)

	// Parse the public key
	block, _ := pem.Decode([]byte(pubKeyPEM))
	if block == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	pubKeyInterface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	pubKey, ok := pubKeyInterface.(*rsa.PublicKey)
	if !ok {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	// Generate AES key and IV
	m.aesKey = make([]byte, 16)
	m.aesIV = make([]byte, 16)
	rand.Read(m.aesKey)
	rand.Read(m.aesIV)

	// Combine key + IV
	combined := append(m.aesKey, m.aesIV...)

	// Encrypt with client's public key
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, combined)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error_code": -1001,
		})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:  "TP_SESSIONID",
		Value: "TEST_SESSION_ID",
	})

	json.NewEncoder(w).Encode(map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"key": base64.StdEncoding.EncodeToString(encrypted),
		},
	})
}

func (m *mockSecurePassthroughServer) handleSecurePassthrough(w http.ResponseWriter, r *http.Request, req map[string]interface{}) {
	params, _ := req["params"].(map[string]interface{})
	encryptedReq, _ := params["request"].(string)

	// Decode and decrypt the request
	encryptedBytes, _ := base64.StdEncoding.DecodeString(encryptedReq)
	decrypted := m.decrypt(encryptedBytes)

	var innerReq map[string]interface{}
	json.Unmarshal(decrypted, &innerReq)

	method, _ := innerReq["method"].(string)

	var response map[string]interface{}

	switch method {
	case "login_device":
		response = m.handleLogin(innerReq)
	case "get_device_info":
		// Check token
		if !strings.Contains(r.URL.RawQuery, "token="+m.token) {
			response = map[string]interface{}{"error_code": -1010}
		} else {
			response = map[string]interface{}{
				"error_code": 0,
				"result":     m.deviceInfo,
			}
		}
	case "get_emeter_data":
		if !strings.Contains(r.URL.RawQuery, "token="+m.token) {
			response = map[string]interface{}{"error_code": -1010}
		} else {
			response = map[string]interface{}{
				"error_code": 0,
				"result": map[string]interface{}{
					"current_ma": 100,
					"voltage_mv": 120000,
					"power_mw":   12000,
					"energy_wh":  1234,
				},
			}
		}
	case "set_device_info":
		if !strings.Contains(r.URL.RawQuery, "token="+m.token) {
			response = map[string]interface{}{"error_code": -1010}
		} else {
			// Update device_on state
			params, _ := innerReq["params"].(map[string]interface{})
			if deviceOn, ok := params["device_on"].(bool); ok {
				m.deviceInfo["device_on"] = deviceOn
			}
			response = map[string]interface{}{"error_code": 0}
		}
	default:
		response = map[string]interface{}{"error_code": -1002}
	}

	// Encrypt response
	respJSON, _ := json.Marshal(response)
	encrypted := m.encrypt(respJSON)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"response": base64.StdEncoding.EncodeToString(encrypted),
		},
	})
}

func (m *mockSecurePassthroughServer) handleLogin(req map[string]interface{}) map[string]interface{} {
	params, _ := req["params"].(map[string]interface{})
	usernameB64, _ := params["username"].(string)

	// Verify username is properly hashed
	expectedUsername := "test@tp-link.net"
	expectedHash := sha1.Sum([]byte(expectedUsername))
	expectedHex := fmt.Sprintf("%x", expectedHash)
	expectedB64 := base64.StdEncoding.EncodeToString([]byte(expectedHex))

	if usernameB64 != expectedB64 {
		return map[string]interface{}{"error_code": -1501}
	}

	return map[string]interface{}{
		"error_code": 0,
		"result": map[string]interface{}{
			"token": m.token,
		},
	}
}

func (m *mockSecurePassthroughServer) encrypt(plaintext []byte) []byte {
	block, _ := aes.NewCipher(m.aesKey)
	padLen := aes.BlockSize - (len(plaintext) % aes.BlockSize)
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, m.aesIV)
	mode.CryptBlocks(ciphertext, padded)
	return ciphertext
}

func (m *mockSecurePassthroughServer) decrypt(ciphertext []byte) []byte {
	if len(ciphertext) == 0 || len(ciphertext)%aes.BlockSize != 0 {
		return nil
	}
	block, _ := aes.NewCipher(m.aesKey)
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, m.aesIV)
	mode.CryptBlocks(plaintext, ciphertext)

	if len(plaintext) > 0 {
		padLen := int(plaintext[len(plaintext)-1])
		if padLen <= 16 && padLen <= len(plaintext) {
			plaintext = plaintext[:len(plaintext)-padLen]
		}
	}
	return plaintext
}

func (m *mockSecurePassthroughServer) Close() {
	m.server.Close()
}

func (m *mockSecurePassthroughServer) Host() string {
	// Extract host and port from server URL
	addr := m.server.Listener.Addr().(*net.TCPAddr)
	return addr.IP.String()
}

func (m *mockSecurePassthroughServer) Port() int {
	addr := m.server.Listener.Addr().(*net.TCPAddr)
	return addr.Port
}

func TestSecurePassthroughConnect(t *testing.T) {
	mock := newMockSecurePassthroughServer()
	defer mock.Close()

	transport := NewSecurePassthroughTransport(
		mock.Host(),
		WithSecurePassthroughPort(mock.Port()),
		WithSecurePassthroughTimeout(5*time.Second),
	)

	ctx := context.Background()
	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !transport.IsConnected() {
		t.Error("Expected IsConnected to return true after Connect")
	}

	transport.Close()
	if transport.IsConnected() {
		t.Error("Expected IsConnected to return false after Close")
	}
}

func TestSecurePassthroughSendCommand(t *testing.T) {
	mock := newMockSecurePassthroughServer()
	defer mock.Close()

	transport := NewSecurePassthroughTransport(
		mock.Host(),
		WithSecurePassthroughPort(mock.Port()),
	)

	ctx := context.Background()
	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Send get_device_info command
	cmd := map[string]interface{}{
		"method": "get_device_info",
	}

	resp, err := transport.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			Model    string `json:"model"`
			DeviceOn bool   `json:"device_on"`
		} `json:"result"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.ErrorCode != 0 {
		t.Errorf("Expected error_code 0, got %d", result.ErrorCode)
	}

	if result.Result.Model != "EP25" {
		t.Errorf("Expected model EP25, got %s", result.Result.Model)
	}

	if !result.Result.DeviceOn {
		t.Error("Expected device_on to be true")
	}
}

func TestSecurePassthroughEmeterData(t *testing.T) {
	mock := newMockSecurePassthroughServer()
	defer mock.Close()

	transport := NewSecurePassthroughTransport(
		mock.Host(),
		WithSecurePassthroughPort(mock.Port()),
	)

	ctx := context.Background()
	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Send get_emeter_data command
	cmd := map[string]interface{}{
		"method": "get_emeter_data",
	}

	resp, err := transport.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result struct {
		ErrorCode int `json:"error_code"`
		Result    struct {
			CurrentMA int `json:"current_ma"`
			VoltageMV int `json:"voltage_mv"`
			PowerMW   int `json:"power_mw"`
			EnergyWh  int `json:"energy_wh"`
		} `json:"result"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.ErrorCode != 0 {
		t.Errorf("Expected error_code 0, got %d", result.ErrorCode)
	}

	if result.Result.CurrentMA != 100 {
		t.Errorf("Expected current_ma 100, got %d", result.Result.CurrentMA)
	}

	if result.Result.VoltageMV != 120000 {
		t.Errorf("Expected voltage_mv 120000, got %d", result.Result.VoltageMV)
	}

	if result.Result.PowerMW != 12000 {
		t.Errorf("Expected power_mw 12000, got %d", result.Result.PowerMW)
	}
}

func TestSecurePassthroughSetDeviceInfo(t *testing.T) {
	mock := newMockSecurePassthroughServer()
	defer mock.Close()

	transport := NewSecurePassthroughTransport(
		mock.Host(),
		WithSecurePassthroughPort(mock.Port()),
	)

	ctx := context.Background()
	err := transport.Connect(ctx)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Turn off the device
	cmd := map[string]interface{}{
		"method": "set_device_info",
		"params": map[string]interface{}{
			"device_on": false,
		},
	}

	resp, err := transport.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result struct {
		ErrorCode int `json:"error_code"`
	}

	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if result.ErrorCode != 0 {
		t.Errorf("Expected error_code 0, got %d", result.ErrorCode)
	}

	// Verify device state changed
	infoCmd := map[string]interface{}{
		"method": "get_device_info",
	}

	resp, err = transport.Send(ctx, infoCmd)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var infoResult struct {
		Result struct {
			DeviceOn bool `json:"device_on"`
		} `json:"result"`
	}

	json.Unmarshal(resp, &infoResult)
	if infoResult.Result.DeviceOn {
		t.Error("Expected device_on to be false after set_device_info")
	}
}

func TestSecurePassthroughNotConnected(t *testing.T) {
	transport := NewSecurePassthroughTransport("localhost")

	ctx := context.Background()
	_, err := transport.Send(ctx, map[string]interface{}{"method": "test"})
	if err == nil {
		t.Error("Expected error when sending without connecting")
	}

	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("Expected 'not connected' error, got: %v", err)
	}
}

func TestSecurePassthroughHost(t *testing.T) {
	transport := NewSecurePassthroughTransport("192.168.1.100")

	if transport.Host() != "192.168.1.100" {
		t.Errorf("Expected host 192.168.1.100, got %s", transport.Host())
	}
}
