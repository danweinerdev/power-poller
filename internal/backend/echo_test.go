package backend

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/danweinerdev/power-poller/internal/metrics"
)

func TestEcho_New(t *testing.T) {
	t.Run("with writer", func(t *testing.T) {
		var buf bytes.Buffer
		e := NewEcho(&buf, nil)
		if e == nil {
			t.Fatal("NewEcho() returned nil")
		}
		if e.Name() != "echo" {
			t.Errorf("Name() = %q, want %q", e.Name(), "echo")
		}
	})

	t.Run("nil writer defaults to stdout", func(t *testing.T) {
		e := NewEcho(nil, nil)
		if e == nil {
			t.Fatal("NewEcho(nil) returned nil")
		}
		// writer should be os.Stdout
	})
}

func TestEcho_NewEchoStdout(t *testing.T) {
	e := NewEchoStdout(nil)
	if e == nil {
		t.Fatal("NewEchoStdout() returned nil")
	}
	if e.Name() != "echo" {
		t.Errorf("Name() = %q, want %q", e.Name(), "echo")
	}
}

func TestEcho_Initialize(t *testing.T) {
	var buf bytes.Buffer
	e := NewEcho(&buf, nil)

	ctx := context.Background()
	if err := e.Initialize(ctx); err != nil {
		t.Errorf("Initialize() error = %v", err)
	}

	if !e.Healthy() {
		t.Error("should be healthy after Initialize()")
	}
}

func TestEcho_Write(t *testing.T) {
	var buf bytes.Buffer
	e := NewEcho(&buf, nil)

	ctx := context.Background()
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	m := metrics.NewMetric("test_measurement").
		WithTag("device", "test_device").
		WithField("value", 123.45).
		WithTimestamp(time.Unix(1000000000, 0))

	if err := e.Write(ctx, []*metrics.Metric{m}); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "test_measurement") {
		t.Error("output should contain measurement name")
	}
	if !strings.Contains(output, "device=test_device") {
		t.Error("output should contain tags")
	}
	if !strings.Contains(output, "value=") {
		t.Error("output should contain field")
	}
}

func TestEcho_Write_EmptyBatch(t *testing.T) {
	var buf bytes.Buffer
	e := NewEcho(&buf, nil)

	ctx := context.Background()
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if err := e.Write(ctx, []*metrics.Metric{}); err != nil {
		t.Errorf("Write() empty batch error = %v", err)
	}

	if buf.Len() != 0 {
		t.Errorf("expected no output for empty batch, got %d bytes", buf.Len())
	}
}

func TestEcho_Write_MultipleBatch(t *testing.T) {
	var buf bytes.Buffer
	e := NewEcho(&buf, nil)

	ctx := context.Background()
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	batch := []*metrics.Metric{
		metrics.NewMetric("metric1").WithField("value", 1),
		metrics.NewMetric("metric2").WithField("value", 2),
		metrics.NewMetric("metric3").WithField("value", 3),
	}

	if err := e.Write(ctx, batch); err != nil {
		t.Errorf("Write() error = %v", err)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestEcho_Close(t *testing.T) {
	var buf bytes.Buffer
	e := NewEcho(&buf, nil)

	ctx := context.Background()
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if !e.Healthy() {
		t.Error("should be healthy before Close()")
	}

	if err := e.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if e.Healthy() {
		t.Error("should not be healthy after Close()")
	}
}

func TestEcho_Healthy(t *testing.T) {
	var buf bytes.Buffer
	e := NewEcho(&buf, nil)

	// Initially healthy (constructor sets healthy: true)
	if !e.Healthy() {
		t.Error("should be healthy initially")
	}

	ctx := context.Background()
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if !e.Healthy() {
		t.Error("should be healthy after Initialize()")
	}

	if err := e.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if e.Healthy() {
		t.Error("should not be healthy after Close()")
	}
}

// failWriter always returns an error on Write.
type failWriter struct{}

func (f *failWriter) Write(p []byte) (n int, err error) {
	return 0, errWriteFailed
}

var errWriteFailed = testError("write failed")

type testError string

func (e testError) Error() string { return string(e) }

func TestEcho_Write_Error(t *testing.T) {
	e := NewEcho(&failWriter{}, nil)

	ctx := context.Background()
	if err := e.Initialize(ctx); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	m := metrics.NewMetric("test").WithField("value", 1)

	err := e.Write(ctx, []*metrics.Metric{m})
	if err == nil {
		t.Error("expected error from failing writer")
	}
}
