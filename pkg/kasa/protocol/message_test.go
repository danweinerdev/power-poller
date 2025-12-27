package protocol

import (
	"bytes"
	"testing"
)

func TestFrameMessage(t *testing.T) {
	tests := []struct {
		name           string
		encrypted      []byte
		expectedHeader []byte
	}{
		{
			name:           "normal message",
			encrypted:      make([]byte, 29),
			expectedHeader: []byte{0x00, 0x00, 0x00, 0x1D}, // 29 in big-endian
		},
		{
			name:           "empty message",
			encrypted:      []byte{},
			expectedHeader: []byte{0x00, 0x00, 0x00, 0x00},
		},
		{
			name:           "256 bytes",
			encrypted:      make([]byte, 256),
			expectedHeader: []byte{0x00, 0x00, 0x01, 0x00}, // 256 in big-endian
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := FrameMessage(tc.encrypted)

			if len(result) < 4 {
				t.Fatal("result too short")
			}

			if !bytes.Equal(result[:4], tc.expectedHeader) {
				t.Errorf("header: got %v, want %v", result[:4], tc.expectedHeader)
			}

			if !bytes.Equal(result[4:], tc.encrypted) {
				t.Error("payload mismatch")
			}
		})
	}
}

func TestEncryptAndFrame(t *testing.T) {
	plaintext := []byte(`{"system":{"get_sysinfo":{}}}`)

	framed := EncryptAndFrame(plaintext)

	// Verify length header
	if len(framed) != 4+len(plaintext) {
		t.Errorf("framed length: got %d, want %d", len(framed), 4+len(plaintext))
	}

	// Decrypt and verify
	decrypted, err := DecryptWithLength(framed)
	if err != nil {
		t.Fatalf("DecryptWithLength failed: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip failed: got %q, want %q", string(decrypted), string(plaintext))
	}
}

func TestReadMessage(t *testing.T) {
	plaintext := []byte(`{"system":{"get_sysinfo":{}}}`)
	framed := EncryptAndFrame(plaintext)

	reader := bytes.NewReader(framed)
	result, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	if string(result) != string(plaintext) {
		t.Errorf("got %q, want %q", string(result), string(plaintext))
	}
}

func TestReadMessageMultiple(t *testing.T) {
	// Test reading multiple messages from a stream
	msg1 := []byte(`{"msg":1}`)
	msg2 := []byte(`{"msg":2}`)

	framed1 := EncryptAndFrame(msg1)
	framed2 := EncryptAndFrame(msg2)

	combined := append(framed1, framed2...)
	reader := bytes.NewReader(combined)

	result1, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage 1 failed: %v", err)
	}
	if string(result1) != string(msg1) {
		t.Errorf("msg1: got %q, want %q", string(result1), string(msg1))
	}

	result2, err := ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage 2 failed: %v", err)
	}
	if string(result2) != string(msg2) {
		t.Errorf("msg2: got %q, want %q", string(result2), string(msg2))
	}
}

func TestDecryptWithLength_Errors(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError string
	}{
		{
			name:        "too short",
			data:        []byte{0x00, 0x00},
			expectError: "data too short",
		},
		{
			name:        "truncated data",
			data:        []byte{0x00, 0x00, 0x00, 0x10, 0x01, 0x02}, // Says 16 bytes, only 2
			expectError: "data truncated",
		},
		{
			name:        "exactly 4 bytes with zero length",
			data:        []byte{0x00, 0x00, 0x00, 0x00},
			expectError: "", // Should succeed with empty result
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := DecryptWithLength(tc.data)

			if tc.expectError != "" {
				if err == nil {
					t.Error("expected error, got nil")
				} else if !bytes.Contains([]byte(err.Error()), []byte(tc.expectError)) {
					t.Errorf("error should contain %q, got %q", tc.expectError, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tc.name == "exactly 4 bytes with zero length" && len(result) != 0 {
					t.Errorf("expected empty result, got %d bytes", len(result))
				}
			}
		})
	}
}

func TestReadMessage_EOF(t *testing.T) {
	reader := bytes.NewReader([]byte{})
	_, err := ReadMessage(reader)
	if err == nil {
		t.Error("expected error on empty reader")
	}
}

func TestReadMessage_PartialHeader(t *testing.T) {
	reader := bytes.NewReader([]byte{0x00, 0x00}) // Only 2 bytes
	_, err := ReadMessage(reader)
	if err == nil {
		t.Error("expected error on partial header")
	}
}

func TestReadMessage_TooLarge(t *testing.T) {
	// Create a header claiming a message larger than MaxMessageSize
	header := []byte{0x10, 0x00, 0x00, 0x00} // 256MB
	reader := bytes.NewReader(header)
	_, err := ReadMessage(reader)
	if err == nil {
		t.Error("expected error on too-large message")
	}
}
