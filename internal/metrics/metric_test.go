package metrics

import (
	"strings"
	"testing"
	"time"
)

func TestNewMetric(t *testing.T) {
	m := NewMetric("power_metrics")
	if m.Measurement != "power_metrics" {
		t.Errorf("measurement = %q, want %q", m.Measurement, "power_metrics")
	}
	if m.Tags == nil {
		t.Error("tags should be initialized")
	}
	if m.Fields == nil {
		t.Error("fields should be initialized")
	}
	if m.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestMetric_WithMethods(t *testing.T) {
	ts := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	m := NewMetric("test").
		WithTag("device", "plug1").
		WithTags(map[string]string{"location": "office", "room": "A1"}).
		WithField("power", 100.5).
		WithFields(map[string]interface{}{"voltage": 120.0, "current": 0.84}).
		WithTimestamp(ts)

	if m.Tags["device"] != "plug1" {
		t.Errorf("tag device = %q, want %q", m.Tags["device"], "plug1")
	}
	if m.Tags["location"] != "office" {
		t.Errorf("tag location = %q, want %q", m.Tags["location"], "office")
	}
	if m.Tags["room"] != "A1" {
		t.Errorf("tag room = %q, want %q", m.Tags["room"], "A1")
	}
	if m.Fields["power"] != 100.5 {
		t.Errorf("field power = %v, want 100.5", m.Fields["power"])
	}
	if m.Fields["voltage"] != 120.0 {
		t.Errorf("field voltage = %v, want 120.0", m.Fields["voltage"])
	}
	if m.Fields["current"] != 0.84 {
		t.Errorf("field current = %v, want 0.84", m.Fields["current"])
	}
	if !m.Timestamp.Equal(ts) {
		t.Errorf("timestamp = %v, want %v", m.Timestamp, ts)
	}
}

func TestMetric_Clone(t *testing.T) {
	original := NewMetric("test").
		WithTag("device", "plug1").
		WithField("power", 100.5)

	clone := original.Clone()

	// Modify original
	original.Tags["device"] = "plug2"
	original.Fields["power"] = 200.0

	// Clone should be unchanged
	if clone.Tags["device"] != "plug1" {
		t.Errorf("clone tag device = %q, want %q (should be independent)", clone.Tags["device"], "plug1")
	}
	if clone.Fields["power"] != 100.5 {
		t.Errorf("clone field power = %v, want 100.5 (should be independent)", clone.Fields["power"])
	}
}

func TestMetric_Validate(t *testing.T) {
	tests := []struct {
		name    string
		metric  *Metric
		wantErr bool
	}{
		{
			name:    "valid metric",
			metric:  NewMetric("test").WithField("value", 1),
			wantErr: false,
		},
		{
			name:    "empty measurement",
			metric:  NewMetric("").WithField("value", 1),
			wantErr: true,
		},
		{
			name:    "no fields",
			metric:  NewMetric("test"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.metric.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMetric_ToLineProtocol(t *testing.T) {
	ts := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	m := NewMetric("power").
		WithTag("device", "plug1").
		WithField("power", 100.5).
		WithField("on", true).
		WithField("count", 42).
		WithField("name", "test").
		WithTimestamp(ts)

	line := m.ToLineProtocol()

	// Check measurement and tag
	if !strings.HasPrefix(line, "power,device=plug1 ") {
		t.Errorf("line protocol prefix wrong: %s", line)
	}

	// Check fields are present (order may vary)
	if !strings.Contains(line, "power=100.5") {
		t.Errorf("line protocol missing power field: %s", line)
	}
	if !strings.Contains(line, "on=true") {
		t.Errorf("line protocol missing on field: %s", line)
	}
	if !strings.Contains(line, "count=42i") {
		t.Errorf("line protocol missing count field: %s", line)
	}
	if !strings.Contains(line, `name="test"`) {
		t.Errorf("line protocol missing name field: %s", line)
	}

	// Check timestamp
	expectedTS := "1704110400000000000"
	if !strings.HasSuffix(line, expectedTS) {
		t.Errorf("line protocol timestamp wrong: %s, want suffix %s", line, expectedTS)
	}
}

func TestMetric_ToLineProtocol_Escaping(t *testing.T) {
	m := NewMetric("my measurement").
		WithTag("tag key", "tag value").
		WithField("field", 1.0).
		WithTimestamp(time.Unix(0, 0))

	line := m.ToLineProtocol()

	// Spaces should be escaped in measurement and tags
	if !strings.Contains(line, "my\\ measurement") {
		t.Errorf("measurement not properly escaped: %s", line)
	}
	if !strings.Contains(line, "tag\\ key=tag\\ value") {
		t.Errorf("tag not properly escaped: %s", line)
	}
}

func TestFormatFieldValue(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{float64(123.456), "123.456"},
		{float32(1.5), "1.5"},
		{int(42), "42i"},
		{int64(100), "100i"},
		{uint(50), "50u"},
		{uint64(200), "200u"},
		{true, "true"},
		{false, "false"},
		{"hello", `"hello"`},
	}

	for _, tt := range tests {
		got := formatFieldValue(tt.value)
		if got != tt.expected {
			t.Errorf("formatFieldValue(%v) = %q, want %q", tt.value, got, tt.expected)
		}
	}
}

func TestEmeterToMetric(t *testing.T) {
	tags := map[string]string{"device": "plug1"}
	m := EmeterToMetric("power_metrics", tags, 120.5, 0.84, 101.22, 1234.5)

	if m.Measurement != "power_metrics" {
		t.Errorf("measurement = %q, want %q", m.Measurement, "power_metrics")
	}
	if m.Tags["device"] != "plug1" {
		t.Errorf("tag device = %q, want %q", m.Tags["device"], "plug1")
	}
	if m.Fields["voltage"] != 120.5 {
		t.Errorf("voltage = %v, want 120.5", m.Fields["voltage"])
	}
	if m.Fields["current"] != 0.84 {
		t.Errorf("current = %v, want 0.84", m.Fields["current"])
	}
	if m.Fields["power"] != 101.22 {
		t.Errorf("power = %v, want 101.22", m.Fields["power"])
	}
	if m.Fields["total"] != 1234.5 {
		t.Errorf("total = %v, want 1234.5", m.Fields["total"])
	}
}
