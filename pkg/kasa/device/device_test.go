package device_test

import (
	"context"
	"testing"
	"time"

	"github.com/danweinerdev/go-power-poller/pkg/kasa/device"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/protocol"
	"github.com/danweinerdev/go-power-poller/pkg/kasa/types"
	"github.com/danweinerdev/go-power-poller/pkg/mockdevice"
)

func TestLoad_Plug(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithDeviceType(mockdevice.DeviceTypePlug),
		mockdevice.WithEmeter(true),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := device.Load(ctx, mock.Address(),
		device.WithTransportOptions(protocol.WithTimeout(2*time.Second)),
	)
	if err != nil {
		t.Fatalf("failed to load device: %v", err)
	}
	defer dev.Close()

	if dev.Type() != types.DeviceTypePlug {
		t.Errorf("type = %v, want Plug", dev.Type())
	}

	if dev.Alias() != "Mock Device" {
		t.Errorf("alias = %q, want %q", dev.Alias(), "Mock Device")
	}
}

func TestPlug_TurnOnOff(t *testing.T) {
	mock := mockdevice.New(mockdevice.WithDeviceType(mockdevice.DeviceTypePlug))
	// Set initial state to off
	mock.IsOn = false
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := device.Load(ctx, mock.Address())
	if err != nil {
		t.Fatalf("failed to load device: %v", err)
	}
	defer dev.Close()

	// Initially off
	if dev.IsOn() {
		t.Error("device should be off initially")
	}

	// Turn on
	if err := dev.TurnOn(ctx); err != nil {
		t.Fatalf("failed to turn on: %v", err)
	}
	if !dev.IsOn() {
		t.Error("device should be on after TurnOn")
	}

	// Turn off
	if err := dev.TurnOff(ctx); err != nil {
		t.Fatalf("failed to turn off: %v", err)
	}
	if dev.IsOn() {
		t.Error("device should be off after TurnOff")
	}
}

func TestPlug_Emeter(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithDeviceType(mockdevice.DeviceTypePlug),
		mockdevice.WithEmeter(true),
		mockdevice.WithEmeterValues(120.5, 0.84, 101.22, 1234.56),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := device.Load(ctx, mock.Address())
	if err != nil {
		t.Fatalf("failed to load device: %v", err)
	}
	defer dev.Close()

	emeterDev, ok := dev.(device.EmeterDevice)
	if !ok {
		t.Fatal("device should implement EmeterDevice")
	}

	if !emeterDev.HasEmeter() {
		t.Fatal("device should have emeter")
	}

	data, err := emeterDev.GetEmeterRealtime(ctx)
	if err != nil {
		t.Fatalf("failed to get emeter: %v", err)
	}

	if data.Voltage != 120.5 {
		t.Errorf("voltage = %v, want 120.5", data.Voltage)
	}
	if data.Current != 0.84 {
		t.Errorf("current = %v, want 0.84", data.Current)
	}
	if data.Power != 101.22 {
		t.Errorf("power = %v, want 101.22", data.Power)
	}
	// Total is returned as total_wh from mock (1234 Wh), normalized to kWh (~1.234)
	if data.Total < 1.23 || data.Total > 1.24 {
		t.Errorf("total = %v, want ~1.234 kWh", data.Total)
	}
}

func TestBulb_Brightness(t *testing.T) {
	mock := mockdevice.New(mockdevice.WithDeviceType(mockdevice.DeviceTypeBulb))
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := device.Load(ctx, mock.Address())
	if err != nil {
		t.Fatalf("failed to load device: %v", err)
	}
	defer dev.Close()

	if dev.Type() != types.DeviceTypeBulb {
		t.Errorf("type = %v, want Bulb", dev.Type())
	}

	dimmable, ok := dev.(device.Dimmable)
	if !ok {
		t.Fatal("bulb should implement Dimmable")
	}

	if !dimmable.IsDimmable() {
		t.Fatal("bulb should be dimmable")
	}

	// Set brightness
	if err := dimmable.SetBrightness(ctx, 75); err != nil {
		t.Fatalf("failed to set brightness: %v", err)
	}

	if dimmable.Brightness() != 75 {
		t.Errorf("brightness = %d, want 75", dimmable.Brightness())
	}
}

func TestBulb_HSV(t *testing.T) {
	mock := mockdevice.New(mockdevice.WithDeviceType(mockdevice.DeviceTypeBulb))
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := device.Load(ctx, mock.Address())
	if err != nil {
		t.Fatalf("failed to load device: %v", err)
	}
	defer dev.Close()

	colorable, ok := dev.(device.Colorable)
	if !ok {
		t.Fatal("bulb should implement Colorable")
	}

	if !colorable.IsColor() {
		t.Fatal("bulb should support color")
	}

	// Set HSV
	if err := colorable.SetHSV(ctx, 180, 50, 80); err != nil {
		t.Fatalf("failed to set HSV: %v", err)
	}

	h, s, b := colorable.HSV()
	if h != 180 || s != 50 || b != 80 {
		t.Errorf("HSV = (%d, %d, %d), want (180, 50, 80)", h, s, b)
	}
}

func TestPowerStrip_Children(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithDeviceType(mockdevice.DeviceTypePowerStrip),
		mockdevice.WithChildren(3),
		mockdevice.WithEmeter(true),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dev, err := device.Load(ctx, mock.Address())
	if err != nil {
		t.Fatalf("failed to load device: %v", err)
	}
	defer dev.Close()

	if dev.Type() != types.DeviceTypePowerStrip {
		t.Errorf("type = %v, want PowerStrip", dev.Type())
	}

	parent, ok := dev.(device.ParentDevice)
	if !ok {
		t.Fatal("power strip should implement ParentDevice")
	}

	if !parent.HasChildren() {
		t.Fatal("power strip should have children")
	}

	children := parent.Children()
	if len(children) != 3 {
		t.Errorf("children count = %d, want 3", len(children))
	}

	// Control individual outlet
	child := children[0]
	if err := child.TurnOn(ctx); err != nil {
		t.Fatalf("failed to turn on outlet: %v", err)
	}
	if !child.IsOn() {
		t.Error("outlet should be on")
	}

	if err := child.TurnOff(ctx); err != nil {
		t.Fatalf("failed to turn off outlet: %v", err)
	}
	if child.IsOn() {
		t.Error("outlet should be off")
	}
}

func TestLoadMultiple(t *testing.T) {
	mock1 := mockdevice.New(mockdevice.WithDeviceType(mockdevice.DeviceTypePlug))
	mock2 := mockdevice.New(mockdevice.WithDeviceType(mockdevice.DeviceTypeBulb))

	if err := mock1.Start(); err != nil {
		t.Fatalf("failed to start mock1: %v", err)
	}
	defer mock1.Stop()

	if err := mock2.Start(); err != nil {
		t.Fatalf("failed to start mock2: %v", err)
	}
	defer mock2.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	devices, errors := device.LoadMultiple(ctx, []string{mock1.Address(), mock2.Address()})

	for i, err := range errors {
		if err != nil {
			t.Errorf("device %d error: %v", i, err)
		}
	}

	if len(devices) != 2 {
		t.Fatalf("devices count = %d, want 2", len(devices))
	}

	// Close all
	for _, d := range devices {
		if d != nil {
			d.Close()
		}
	}
}

func TestDevice_Timeout(t *testing.T) {
	mock := mockdevice.New(
		mockdevice.WithDeviceType(mockdevice.DeviceTypePlug),
		mockdevice.WithErrorBehavior(mockdevice.ErrorBehavior{
			ResponseDelay: 500 * time.Millisecond,
		}),
	)
	if err := mock.Start(); err != nil {
		t.Fatalf("failed to start mock: %v", err)
	}
	defer mock.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := device.Load(ctx, mock.Address())
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
