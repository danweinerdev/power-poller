package mockdevice

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/command"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
)

func TestMockDevice_StartStop(t *testing.T) {
	mock := New()

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if mock.Port() == 0 {
		t.Error("Port should be non-zero after start")
	}

	if err := mock.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestMockDevice_GetSysInfo(t *testing.T) {
	mock := New(
		WithAlias("Test Plug"),
		WithEmeter(true),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	// Connect and send command
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewTransport(mock.Host(), protocol.WithPort(mock.Port()))
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	resp, err := transport.Send(ctx, command.GetSysInfo())
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Parse response
	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify response structure
	system, ok := result["system"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing system key in response")
	}

	sysinfo, ok := system["get_sysinfo"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing get_sysinfo key in response")
	}

	if sysinfo["alias"] != "Test Plug" {
		t.Errorf("alias: got %v, want %v", sysinfo["alias"], "Test Plug")
	}

	// Verify command was recorded
	cmds := mock.GetReceivedCommands()
	if len(cmds) != 1 {
		t.Errorf("expected 1 recorded command, got %d", len(cmds))
	}
}

func TestMockDevice_SetRelayState(t *testing.T) {
	mock := New(WithState(false))

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewTransport(mock.Host(), protocol.WithPort(mock.Port()))
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Turn on
	_, err := transport.Send(ctx, command.SetRelayState(true))
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify state changed
	if !mock.IsOn {
		t.Error("Device should be on after SetRelayState(true)")
	}
}

func TestMockDevice_Emeter(t *testing.T) {
	mock := New(
		WithEmeter(true),
		WithEmeterValues(120.5, 0.5, 60.0, 1234.5),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewTransport(mock.Host(), protocol.WithPort(mock.Port()))
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	resp, err := transport.Send(ctx, command.GetEmeterRealtime())
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	emeter, ok := result["emeter"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing emeter key")
	}

	realtime, ok := emeter["get_realtime"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing get_realtime key")
	}

	// Check values (in millivolts/milliwatts from mock)
	if realtime["voltage_mv"].(float64) != 120500 {
		t.Errorf("voltage_mv: got %v, want 120500", realtime["voltage_mv"])
	}
}

func TestMockDevice_PowerStrip(t *testing.T) {
	mock := New(
		WithDeviceType(DeviceTypePowerStrip),
		WithChildren(6),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewTransport(mock.Host(), protocol.WithPort(mock.Port()))
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	resp, err := transport.Send(ctx, command.GetSysInfo())
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	system := result["system"].(map[string]interface{})
	sysinfo := system["get_sysinfo"].(map[string]interface{})

	childNum := int(sysinfo["child_num"].(float64))
	if childNum != 6 {
		t.Errorf("child_num: got %d, want 6", childNum)
	}

	children := sysinfo["children"].([]interface{})
	if len(children) != 6 {
		t.Errorf("children count: got %d, want 6", len(children))
	}
}

func TestMockDevice_Bulb(t *testing.T) {
	mock := New(
		WithDeviceType(DeviceTypeBulb),
		WithBrightness(50),
		WithHSV(120, 80, 50),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewTransport(mock.Host(), protocol.WithPort(mock.Port()))
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	resp, err := transport.Send(ctx, command.GetSysInfo())
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	system := result["system"].(map[string]interface{})
	sysinfo := system["get_sysinfo"].(map[string]interface{})

	// Verify bulb-specific fields
	if sysinfo["is_dimmable"].(float64) != 1 {
		t.Error("bulb should be dimmable")
	}
	if sysinfo["is_color"].(float64) != 1 {
		t.Error("bulb should have color")
	}

	lightState := sysinfo["light_state"].(map[string]interface{})
	if int(lightState["brightness"].(float64)) != 50 {
		t.Errorf("brightness: got %v, want 50", lightState["brightness"])
	}
	if int(lightState["hue"].(float64)) != 120 {
		t.Errorf("hue: got %v, want 120", lightState["hue"])
	}
}

func TestMockDevice_ErrorBehavior_ResponseDelay(t *testing.T) {
	mock := New(WithResponseDelay(100 * time.Millisecond))

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewTransport(mock.Host(), protocol.WithPort(mock.Port()))
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	start := time.Now()
	_, err := transport.Send(ctx, command.GetSysInfo())
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if elapsed < 100*time.Millisecond {
		t.Errorf("response should have taken at least 100ms, took %v", elapsed)
	}
}

func TestMockDevice_Reset(t *testing.T) {
	mock := New()

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewTransport(mock.Host(), protocol.WithPort(mock.Port()))
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Send a command
	_, err := transport.Send(ctx, command.GetSysInfo())
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	if len(mock.GetReceivedCommands()) != 1 {
		t.Error("should have 1 command")
	}

	// Reset
	mock.Reset()

	if len(mock.GetReceivedCommands()) != 0 {
		t.Error("should have 0 commands after reset")
	}
}
