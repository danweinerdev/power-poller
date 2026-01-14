package protocol

import (
	"log"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

// loggingRoundTripper wraps an http.RoundTripper to provide device context
// for HTTP-level logging, particularly for Go's internal "Unsolicited response" warnings.
type loggingRoundTripper struct {
	transport http.RoundTripper
	host      string
	logger    *slog.Logger
}

// newLoggingRoundTripper creates a RoundTripper that logs HTTP errors with device context.
func newLoggingRoundTripper(transport http.RoundTripper, host string, logger *slog.Logger) *loggingRoundTripper {
	return &loggingRoundTripper{
		transport: transport,
		host:      host,
		logger:    logger,
	}
}

// RoundTrip implements http.RoundTripper.
func (l *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Register this device as active before the request
	registerActiveDevice(l.host)
	defer unregisterActiveDevice(l.host)

	resp, err := l.transport.RoundTrip(req)
	if err != nil {
		l.logger.Debug("HTTP request error",
			"host", l.host,
			"url", req.URL.String(),
			"error", err,
		)
	}
	return resp, err
}

// activeDevice tracks a device that recently made an HTTP request.
type activeDevice struct {
	host      string
	timestamp time.Time
}

// deviceLogWriter intercepts log output and adds device context for known message patterns.
type deviceLogWriter struct {
	mu            sync.RWMutex
	activeDevices map[string]time.Time // host -> last activity time
	logger        *slog.Logger
}

var (
	globalLogWriter     *deviceLogWriter
	globalLogWriterOnce sync.Once
)

// InitDeviceLogger sets up a global log interceptor that adds device context
// to Go's internal HTTP log messages.
func InitDeviceLogger(logger *slog.Logger) {
	globalLogWriterOnce.Do(func() {
		globalLogWriter = &deviceLogWriter{
			activeDevices: make(map[string]time.Time),
			logger:        logger,
		}
		log.SetOutput(globalLogWriter)
		log.SetFlags(0) // We handle formatting in slog
	})
}

// registerActiveDevice marks a device as currently making an HTTP request.
func registerActiveDevice(host string) {
	if globalLogWriter == nil {
		return
	}
	globalLogWriter.mu.Lock()
	defer globalLogWriter.mu.Unlock()
	globalLogWriter.activeDevices[host] = time.Now()
}

// unregisterActiveDevice marks a device as no longer making an HTTP request.
// We keep it in the map briefly to catch async log messages.
func unregisterActiveDevice(host string) {
	// Don't remove immediately - the log message might come slightly after
	// the RoundTrip returns. The cleanup happens in findActiveDevices.
}

// Write implements io.Writer to intercept log messages.
func (w *deviceLogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	// Check if this is the "Unsolicited response" message we want to enhance
	if strings.Contains(msg, "Unsolicited response") {
		devices := w.findActiveDevices()
		if len(devices) == 1 {
			w.logger.Debug("HTTP idle channel received unexpected response",
				"host", devices[0],
				"detail", msg,
			)
		} else if len(devices) > 1 {
			w.logger.Debug("HTTP idle channel received unexpected response",
				"possible_hosts", devices,
				"detail", msg,
			)
		} else {
			// No active devices, log without context
			w.logger.Debug("HTTP idle channel received unexpected response",
				"detail", msg,
			)
		}
		return len(p), nil
	}

	// For other net/http messages, log at debug level
	w.logger.Debug(msg, "source", "net/http")
	return len(p), nil
}

// findActiveDevices returns devices that were active in the last few seconds.
func (w *deviceLogWriter) findActiveDevices() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Second)
	var active []string
	var stale []string

	for host, ts := range w.activeDevices {
		if ts.After(cutoff) {
			active = append(active, host)
		} else {
			stale = append(stale, host)
		}
	}

	// Clean up stale entries
	for _, host := range stale {
		delete(w.activeDevices, host)
	}

	return active
}
