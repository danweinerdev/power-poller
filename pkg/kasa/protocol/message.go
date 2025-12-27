package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MaxMessageSize is the maximum allowed message size (1MB).
const MaxMessageSize = 1 << 20

// FrameMessage prepends a 4-byte big-endian length header to the encrypted payload.
func FrameMessage(encrypted []byte) []byte {
	framed := make([]byte, 4+len(encrypted))
	binary.BigEndian.PutUint32(framed[:4], uint32(len(encrypted)))
	copy(framed[4:], encrypted)
	return framed
}

// EncryptAndFrame encrypts plaintext and adds the length header.
func EncryptAndFrame(plaintext []byte) []byte {
	encrypted := Encrypt(plaintext)
	return FrameMessage(encrypted)
}

// ReadMessage reads a framed message from a reader and returns the decrypted payload.
func ReadMessage(r io.Reader) ([]byte, error) {
	// Read 4-byte length header
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("connection closed: %w", err)
		}
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	length := binary.BigEndian.Uint32(header)
	if length > MaxMessageSize {
		return nil, fmt.Errorf("message too large: %d bytes (max %d)", length, MaxMessageSize)
	}

	if length == 0 {
		return []byte{}, nil
	}

	// Read encrypted payload
	encrypted := make([]byte, length)
	if _, err := io.ReadFull(r, encrypted); err != nil {
		return nil, fmt.Errorf("failed to read payload: %w", err)
	}

	// Decrypt and return
	return Decrypt(encrypted), nil
}

// DecryptWithLength decrypts a complete framed message (header + encrypted data).
// Useful for testing or processing captured data.
func DecryptWithLength(data []byte) ([]byte, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("data too short: need at least 4 bytes, got %d", len(data))
	}

	length := binary.BigEndian.Uint32(data[:4])
	if len(data) < int(4+length) {
		return nil, fmt.Errorf("data truncated: expected %d bytes of payload, got %d", length, len(data)-4)
	}

	if length == 0 {
		return []byte{}, nil
	}

	return Decrypt(data[4 : 4+length]), nil
}
