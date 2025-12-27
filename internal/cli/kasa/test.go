package kasa

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
	"github.com/spf13/cobra"
)

// TestResult represents the result of a single test.
type TestResult struct {
	Name    string
	Passed  bool
	Message string
	Error   error
}

// TestSuite runs integration tests against a real device.
type TestSuite struct {
	device  device.Device
	results []TestResult
	logger  *slog.Logger
}

func newTestCommand() *cobra.Command {
	var testTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "test",
		Short: "Run integration tests against a real device",
		Long: `Connects to a real KASA device, auto-discovers its type,
and runs a comprehensive suite of tests based on device capabilities.

Tests include:
  - Connection and sysinfo retrieval
  - Power state control (on/off/toggle)
  - Energy meter readings (if supported)
  - Brightness control (bulbs only)
  - Color/HSV control (color bulbs only)
  - Color temperature (tunable bulbs only)
  - Child device control (power strips only)

The device's original state is restored after testing.`,
		Example: `  kasa-monitor kasa test -H 192.168.1.100
  kasa-monitor kasa test --host 10.5.0.31 --test-timeout 60s`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTest(cmd, args, testTimeout)
		},
	}

	cmd.Flags().DurationVar(&testTimeout, "test-timeout", 30*time.Second, "Overall test timeout")

	return cmd
}

func runTest(cmd *cobra.Command, args []string, testTimeout time.Duration) error {
	// host is the global variable from kasa.go

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	logger := slog.Default()

	fmt.Printf("\n╔══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║           KASA Device Integration Test Suite                 ║\n")
	fmt.Printf("╚══════════════════════════════════════════════════════════════╝\n\n")

	fmt.Printf("Target: %s\n", host)
	fmt.Printf("Timeout: %s\n\n", testTimeout)

	// Phase 1: Discovery
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Phase 1: Device Discovery\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	dev, err := device.Load(ctx, host)
	if err != nil {
		fmt.Printf("  ✗ FAILED: Could not connect to device: %v\n\n", err)
		return fmt.Errorf("device discovery failed: %w", err)
	}
	defer dev.Close()

	fmt.Printf("  ✓ Connected successfully\n")
	fmt.Printf("  ✓ Device Type: %s\n", dev.Type())
	fmt.Printf("  ✓ Model: %s\n", dev.Model())
	fmt.Printf("  ✓ Alias: %s\n", dev.Alias())
	fmt.Printf("  ✓ MAC: %s\n", dev.MAC())
	fmt.Printf("  ✓ Device ID: %s\n", dev.DeviceID())

	sysinfo := dev.SysInfo()
	if sysinfo != nil {
		fmt.Printf("  ✓ Hardware: %s\n", sysinfo.HWVersion)
		fmt.Printf("  ✓ Firmware: %s\n", sysinfo.SWVersion)
		if sysinfo.Feature != "" {
			fmt.Printf("  ✓ Features: %s\n", sysinfo.Feature)
		}
	}
	fmt.Println()

	// Create test suite
	suite := &TestSuite{
		device: dev,
		logger: logger,
	}

	// Phase 2: Run device-specific tests
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Phase 2: Device Tests\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	// Run common tests
	suite.testPowerControl(ctx)

	// Run emeter tests if supported
	if emeterDev, ok := dev.(device.EmeterDevice); ok && emeterDev.HasEmeter() {
		suite.testEmeter(ctx, emeterDev)
	}

	// Run bulb-specific tests
	if dev.Type() == types.DeviceTypeBulb || dev.Type() == types.DeviceTypeLightStrip {
		suite.testBulbFeatures(ctx)
	}

	// Run power strip tests
	if parentDev, ok := dev.(device.ParentDevice); ok && parentDev.HasChildren() {
		suite.testPowerStripChildren(ctx, parentDev)
	}

	// Phase 3: Summary
	fmt.Printf("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("Test Summary\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n\n")

	passed := 0
	failed := 0
	for _, r := range suite.results {
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("  Total:  %d\n", len(suite.results))
	fmt.Printf("  Passed: %d\n", passed)
	fmt.Printf("  Failed: %d\n", failed)

	if failed > 0 {
		fmt.Printf("\n  Failed tests:\n")
		for _, r := range suite.results {
			if !r.Passed {
				fmt.Printf("    ✗ %s: %s\n", r.Name, r.Message)
				if r.Error != nil {
					fmt.Printf("      Error: %v\n", r.Error)
				}
			}
		}
		fmt.Println()
		return fmt.Errorf("%d tests failed", failed)
	}

	fmt.Printf("\n  ✓ All tests passed!\n\n")
	return nil
}

func (s *TestSuite) addResult(name string, passed bool, message string, err error) {
	result := TestResult{
		Name:    name,
		Passed:  passed,
		Message: message,
		Error:   err,
	}
	s.results = append(s.results, result)

	if passed {
		fmt.Printf("  ✓ %s: %s\n", name, message)
	} else {
		fmt.Printf("  ✗ %s: %s\n", name, message)
		if err != nil {
			fmt.Printf("    Error: %v\n", err)
		}
	}
}

func (s *TestSuite) testPowerControl(ctx context.Context) {
	fmt.Printf("\n[Power Control Tests]\n")

	// Save original state
	originalState := s.device.IsOn()
	s.addResult("Get Power State", true, fmt.Sprintf("Current state: %s", onOff(originalState)), nil)

	// Test turning off
	if err := s.device.TurnOff(ctx); err != nil {
		s.addResult("Turn Off", false, "Failed to turn off", err)
	} else {
		// Small delay for device to update
		time.Sleep(500 * time.Millisecond)
		if err := s.device.Update(ctx); err != nil {
			s.addResult("Turn Off", false, "Failed to update state after turn off", err)
		} else if s.device.IsOn() {
			s.addResult("Turn Off", false, "Device still reports ON after TurnOff", nil)
		} else {
			s.addResult("Turn Off", true, "Device turned off successfully", nil)
		}
	}

	// Test turning on
	if err := s.device.TurnOn(ctx); err != nil {
		s.addResult("Turn On", false, "Failed to turn on", err)
	} else {
		time.Sleep(500 * time.Millisecond)
		if err := s.device.Update(ctx); err != nil {
			s.addResult("Turn On", false, "Failed to update state after turn on", err)
		} else if !s.device.IsOn() {
			s.addResult("Turn On", false, "Device still reports OFF after TurnOn", nil)
		} else {
			s.addResult("Turn On", true, "Device turned on successfully", nil)
		}
	}

	// Restore original state
	if originalState {
		s.device.TurnOn(ctx)
	} else {
		s.device.TurnOff(ctx)
	}
	s.addResult("Restore State", true, fmt.Sprintf("Restored to: %s", onOff(originalState)), nil)
}

func (s *TestSuite) testEmeter(ctx context.Context, emeterDev device.EmeterDevice) {
	fmt.Printf("\n[Energy Meter Tests]\n")

	s.addResult("Has Emeter", true, "Device supports energy monitoring", nil)

	data, err := emeterDev.GetEmeterRealtime(ctx)
	if err != nil {
		s.addResult("Get Realtime", false, "Failed to get realtime data", err)
		return
	}

	// Validate readings are in reasonable ranges
	valid := true
	var issues []string

	if data.Voltage < 0 || data.Voltage > 300 {
		valid = false
		issues = append(issues, fmt.Sprintf("voltage out of range: %.2f", data.Voltage))
	}
	if data.Current < 0 || data.Current > 20 {
		valid = false
		issues = append(issues, fmt.Sprintf("current out of range: %.2f", data.Current))
	}
	if data.Power < 0 || data.Power > 5000 {
		valid = false
		issues = append(issues, fmt.Sprintf("power out of range: %.2f", data.Power))
	}
	if data.Total < 0 {
		valid = false
		issues = append(issues, fmt.Sprintf("total negative: %.2f", data.Total))
	}

	if valid {
		s.addResult("Get Realtime", true,
			fmt.Sprintf("V=%.1fV, I=%.3fA, P=%.1fW, Total=%.2fkWh",
				data.Voltage, data.Current, data.Power, data.Total), nil)
	} else {
		s.addResult("Get Realtime", false, strings.Join(issues, "; "), nil)
	}

	// Test daily stats
	now := time.Now()
	daily, err := emeterDev.GetEmeterDaily(ctx, now.Year(), int(now.Month()))
	if err != nil {
		s.addResult("Get Daily Stats", false, "Failed to get daily stats", err)
	} else {
		s.addResult("Get Daily Stats", true, fmt.Sprintf("Got %d days of data", len(daily)), nil)
	}

	// Test monthly stats
	monthly, err := emeterDev.GetEmeterMonthly(ctx, now.Year())
	if err != nil {
		s.addResult("Get Monthly Stats", false, "Failed to get monthly stats", err)
	} else {
		s.addResult("Get Monthly Stats", true, fmt.Sprintf("Got %d months of data", len(monthly)), nil)
	}
}

func (s *TestSuite) testBulbFeatures(ctx context.Context) {
	fmt.Printf("\n[Bulb Feature Tests]\n")

	// Test dimmable interface
	dimmable, ok := s.device.(device.Dimmable)
	if !ok {
		s.addResult("Dimmable Interface", false, "Device doesn't implement Dimmable", nil)
		return
	}

	if !dimmable.IsDimmable() {
		s.addResult("Dimmable", false, "Device reports not dimmable", nil)
	} else {
		s.addResult("Dimmable", true, "Device supports dimming", nil)

		// Save original brightness
		originalBrightness := dimmable.Brightness()
		s.addResult("Get Brightness", true, fmt.Sprintf("Current: %d%%", originalBrightness), nil)

		// Test setting brightness
		testBrightness := 50
		if originalBrightness == 50 {
			testBrightness = 75
		}

		if err := dimmable.SetBrightness(ctx, testBrightness); err != nil {
			s.addResult("Set Brightness", false, fmt.Sprintf("Failed to set to %d%%", testBrightness), err)
		} else {
			time.Sleep(500 * time.Millisecond)
			s.device.Update(ctx)
			if dimmable.Brightness() != testBrightness {
				s.addResult("Set Brightness", false,
					fmt.Sprintf("Expected %d%%, got %d%%", testBrightness, dimmable.Brightness()), nil)
			} else {
				s.addResult("Set Brightness", true, fmt.Sprintf("Set to %d%%", testBrightness), nil)
			}
		}

		// Restore original brightness
		dimmable.SetBrightness(ctx, originalBrightness)
		s.addResult("Restore Brightness", true, fmt.Sprintf("Restored to %d%%", originalBrightness), nil)
	}

	// Test colorable interface
	colorable, ok := s.device.(device.Colorable)
	if !ok {
		s.addResult("Colorable Interface", false, "Device doesn't implement Colorable", nil)
		return
	}

	if colorable.IsColor() {
		s.addResult("Color Support", true, "Device supports color", nil)

		// Save original HSV
		origH, origS, origV := colorable.HSV()
		s.addResult("Get HSV", true, fmt.Sprintf("H=%d, S=%d, V=%d", origH, origS, origV), nil)

		// Test setting HSV (use a distinct color)
		testH, testS, testV := 120, 80, 90 // Green
		if origH == 120 {
			testH = 240 // Blue instead
		}

		if err := colorable.SetHSV(ctx, testH, testS, testV); err != nil {
			s.addResult("Set HSV", false, fmt.Sprintf("Failed to set HSV(%d,%d,%d)", testH, testS, testV), err)
		} else {
			time.Sleep(500 * time.Millisecond)
			s.device.Update(ctx)
			h, sat, v := colorable.HSV()
			// Allow some tolerance in HSV values
			if abs(h-testH) > 5 || abs(sat-testS) > 5 || abs(v-testV) > 5 {
				s.addResult("Set HSV", false,
					fmt.Sprintf("Expected HSV(%d,%d,%d), got HSV(%d,%d,%d)", testH, testS, testV, h, sat, v), nil)
			} else {
				s.addResult("Set HSV", true, fmt.Sprintf("Set to HSV(%d,%d,%d)", testH, testS, testV), nil)
			}
		}

		// Restore original HSV
		colorable.SetHSV(ctx, origH, origS, origV)
		s.addResult("Restore HSV", true, fmt.Sprintf("Restored to HSV(%d,%d,%d)", origH, origS, origV), nil)
	} else {
		s.addResult("Color Support", true, "Device is tunable white (no color)", nil)
	}

	// Test color temperature (separate interface)
	tempCtrl, ok := s.device.(device.TemperatureControllable)
	if ok && tempCtrl.IsVariableColorTemp() {
		s.addResult("Variable Temp", true, "Device supports variable color temperature", nil)

		origTemp := tempCtrl.ColorTemp()
		minTemp, maxTemp := tempCtrl.ColorTempRange()
		s.addResult("Get Color Temp", true, fmt.Sprintf("Current: %dK (range: %d-%dK)", origTemp, minTemp, maxTemp), nil)

		// Test setting temperature (use middle of range)
		testTemp := (minTemp + maxTemp) / 2
		if abs(origTemp-testTemp) < 500 {
			testTemp = minTemp + 1000
		}

		if err := tempCtrl.SetColorTemp(ctx, testTemp); err != nil {
			s.addResult("Set Color Temp", false, fmt.Sprintf("Failed to set to %dK", testTemp), err)
		} else {
			time.Sleep(500 * time.Millisecond)
			s.device.Update(ctx)
			newTemp := tempCtrl.ColorTemp()
			if abs(newTemp-testTemp) > 100 {
				s.addResult("Set Color Temp", false,
					fmt.Sprintf("Expected %dK, got %dK", testTemp, newTemp), nil)
			} else {
				s.addResult("Set Color Temp", true, fmt.Sprintf("Set to %dK", testTemp), nil)
			}
		}

		// Restore original temperature
		tempCtrl.SetColorTemp(ctx, origTemp)
		s.addResult("Restore Color Temp", true, fmt.Sprintf("Restored to %dK", origTemp), nil)
	}
}

func (s *TestSuite) testPowerStripChildren(ctx context.Context, parent device.ParentDevice) {
	fmt.Printf("\n[Power Strip Child Tests]\n")

	children := parent.Children()
	s.addResult("Get Children", true, fmt.Sprintf("Found %d outlets", len(children)), nil)

	if len(children) == 0 {
		s.addResult("Child Count", false, "No children found", nil)
		return
	}

	// Test first child
	child := children[0]
	s.addResult("Child Info", true,
		fmt.Sprintf("Testing outlet 0: %s (ID: %s)", child.Alias(), child.DeviceID()), nil)

	// Save original state
	originalState := child.IsOn()
	s.addResult("Child State", true, fmt.Sprintf("Current state: %s", onOff(originalState)), nil)

	// Toggle child
	if err := child.TurnOff(ctx); err != nil {
		s.addResult("Child Turn Off", false, "Failed to turn off", err)
	} else {
		time.Sleep(500 * time.Millisecond)
		child.Update(ctx)
		if child.IsOn() {
			s.addResult("Child Turn Off", false, "Child still ON after TurnOff", nil)
		} else {
			s.addResult("Child Turn Off", true, "Outlet turned off", nil)
		}
	}

	if err := child.TurnOn(ctx); err != nil {
		s.addResult("Child Turn On", false, "Failed to turn on", err)
	} else {
		time.Sleep(500 * time.Millisecond)
		child.Update(ctx)
		if !child.IsOn() {
			s.addResult("Child Turn On", false, "Child still OFF after TurnOn", nil)
		} else {
			s.addResult("Child Turn On", true, "Outlet turned on", nil)
		}
	}

	// Restore original state
	if originalState {
		child.TurnOn(ctx)
	} else {
		child.TurnOff(ctx)
	}
	s.addResult("Child Restore", true, fmt.Sprintf("Restored to: %s", onOff(originalState)), nil)

	// Test child emeter if available
	if emeterChild, ok := child.(device.EmeterDevice); ok && emeterChild.HasEmeter() {
		data, err := emeterChild.GetEmeterRealtime(ctx)
		if err != nil {
			s.addResult("Child Emeter", false, "Failed to get child emeter", err)
		} else {
			s.addResult("Child Emeter", true,
				fmt.Sprintf("V=%.1fV, P=%.1fW", data.Voltage, data.Power), nil)
		}
	}
}

func onOff(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
