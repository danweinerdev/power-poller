package protocol

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/danweinerdev/power-poller/pkg/mockdevice"
)

func TestTransport_NewTransport(t *testing.T) {
	tests := []struct {
		name        string
		host        string
		opts        []TransportOption
		wantPort    int
		wantTimeout time.Duration
	}{
		{
			name:        "default options",
			host:        "192.168.1.100",
			wantPort:    DefaultPort,
			wantTimeout: DefaultTimeout,
		},
		{
			name:        "custom port",
			host:        "192.168.1.100",
			opts:        []TransportOption{WithPort(8080)},
			wantPort:    8080,
			wantTimeout: DefaultTimeout,
		},
		{
			name:        "custom timeout",
			host:        "192.168.1.100",
			opts:        []TransportOption{WithTimeout(10 * time.Second)},
			wantPort:    DefaultPort,
			wantTimeout: 10 * time.Second,
		},
		{
			name:        "multiple options",
			host:        "192.168.1.100",
			opts:        []TransportOption{WithPort(8080), WithTimeout(30 * time.Second)},
			wantPort:    8080,
			wantTimeout: 30 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tr := NewTransport(tc.host, tc.opts...)

			if tr.Host() != tc.host {
				t.Errorf("Host() = %q, want %q", tr.Host(), tc.host)
			}
			if tr.port != tc.wantPort {
				t.Errorf("port = %d, want %d", tr.port, tc.wantPort)
			}
			if tr.timeout != tc.wantTimeout {
				t.Errorf("timeout = %v, want %v", tr.timeout, tc.wantTimeout)
			}
		})
	}
}

func TestTransport_Connect(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()

	t.Run("successful connection", func(t *testing.T) {
		tr := NewTransport(mock.Address())

		if tr.IsConnected() {
			t.Error("should not be connected before Connect()")
		}

		if err := tr.Connect(ctx); err != nil {
			t.Errorf("Connect() error = %v", err)
		}
		defer tr.Close()

		if !tr.IsConnected() {
			t.Error("should be connected after Connect()")
		}
	})

	t.Run("already connected", func(t *testing.T) {
		tr := NewTransport(mock.Address())

		if err := tr.Connect(ctx); err != nil {
			t.Fatalf("first Connect() error = %v", err)
		}
		defer tr.Close()

		// Second connect should be a no-op
		if err := tr.Connect(ctx); err != nil {
			t.Errorf("second Connect() error = %v", err)
		}
	})
}

func TestTransport_Connect_InvalidHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	tr := NewTransport("invalid.host.that.does.not.exist.local")

	err := tr.Connect(ctx)
	if err == nil {
		tr.Close()
		t.Error("expected error for invalid host, got nil")
	}
}

func TestTransport_Connect_Timeout(t *testing.T) {
	// Use a non-routable IP to force timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tr := NewTransport("10.255.255.1") // Non-routable IP

	err := tr.Connect(ctx)
	if err == nil {
		tr.Close()
		t.Error("expected timeout error, got nil")
	}
}

func TestTransport_Close(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	tr := NewTransport(mock.Address())

	// Close without connect should be no-op
	if err := tr.Close(); err != nil {
		t.Errorf("Close() on unconnected transport error = %v", err)
	}

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if err := tr.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if tr.IsConnected() {
		t.Error("should not be connected after Close()")
	}

	// Double close should be no-op
	if err := tr.Close(); err != nil {
		t.Errorf("second Close() error = %v", err)
	}
}

func TestTransport_IsConnected(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	tr := NewTransport(mock.Address())

	if tr.IsConnected() {
		t.Error("should not be connected initially")
	}

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if !tr.IsConnected() {
		t.Error("should be connected after Connect()")
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if tr.IsConnected() {
		t.Error("should not be connected after Close()")
	}
}

func TestTransport_Send(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	tr := NewTransport(mock.Address())

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	resp, err := tr.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}

	// Verify command was received
	commands := mock.GetReceivedCommands()
	if len(commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(commands))
	}
}

func TestTransport_Send_NotConnected(t *testing.T) {
	tr := NewTransport("127.0.0.1")

	_, err := tr.Send(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error when not connected, got nil")
	}
}

func TestTransport_SendJSON(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	tr := NewTransport(mock.Address())

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	jsonCmd := []byte(`{"system":{"get_sysinfo":{}}}`)

	resp, err := tr.SendJSON(ctx, jsonCmd)
	if err != nil {
		t.Fatalf("SendJSON() error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestTransport_SendJSON_NotConnected(t *testing.T) {
	tr := NewTransport("127.0.0.1")

	_, err := tr.SendJSON(context.Background(), []byte(`{}`))
	if err == nil {
		t.Error("expected error when not connected, got nil")
	}
}

func TestTransport_Send_ConnectionClosed(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithErrorBehavior(mockdevice.ErrorBehavior{
			DropConnection: true,
		}),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	tr := NewTransport(mock.Address())

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	_, err := tr.Send(ctx, cmd)
	if err == nil {
		t.Error("expected error when connection is dropped, got nil")
	}

	// Connection should be cleared after error
	if tr.IsConnected() {
		t.Error("connection should be cleared after send error")
	}
}

func TestTransport_Send_ContextCancellation(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithErrorBehavior(mockdevice.ErrorBehavior{
			ResponseDelay: 5 * time.Second,
		}),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tr := NewTransport(mock.Address())

	// Connect with a longer timeout
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connectCancel()

	if err := tr.Connect(connectCtx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	_, err := tr.Send(ctx, cmd)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestTransport_ConcurrentAccess(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	tr := NewTransport(mock.Address())

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer tr.Close()

	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	// Run 10 concurrent sends
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tr.Send(ctx, cmd)
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Concurrent sends should work - the mutex ensures serialization
	for err := range errChan {
		t.Errorf("concurrent Send() error = %v", err)
	}
}

func TestTransport_Reconnection(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	tr := NewTransport(mock.Address())

	// First connection
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("first Connect() error = %v", err)
	}

	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	if _, err := tr.Send(ctx, cmd); err != nil {
		t.Errorf("first Send() error = %v", err)
	}

	// Close connection
	if err := tr.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Reconnect
	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("second Connect() error = %v", err)
	}
	defer tr.Close()

	if _, err := tr.Send(ctx, cmd); err != nil {
		t.Errorf("second Send() after reconnect error = %v", err)
	}
}

func TestQuery_OneOff(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	resp, err := Query(ctx, mock.Address(), cmd)
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestQuery_InvalidHost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	cmd := map[string]interface{}{
		"system": map[string]interface{}{
			"get_sysinfo": map[string]interface{}{},
		},
	}

	_, err := Query(ctx, "invalid.host.local", cmd)
	if err == nil {
		t.Error("expected error for invalid host, got nil")
	}
}

func TestQueryJSON_OneOff(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()
	jsonCmd := []byte(`{"system":{"get_sysinfo":{}}}`)

	resp, err := QueryJSON(ctx, mock.Address(), jsonCmd)
	if err != nil {
		t.Fatalf("QueryJSON() error = %v", err)
	}

	if len(resp) == 0 {
		t.Error("expected non-empty response")
	}
}

func TestTransport_HostWithPort(t *testing.T) {
	mock := mockdevice.New()
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock device: %v", err)
	}
	defer mock.Stop()

	ctx := context.Background()

	// Create transport with host that already includes port
	tr := NewTransport(mock.Address()) // Address() returns "127.0.0.1:port"

	if err := tr.Connect(ctx); err != nil {
		t.Fatalf("Connect() with host:port error = %v", err)
	}
	defer tr.Close()

	if !tr.IsConnected() {
		t.Error("should be connected")
	}
}
