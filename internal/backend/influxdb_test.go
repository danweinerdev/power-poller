package backend

import (
	"context"
	"testing"

	"github.com/danweinerdev/go-power-poller/internal/config"
	"github.com/danweinerdev/go-power-poller/internal/metrics"
)

func TestInfluxDB_New(t *testing.T) {
	cfg := config.InfluxDBConfig{
		Server: "localhost",
		Port:   8086,
		Token:  "test-token",
		Org:    "test-org",
		Bucket: "test-bucket",
	}
	i := NewInfluxDB(cfg, nil)
	if i == nil {
		t.Fatal("NewInfluxDB() returned nil")
	}
	if i.Name() != "influxdb" {
		t.Errorf("Name() = %q, want %q", i.Name(), "influxdb")
	}
}

func TestInfluxDB_Healthy_BeforeInit(t *testing.T) {
	cfg := config.InfluxDBConfig{
		Server: "localhost",
		Port:   8086,
	}
	i := NewInfluxDB(cfg, nil)

	if i.Healthy() {
		t.Error("should not be healthy before Initialize()")
	}
}

func TestInfluxDB_Write_NotInitialized(t *testing.T) {
	cfg := config.InfluxDBConfig{
		Server: "localhost",
		Port:   8086,
	}
	i := NewInfluxDB(cfg, nil)

	batch := []*metrics.Metric{
		metrics.NewMetric("test").WithField("value", 1),
	}

	ctx := context.Background()
	err := i.Write(ctx, batch)
	if err == nil {
		t.Error("expected error when not initialized")
	}
}

func TestInfluxDB_Write_EmptyBatch(t *testing.T) {
	cfg := config.InfluxDBConfig{
		Server: "localhost",
		Port:   8086,
	}
	i := NewInfluxDB(cfg, nil)

	ctx := context.Background()
	// Empty batch should return nil even without initialization
	if err := i.Write(ctx, []*metrics.Metric{}); err != nil {
		t.Errorf("Write() empty batch error = %v", err)
	}
}

func TestInfluxDB_Close_NotInitialized(t *testing.T) {
	cfg := config.InfluxDBConfig{
		Server: "localhost",
		Port:   8086,
	}
	i := NewInfluxDB(cfg, nil)

	// Close without Initialize should be safe
	if err := i.Close(); err != nil {
		t.Errorf("Close() without Initialize error = %v", err)
	}

	if i.Healthy() {
		t.Error("should not be healthy after Close()")
	}
}

func TestInfluxDB_Initialize_InvalidServer(t *testing.T) {
	cfg := config.InfluxDBConfig{
		Server: "invalid.host.that.does.not.exist.local",
		Port:   8086,
		Token:  "test-token",
		Org:    "test-org",
		Bucket: "test-bucket",
	}
	i := NewInfluxDB(cfg, nil)

	ctx := context.Background()
	err := i.Initialize(ctx)

	// Should fail to connect
	if err == nil {
		i.Close()
		t.Error("expected error for invalid server")
	}
}
