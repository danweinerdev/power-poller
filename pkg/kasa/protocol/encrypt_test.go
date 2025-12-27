package protocol

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple command",
			plaintext: `{"system":{"get_sysinfo":{}}}`,
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "single byte",
			plaintext: "a",
		},
		{
			name:      "complex json",
			plaintext: `{"emeter":{"get_realtime":{"child_ids":["800012345600"]}}}`,
		},
		{
			name:      "unicode characters",
			plaintext: `{"system":{"set_dev_alias":{"alias":"Test Device 中文"}}}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encrypted := Encrypt([]byte(tc.plaintext))
			decrypted := Decrypt(encrypted)

			if string(decrypted) != tc.plaintext {
				t.Errorf("round-trip failed: got %q, want %q", string(decrypted), tc.plaintext)
			}
		})
	}
}

func TestEncryptKnownValue(t *testing.T) {
	// Test against known encrypted value
	// The XOR cipher with rolling key starting at 0xAB should produce predictable output
	plaintext := []byte("test")
	encrypted := Encrypt(plaintext)

	// First byte: 't' (0x74) ^ 0xAB = 0xDF
	if encrypted[0] != 0xDF {
		t.Errorf("first byte: got 0x%02X, want 0xDF", encrypted[0])
	}

	// Verify it decrypts back
	decrypted := Decrypt(encrypted)
	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("decryption failed: got %v, want %v", decrypted, plaintext)
	}
}

func TestEncryptEmpty(t *testing.T) {
	encrypted := Encrypt([]byte{})
	if len(encrypted) != 0 {
		t.Errorf("encrypting empty should return empty, got %d bytes", len(encrypted))
	}

	decrypted := Decrypt([]byte{})
	if len(decrypted) != 0 {
		t.Errorf("decrypting empty should return empty, got %d bytes", len(decrypted))
	}
}

func TestEncryptLargePayload(t *testing.T) {
	// Test with a 10KB payload
	plaintext := make([]byte, 10000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	encrypted := Encrypt(plaintext)
	decrypted := Decrypt(encrypted)

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("large payload round-trip failed")
	}
}

func BenchmarkEncrypt(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		data := make([]byte, size)
		b.Run(string(rune('0'+size/1000))+"KB", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Encrypt(data)
			}
		})
	}
}

func BenchmarkDecrypt(b *testing.B) {
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		data := make([]byte, size)
		encrypted := Encrypt(data)
		b.Run(string(rune('0'+size/1000))+"KB", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Decrypt(encrypted)
			}
		})
	}
}
