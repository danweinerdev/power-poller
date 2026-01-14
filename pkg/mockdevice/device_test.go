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

func TestMockDevice_KLAP(t *testing.T) {
	mock := New(
		WithProtocol(ProtocolKLAP),
		WithAlias("KLAP Plug"),
		WithEmeter(true),
		WithEmeterValues(120.5, 0.5, 60.0, 1234.5),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use KLAP transport
	creds := &protocol.Credentials{
		Username: "test@tp-link.net",
		Password: "test",
	}
	transport := protocol.NewKLAPTransport(mock.Host(),
		protocol.WithKLAPPort(mock.Port()),
		protocol.WithCredentials(creds),
	)

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Send get_sysinfo command
	resp, err := transport.Send(ctx, command.GetSysInfo())
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	system, ok := result["system"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing system key in response")
	}

	sysinfo, ok := system["get_sysinfo"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing get_sysinfo key in response")
	}

	if sysinfo["alias"] != "KLAP Plug" {
		t.Errorf("alias: got %v, want KLAP Plug", sysinfo["alias"])
	}

	// Verify command was recorded
	cmds := mock.GetReceivedCommands()
	if len(cmds) < 1 {
		t.Errorf("expected at least 1 recorded command, got %d", len(cmds))
	}
}

func TestMockDevice_KLAP_Emeter(t *testing.T) {
	mock := New(
		WithProtocol(ProtocolKLAP),
		WithEmeter(true),
		WithEmeterValues(120.5, 0.5, 60.0, 1234.5),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	creds := &protocol.Credentials{
		Username: "test@tp-link.net",
		Password: "test",
	}
	transport := protocol.NewKLAPTransport(mock.Host(),
		protocol.WithKLAPPort(mock.Port()),
		protocol.WithCredentials(creds),
	)

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

	// Check values
	if realtime["voltage_mv"].(float64) != 120500 {
		t.Errorf("voltage_mv: got %v, want 120500", realtime["voltage_mv"])
	}
}

func TestMockDevice_SecurePassthrough(t *testing.T) {
	mock := New(
		WithProtocol(ProtocolSecurePassthrough),
		WithAlias("TAPO Plug"),
		WithEmeter(true),
		WithEmeterValues(120.5, 0.5, 60.0, 1234.5),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use SecurePassthrough transport
	transport := protocol.NewSecurePassthroughTransport(mock.Host(),
		protocol.WithSecurePassthroughPort(mock.Port()),
	)

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Send get_device_info command (TAPO format)
	cmd := map[string]interface{}{
		"method": "get_device_info",
	}
	resp, err := transport.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	errorCode, ok := result["error_code"].(float64)
	if !ok || errorCode != 0 {
		t.Fatalf("Expected error_code 0, got %v", result["error_code"])
	}

	deviceResult, ok := result["result"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing result key in response")
	}

	if deviceResult["model"] != "EP25" {
		t.Errorf("model: got %v, want EP25", deviceResult["model"])
	}

	// Verify command was recorded
	cmds := mock.GetReceivedCommands()
	if len(cmds) < 1 {
		t.Errorf("expected at least 1 recorded command, got %d", len(cmds))
	}
}

func TestMockDevice_SecurePassthrough_Emeter(t *testing.T) {
	mock := New(
		WithProtocol(ProtocolSecurePassthrough),
		WithEmeter(true),
		WithEmeterValues(120.5, 0.5, 60.0, 1234.5),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewSecurePassthroughTransport(mock.Host(),
		protocol.WithSecurePassthroughPort(mock.Port()),
	)

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Send get_emeter_data command (TAPO format)
	cmd := map[string]interface{}{
		"method": "get_emeter_data",
	}
	resp, err := transport.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	errorCode, ok := result["error_code"].(float64)
	if !ok || errorCode != 0 {
		t.Fatalf("Expected error_code 0, got %v", result["error_code"])
	}

	emeterResult, ok := result["result"].(map[string]interface{})
	if !ok {
		t.Fatal("Missing result key in response")
	}

	// Check values (in millivolts/milliamps/milliwatts from mock)
	if emeterResult["voltage_mv"].(float64) != 120500 {
		t.Errorf("voltage_mv: got %v, want 120500", emeterResult["voltage_mv"])
	}
	if emeterResult["current_ma"].(float64) != 500 {
		t.Errorf("current_ma: got %v, want 500", emeterResult["current_ma"])
	}
	if emeterResult["power_mw"].(float64) != 60000 {
		t.Errorf("power_mw: got %v, want 60000", emeterResult["power_mw"])
	}
}

func TestMockDevice_SecurePassthrough_SetDeviceInfo(t *testing.T) {
	mock := New(
		WithProtocol(ProtocolSecurePassthrough),
		WithState(false),
	)

	if err := mock.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	transport := protocol.NewSecurePassthroughTransport(mock.Host(),
		protocol.WithSecurePassthroughPort(mock.Port()),
	)

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer transport.Close()

	// Turn on the device
	cmd := map[string]interface{}{
		"method": "set_device_info",
		"params": map[string]interface{}{
			"device_on": true,
		},
	}
	resp, err := transport.Send(ctx, cmd)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	errorCode, ok := result["error_code"].(float64)
	if !ok || errorCode != 0 {
		t.Fatalf("Expected error_code 0, got %v", result["error_code"])
	}

	// Verify state changed
	if !mock.IsOn {
		t.Error("Device should be on after set_device_info with device_on=true")
	}
}
