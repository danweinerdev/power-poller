// Package mockdevice provides a mock KASA device for testing.
package mockdevice

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

// ProtocolType defines the protocol the mock device should use.
type ProtocolType string

const (
	// ProtocolLegacy uses TCP port 9999 with XOR encryption.
	ProtocolLegacy ProtocolType = "legacy"

	// ProtocolKLAP uses HTTP with binary KLAP handshake.
	ProtocolKLAP ProtocolType = "klap"

	// ProtocolSecurePassthrough uses HTTP with RSA/AES encryption.
	ProtocolSecurePassthrough ProtocolType = "securepassthrough"
)

// DeviceType represents the type of mock device.
type DeviceType string

const (
	DeviceTypePlug       DeviceType = "plug"
	DeviceTypeBulb       DeviceType = "bulb"
	DeviceTypeLightStrip DeviceType = "lightstrip"
	DeviceTypePowerStrip DeviceType = "powerstrip"
)

// DeviceCapabilities defines what features the mock device supports.
type DeviceCapabilities struct {
	HasEmeter    bool
	HasChildren  bool
	ChildCount   int
	HasDimmer    bool
	HasColorTemp bool
	HasColor     bool
}

// ErrorBehavior defines how the mock should simulate errors.
type ErrorBehavior struct {
	DisconnectAfterCommands int           // Disconnect after N commands (0 = never)
	ResponseDelay           time.Duration // Delay before responding
	DropConnection          bool          // Abruptly close connection
	InvalidResponse         bool          // Return malformed JSON
	TimeoutOnConnect        bool          // Never accept connection
	PartialResponse         bool          // Send incomplete data
}

// CommandRecord tracks commands received by the mock.
type CommandRecord struct {
	Timestamp time.Time
	Command   map[string]interface{}
	RawBytes  []byte
}

// ChildDevice represents a child outlet on a power strip.
type ChildDevice struct {
	ID      string
	Alias   string
	IsOn    bool
	Voltage float64
	Current float64
	Power   float64
	TotalWh float64
}

// MockDevice simulates a KASA smart device.
type MockDevice struct {
	mu sync.RWMutex

	// Configuration
	DeviceType   DeviceType
	Protocol     ProtocolType
	Capabilities DeviceCapabilities
	Alias        string
	Model        string
	MAC          string
	DeviceID     string
	HWVersion    string
	FWVersion    string

	// State
	IsOn       bool
	Brightness int // 0-100 for dimmable devices
	Hue        int // 0-360 for color devices
	Saturation int // 0-100 for color devices
	ColorTemp  int // Kelvin for color temp devices

	// Emeter state
	Voltage float64
	Current float64
	Power   float64
	TotalWh float64

	// Child device state (for power strips)
	Children []*ChildDevice

	// Error injection
	ErrorBehavior ErrorBehavior

	// Tracking
	ReceivedCommands []CommandRecord
	commandCount     int

	// Networking - Legacy TCP
	listener   net.Listener
	port       int
	running    bool
	shutdownCh chan struct{}

	// Networking - HTTP (KLAP/SecurePassthrough)
	httpServer *http.Server
	httpPort   int

	// Protocol-specific state
	klapState *KLAPState
	spState   *SecurePassthroughState

	// Custom response overrides
	responseOverrides map[string]interface{}
}

// Option is a functional option for configuring the mock.
type Option func(*MockDevice)

// New creates a new mock KASA device.
func New(opts ...Option) *MockDevice {
	d := &MockDevice{
		DeviceType:        DeviceTypePlug,
		Protocol:          ProtocolLegacy,
		Alias:             "Mock Device",
		Model:             "HS110(US)",
		MAC:               "AA:BB:CC:DD:EE:FF",
		DeviceID:          "80067ABCDEF1234567890",
		HWVersion:         "2.0",
		FWVersion:         "1.5.8",
		IsOn:              true,
		Voltage:           120.5,
		Current:           0.5,
		Power:             60.0,
		TotalWh:           1234.567,
		Brightness:        100,
		Hue:               0,
		Saturation:        0,
		ColorTemp:         2700,
		shutdownCh:        make(chan struct{}),
		responseOverrides: make(map[string]interface{}),
		Capabilities: DeviceCapabilities{
			HasEmeter: true,
		},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Start begins listening for connections.
func (d *MockDevice) Start() error {
	d.mu.Lock()
	protocol := d.Protocol
	d.mu.Unlock()

	switch protocol {
	case ProtocolKLAP:
		return d.startHTTP(d.handleKLAP)
	case ProtocolSecurePassthrough:
		return d.startHTTP(d.handleSecurePassthrough)
	default:
		return d.startLegacy()
	}
}

// startLegacy starts the legacy TCP server.
func (d *MockDevice) startLegacy() error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start listener: %w", err)
	}

	d.mu.Lock()
	d.listener = listener
	d.port = listener.Addr().(*net.TCPAddr).Port
	d.running = true
	d.mu.Unlock()

	go d.acceptLoop()
	return nil
}

// startHTTP starts an HTTP server for KLAP or SecurePassthrough.
func (d *MockDevice) startHTTP(handler http.HandlerFunc) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to start HTTP listener: %w", err)
	}

	d.mu.Lock()
	d.httpPort = listener.Addr().(*net.TCPAddr).Port
	d.port = d.httpPort // Use same port field for compatibility
	d.running = true
	d.httpServer = &http.Server{Handler: handler}
	d.mu.Unlock()

	go func() {
		d.httpServer.Serve(listener)
	}()

	return nil
}

// Port returns the TCP port the mock is listening on.
func (d *MockDevice) Port() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.port
}

// Address returns the full address string for connecting.
func (d *MockDevice) Address() string {
	return fmt.Sprintf("127.0.0.1:%d", d.Port())
}

// Host returns just the host portion (127.0.0.1).
func (d *MockDevice) Host() string {
	return "127.0.0.1"
}

// Stop shuts down the mock device.
func (d *MockDevice) Stop() error {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return nil
	}
	d.running = false

	// Close shutdown channel if it exists and isn't already closed
	select {
	case <-d.shutdownCh:
		// Already closed
	default:
		close(d.shutdownCh)
	}

	httpServer := d.httpServer
	listener := d.listener
	d.mu.Unlock()

	// Shutdown HTTP server if running
	if httpServer != nil {
		httpServer.Close()
	}

	// Close legacy listener if running
	if listener != nil {
		return listener.Close()
	}
	return nil
}

// Reset clears recorded commands and resets error behavior.
func (d *MockDevice) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ReceivedCommands = nil
	d.commandCount = 0
	d.ErrorBehavior = ErrorBehavior{}
}

// GetReceivedCommands returns all commands received.
func (d *MockDevice) GetReceivedCommands() []CommandRecord {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return append([]CommandRecord{}, d.ReceivedCommands...)
}

// SetResponseOverride sets a custom response for a specific command path.
// Path format: "namespace.action" (e.g., "system.get_sysinfo")
func (d *MockDevice) SetResponseOverride(path string, response interface{}) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.responseOverrides[path] = response
}

// ClearResponseOverrides removes all response overrides.
func (d *MockDevice) ClearResponseOverrides() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.responseOverrides = make(map[string]interface{})
}

// InjectError configures error behavior.
func (d *MockDevice) InjectError(behavior ErrorBehavior) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ErrorBehavior = behavior
}

// acceptLoop handles incoming connections.
func (d *MockDevice) acceptLoop() {
	for {
		select {
		case <-d.shutdownCh:
			return
		default:
		}

		d.mu.RLock()
		running := d.running
		d.mu.RUnlock()

		if !running {
			return
		}

		// Set accept deadline to allow checking shutdown
		d.listener.(*net.TCPListener).SetDeadline(time.Now().Add(100 * time.Millisecond))

		conn, err := d.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Normal timeout, check shutdown
			}
			return // Listener closed
		}

		go d.handleConnection(conn)
	}
}

// handleConnection handles a single client connection.
func (d *MockDevice) handleConnection(conn net.Conn) {
	defer conn.Close()

	for {
		d.mu.RLock()
		behavior := d.ErrorBehavior
		d.mu.RUnlock()

		// Check disconnect after N commands
		if behavior.DisconnectAfterCommands > 0 {
			d.mu.RLock()
			count := d.commandCount
			d.mu.RUnlock()
			if count >= behavior.DisconnectAfterCommands {
				return
			}
		}

		// Set read deadline
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// Read length header
		header := make([]byte, 4)
		if _, err := io.ReadFull(conn, header); err != nil {
			return // Connection closed or error
		}

		length := binary.BigEndian.Uint32(header)
		if length == 0 || length > 1<<20 {
			return // Invalid length
		}

		// Read encrypted payload
		encrypted := make([]byte, length)
		if _, err := io.ReadFull(conn, encrypted); err != nil {
			return
		}

		// Decrypt
		decrypted := decrypt(encrypted)

		// Parse command
		var cmd map[string]interface{}
		if err := json.Unmarshal(decrypted, &cmd); err != nil {
			return // Invalid JSON
		}

		// Record command
		d.mu.Lock()
		d.ReceivedCommands = append(d.ReceivedCommands, CommandRecord{
			Timestamp: time.Now(),
			Command:   cmd,
			RawBytes:  decrypted,
		})
		d.commandCount++
		d.mu.Unlock()

		// Apply response delay
		if behavior.ResponseDelay > 0 {
			time.Sleep(behavior.ResponseDelay)
		}

		// Generate response
		var response []byte
		if behavior.InvalidResponse {
			response = []byte("not valid json {{{")
		} else if behavior.DropConnection {
			return
		} else {
			resp := d.handleCommand(cmd)
			response, _ = json.Marshal(resp)
		}

		// Send partial response if configured
		if behavior.PartialResponse && len(response) > 10 {
			response = response[:len(response)/2]
		}

		// Encrypt and frame response
		encrypted = encrypt(response)
		framed := make([]byte, 4+len(encrypted))
		binary.BigEndian.PutUint32(framed[:4], uint32(len(encrypted)))
		copy(framed[4:], encrypted)

		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := conn.Write(framed); err != nil {
			return
		}
	}
}

// handleCommand processes a received command and returns a response.
func (d *MockDevice) handleCommand(cmd map[string]interface{}) map[string]interface{} {
	response := make(map[string]interface{})

	for namespace, methods := range cmd {
		methodsMap, ok := methods.(map[string]interface{})
		if !ok {
			continue
		}

		switch namespace {
		case "system":
			response["system"] = d.handleSystem(methodsMap)
		case "emeter":
			response["emeter"] = d.handleEmeter(methodsMap)
		case "smartlife.iot.smartbulb.lightingservice":
			response["smartlife.iot.smartbulb.lightingservice"] = d.handleLighting(methodsMap)
		case "smartlife.iot.common.emeter":
			response["smartlife.iot.common.emeter"] = d.handleEmeter(methodsMap)
		case "smartlife.iot.lightStrip":
			response["smartlife.iot.lightStrip"] = d.handleLighting(methodsMap)
		}
	}

	return response
}

// handleSystem handles system namespace commands.
func (d *MockDevice) handleSystem(methods map[string]interface{}) map[string]interface{} {
	response := make(map[string]interface{})

	for method, args := range methods {
		// Check for override
		d.mu.RLock()
		override, hasOverride := d.responseOverrides["system."+method]
		d.mu.RUnlock()
		if hasOverride {
			response[method] = override
			continue
		}

		switch method {
		case "get_sysinfo":
			response["get_sysinfo"] = d.getSysInfo()
		case "set_relay_state":
			response["set_relay_state"] = d.setRelayState(args)
		case "reboot":
			response["reboot"] = map[string]interface{}{"err_code": 0}
		case "set_dev_alias":
			response["set_dev_alias"] = d.setAlias(args)
		case "set_led_off":
			response["set_led_off"] = map[string]interface{}{"err_code": 0}
		}
	}

	return response
}

// getSysInfo returns the sysinfo response.
func (d *MockDevice) getSysInfo() map[string]interface{} {
	d.mu.RLock()
	defer d.mu.RUnlock()

	deviceType := "IOT.SMARTPLUGSWITCH"
	switch d.DeviceType {
	case DeviceTypeBulb:
		deviceType = "IOT.SMARTBULB"
	case DeviceTypeLightStrip:
		deviceType = "IOT.SMARTBULB"
	case DeviceTypePowerStrip:
		deviceType = "IOT.SMARTPLUGSWITCH"
	}

	info := map[string]interface{}{
		"err_code":    0,
		"sw_ver":      d.FWVersion,
		"hw_ver":      d.HWVersion,
		"type":        deviceType,
		"model":       d.Model,
		"mac":         d.MAC,
		"deviceId":    d.DeviceID,
		"hwId":        "80067ABCDEF1234567890HWID",
		"fwId":        "80067ABCDEF1234567890FWID",
		"oemId":       "80067ABCDEF1234567890OEMID",
		"alias":       d.Alias,
		"dev_name":    "Smart Wi-Fi Device",
		"icon_hash":   "",
		"relay_state": boolToInt(d.IsOn),
		"on_time":     12345,
		"active_mode": "schedule",
		"updating":    0,
		"rssi":        -50,
	}

	// Add feature string
	if d.Capabilities.HasEmeter {
		info["feature"] = "TIM:ENE"
	} else {
		info["feature"] = "TIM"
	}

	// Add bulb-specific fields
	if d.DeviceType == DeviceTypeBulb || d.DeviceType == DeviceTypeLightStrip {
		info["is_dimmable"] = boolToInt(d.Capabilities.HasDimmer)
		info["is_color"] = boolToInt(d.Capabilities.HasColor)
		info["is_variable_color_temp"] = boolToInt(d.Capabilities.HasColorTemp)
		info["light_state"] = map[string]interface{}{
			"on_off":     boolToInt(d.IsOn),
			"mode":       "normal",
			"hue":        d.Hue,
			"saturation": d.Saturation,
			"color_temp": d.ColorTemp,
			"brightness": d.Brightness,
		}
	}

	// Add length for light strips
	if d.DeviceType == DeviceTypeLightStrip {
		info["length"] = 16
	}

	// Add children for power strips
	if d.Capabilities.HasChildren && len(d.Children) > 0 {
		children := make([]map[string]interface{}, len(d.Children))
		for i, child := range d.Children {
			children[i] = map[string]interface{}{
				"id":      child.ID,
				"alias":   child.Alias,
				"state":   boolToInt(child.IsOn),
				"on_time": 1000,
			}
		}
		info["children"] = children
		info["child_num"] = len(d.Children)
	}

	return info
}

// setRelayState handles set_relay_state command.
func (d *MockDevice) setRelayState(args interface{}) map[string]interface{} {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if state, ok := argsMap["state"]; ok {
			d.mu.Lock()
			d.IsOn = state.(float64) != 0
			d.mu.Unlock()
		}
	}
	return map[string]interface{}{"err_code": 0}
}

// setAlias handles set_dev_alias command.
func (d *MockDevice) setAlias(args interface{}) map[string]interface{} {
	if argsMap, ok := args.(map[string]interface{}); ok {
		if alias, ok := argsMap["alias"]; ok {
			d.mu.Lock()
			d.Alias = alias.(string)
			d.mu.Unlock()
		}
	}
	return map[string]interface{}{"err_code": 0}
}

// handleEmeter handles emeter namespace commands.
func (d *MockDevice) handleEmeter(methods map[string]interface{}) map[string]interface{} {
	response := make(map[string]interface{})

	for method := range methods {
		switch method {
		case "get_realtime":
			d.mu.RLock()
			response["get_realtime"] = map[string]interface{}{
				"err_code":   0,
				"voltage_mv": int(d.Voltage * 1000),
				"current_ma": int(d.Current * 1000),
				"power_mw":   int(d.Power * 1000),
				"total_wh":   int(d.TotalWh),
			}
			d.mu.RUnlock()
		case "get_daystat":
			response["get_daystat"] = map[string]interface{}{
				"err_code": 0,
				"day_list": []map[string]interface{}{
					{"year": 2024, "month": 1, "day": 1, "energy_wh": 100},
					{"year": 2024, "month": 1, "day": 2, "energy_wh": 150},
				},
			}
		case "get_monthstat":
			response["get_monthstat"] = map[string]interface{}{
				"err_code": 0,
				"month_list": []map[string]interface{}{
					{"year": 2024, "month": 1, "energy_wh": 3000},
					{"year": 2024, "month": 2, "energy_wh": 2800},
				},
			}
		}
	}

	return response
}

// handleLighting handles lighting namespace commands.
func (d *MockDevice) handleLighting(methods map[string]interface{}) map[string]interface{} {
	response := make(map[string]interface{})

	for method, args := range methods {
		switch method {
		case "get_light_state":
			d.mu.RLock()
			response["get_light_state"] = map[string]interface{}{
				"err_code":   0,
				"on_off":     boolToInt(d.IsOn),
				"mode":       "normal",
				"hue":        d.Hue,
				"saturation": d.Saturation,
				"color_temp": d.ColorTemp,
				"brightness": d.Brightness,
			}
			d.mu.RUnlock()
		case "transition_light_state", "set_light_state":
			if argsMap, ok := args.(map[string]interface{}); ok {
				d.mu.Lock()
				if onOff, ok := argsMap["on_off"]; ok {
					d.IsOn = onOff.(float64) != 0
				}
				if brightness, ok := argsMap["brightness"]; ok {
					d.Brightness = int(brightness.(float64))
				}
				if hue, ok := argsMap["hue"]; ok {
					d.Hue = int(hue.(float64))
				}
				if saturation, ok := argsMap["saturation"]; ok {
					d.Saturation = int(saturation.(float64))
				}
				if colorTemp, ok := argsMap["color_temp"]; ok {
					d.ColorTemp = int(colorTemp.(float64))
				}
				d.mu.Unlock()
			}
			response[method] = map[string]interface{}{"err_code": 0}
		}
	}

	return response
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
