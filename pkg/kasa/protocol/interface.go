package protocol

import "context"

// Transporter defines the interface for device communication transports.
// Both legacy TCP/XOR and KLAP HTTP/AES transports implement this interface.
type Transporter interface {
	// Connect establishes a connection to the device.
	// For legacy transport, this opens a TCP connection.
	// For KLAP transport, this performs the handshake.
	Connect(ctx context.Context) error

	// Close closes the connection to the device.
	Close() error

	// IsConnected returns true if the transport has an active connection.
	IsConnected() bool

	// Send sends a command and receives the response.
	// The command should be a struct that can be marshaled to JSON.
	// Returns the decrypted response bytes.
	Send(ctx context.Context, cmd interface{}) ([]byte, error)

	// SendJSON sends a raw JSON command and receives the response.
	// Returns the decrypted response bytes.
	SendJSON(ctx context.Context, jsonCmd []byte) ([]byte, error)

	// Host returns the device host address.
	Host() string
}

// Ensure Transport implements Transporter.
var _ Transporter = (*Transport)(nil)
