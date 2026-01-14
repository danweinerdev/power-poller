package protocol

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

const (
	// DefaultPort is the standard KASA device port.
	DefaultPort = 9999

	// DefaultTimeout is the default read/write timeout.
	DefaultTimeout = 5 * time.Second

	// ConnectTimeout is the timeout for establishing connections.
	ConnectTimeout = 10 * time.Second
)

// Transport handles TCP communication with KASA devices.
type Transport struct {
	host    string
	port    int
	timeout time.Duration

	mu   sync.Mutex
	conn net.Conn
}

// TransportOption configures a Transport.
type TransportOption func(*Transport)

// WithPort sets a custom port.
func WithPort(port int) TransportOption {
	return func(t *Transport) {
		t.port = port
	}
}

// WithTimeout sets the read/write timeout.
func WithTimeout(d time.Duration) TransportOption {
	return func(t *Transport) {
		t.timeout = d
	}
}

// NewTransport creates a new Transport for the given host.
func NewTransport(host string, opts ...TransportOption) *Transport {
	t := &Transport{
		host:    host,
		port:    DefaultPort,
		timeout: DefaultTimeout,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Host returns the device host address.
func (t *Transport) Host() string {
	return t.host
}

// Connect establishes a TCP connection to the device.
func (t *Transport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.connectLocked(ctx)
}

// connectLocked establishes connection (caller must hold lock).
func (t *Transport) connectLocked(ctx context.Context) error {
	if t.conn != nil {
		return nil // Already connected
	}

	// Check if host already includes a port
	addr := t.host
	if _, _, err := net.SplitHostPort(t.host); err != nil {
		// No port in host, add the default/configured port
		addr = fmt.Sprintf("%s:%d", t.host, t.port)
	}

	dialer := &net.Dialer{
		Timeout: ConnectTimeout,
	}

	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	t.conn = conn
	return nil
}

// Close closes the TCP connection.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return nil
	}

	err := t.conn.Close()
	t.conn = nil
	return err
}

// IsConnected returns true if the transport has an active connection.
func (t *Transport) IsConnected() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.conn != nil
}

// Send sends a command and receives the response.
// The command should be a struct that can be marshaled to JSON.
func (t *Transport) Send(ctx context.Context, cmd interface{}) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Marshal command to JSON
	payload, err := json.Marshal(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal command: %w", err)
	}

	return t.sendRaw(ctx, payload)
}

// SendJSON sends a raw JSON command and receives the response.
func (t *Transport) SendJSON(ctx context.Context, jsonCmd []byte) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	return t.sendRaw(ctx, jsonCmd)
}

// sendRaw sends raw JSON bytes and receives the response (must hold lock).
func (t *Transport) sendRaw(ctx context.Context, payload []byte) ([]byte, error) {
	response, err := t.doSend(ctx, payload)
	if err != nil {
		// Some older devices (e.g., HS110) close the connection after each command.
		// If we got an EOF (conn was set to nil), try reconnecting once and resending.
		if t.conn == nil {
			if reconnErr := t.connectLocked(ctx); reconnErr != nil {
				return nil, fmt.Errorf("reconnect failed after %w: %v", err, reconnErr)
			}

			// Retry the send
			response, err = t.doSend(ctx, payload)
			if err != nil {
				return nil, err
			}
			return response, nil
		}
		return nil, err
	}
	return response, nil
}

// doSend performs the actual send/receive (must hold lock).
func (t *Transport) doSend(ctx context.Context, payload []byte) ([]byte, error) {
	if t.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	// Set deadline from context or timeout
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(t.timeout)
	}
	if err := t.conn.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// Encrypt and frame
	framed := EncryptAndFrame(payload)

	// Send
	if _, err := t.conn.Write(framed); err != nil {
		t.conn.Close()
		t.conn = nil // Connection is broken
		return nil, fmt.Errorf("failed to send: %w", err)
	}

	// Read response
	response, err := ReadMessage(t.conn)
	if err != nil {
		t.conn.Close()
		t.conn = nil // Connection is broken
		return nil, fmt.Errorf("failed to receive: %w", err)
	}

	return response, nil
}

// Query sends a command to a device without maintaining a persistent connection.
// This is useful for one-off commands.
func Query(ctx context.Context, host string, cmd interface{}, opts ...TransportOption) ([]byte, error) {
	t := NewTransport(host, opts...)

	if err := t.Connect(ctx); err != nil {
		return nil, err
	}
	defer t.Close()

	return t.Send(ctx, cmd)
}

// QueryJSON sends a raw JSON command without maintaining a persistent connection.
func QueryJSON(ctx context.Context, host string, jsonCmd []byte, opts ...TransportOption) ([]byte, error) {
	t := NewTransport(host, opts...)

	if err := t.Connect(ctx); err != nil {
		return nil, err
	}
	defer t.Close()

	return t.SendJSON(ctx, jsonCmd)
}
