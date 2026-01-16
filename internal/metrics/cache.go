package metrics

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CacheConfig configures the file-backed metrics cache.
type CacheConfig struct {
	// Path is the file path for the cache. If empty, caching is disabled.
	Path string
	// MaxMetrics is the maximum number of metrics to cache (0 = unlimited).
	MaxMetrics int
	// MaxBytes is the maximum cache file size in bytes (0 = unlimited).
	MaxBytes int64
	// Logger for cache operations.
	Logger *slog.Logger
}

// DefaultCacheConfig returns sensible cache defaults.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Path:       "",               // Disabled by default
		MaxMetrics: 10000,            // 10k metrics max
		MaxBytes:   10 * 1024 * 1024, // 10MB max
		Logger:     slog.Default(),
	}
}

// Cache provides file-backed storage for metrics that couldn't be sent.
type Cache struct {
	cfg    CacheConfig
	logger *slog.Logger

	mu      sync.Mutex
	count   int   // Number of cached metrics
	size    int64 // Current file size in bytes
	enabled bool
}

// cachedMetric is the JSON structure for cached metrics.
type cachedMetric struct {
	Measurement string                 `json:"measurement"`
	Tags        map[string]string      `json:"tags"`
	Fields      map[string]interface{} `json:"fields"`
	Timestamp   time.Time              `json:"timestamp"`
}

// NewCache creates a new file-backed metrics cache.
func NewCache(cfg CacheConfig) *Cache {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	c := &Cache{
		cfg:     cfg,
		logger:  cfg.Logger,
		enabled: cfg.Path != "",
	}

	if c.enabled {
		// Ensure cache directory exists
		dir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			c.logger.Error("failed to create cache directory, caching disabled",
				"path", dir, "error", err)
			c.enabled = false
			return c
		}

		// Get current cache stats
		if info, err := os.Stat(cfg.Path); err == nil {
			c.size = info.Size()
			c.count = c.countLines()
			c.logger.Info("loaded existing metrics cache",
				"path", cfg.Path,
				"metrics", c.count,
				"size_bytes", c.size)
		}
	}

	return c
}

// Enabled returns true if caching is enabled.
func (c *Cache) Enabled() bool {
	return c.enabled
}

// Count returns the number of cached metrics.
func (c *Cache) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.count
}

// Append adds metrics to the cache file.
func (c *Cache) Append(metrics []*Metric) error {
	if !c.enabled || len(metrics) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check limits before appending
	if c.cfg.MaxMetrics > 0 && c.count >= c.cfg.MaxMetrics {
		c.logger.Warn("cache full, dropping metrics",
			"cached", c.count,
			"max", c.cfg.MaxMetrics,
			"dropped", len(metrics))
		return fmt.Errorf("cache full: %d metrics (max %d)", c.count, c.cfg.MaxMetrics)
	}

	if c.cfg.MaxBytes > 0 && c.size >= c.cfg.MaxBytes {
		c.logger.Warn("cache size limit reached, dropping metrics",
			"size", c.size,
			"max", c.cfg.MaxBytes,
			"dropped", len(metrics))
		return fmt.Errorf("cache size limit reached: %d bytes (max %d)", c.size, c.cfg.MaxBytes)
	}

	// Open file for appending
	f, err := os.OpenFile(c.cfg.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open cache file: %w", err)
	}
	defer f.Close()

	// Write metrics as JSON lines
	encoder := json.NewEncoder(f)
	written := 0
	for _, m := range metrics {
		// Check limits per metric
		if c.cfg.MaxMetrics > 0 && c.count+written >= c.cfg.MaxMetrics {
			break
		}

		cm := cachedMetric{
			Measurement: m.Measurement,
			Tags:        m.Tags,
			Fields:      m.Fields,
			Timestamp:   m.Timestamp,
		}

		if err := encoder.Encode(cm); err != nil {
			c.logger.Error("failed to encode metric", "error", err)
			continue
		}
		written++
	}

	// Update stats
	if info, err := f.Stat(); err == nil {
		c.size = info.Size()
	}
	c.count += written

	c.logger.Debug("cached metrics",
		"written", written,
		"total_cached", c.count)

	return nil
}

// Load reads all cached metrics from the file.
func (c *Cache) Load() ([]*Metric, error) {
	if !c.enabled {
		return nil, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	f, err := os.Open(c.cfg.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open cache file: %w", err)
	}
	defer f.Close()

	var metrics []*Metric
	scanner := bufio.NewScanner(f)
	// Increase buffer size for large lines
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		var cm cachedMetric
		if err := json.Unmarshal(scanner.Bytes(), &cm); err != nil {
			c.logger.Warn("failed to parse cached metric",
				"line", lineNum,
				"error", err)
			continue
		}

		m := &Metric{
			Measurement: cm.Measurement,
			Tags:        cm.Tags,
			Fields:      cm.Fields,
			Timestamp:   cm.Timestamp,
		}
		metrics = append(metrics, m)
	}

	if err := scanner.Err(); err != nil {
		return metrics, fmt.Errorf("error reading cache file: %w", err)
	}

	c.logger.Info("loaded cached metrics", "count", len(metrics))
	return metrics, nil
}

// Clear removes all cached metrics.
func (c *Cache) Clear() error {
	if !c.enabled {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.Remove(c.cfg.Path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	c.count = 0
	c.size = 0
	c.logger.Info("cleared metrics cache")
	return nil
}

// FlushTo attempts to send cached metrics to a backend.
// Returns the number of metrics successfully flushed.
func (c *Cache) FlushTo(ctx context.Context, b Backend) (int, error) {
	if !c.enabled {
		return 0, nil
	}

	metrics, err := c.Load()
	if err != nil {
		return 0, fmt.Errorf("failed to load cache: %w", err)
	}

	if len(metrics) == 0 {
		return 0, nil
	}

	c.logger.Info("flushing cached metrics to backend",
		"backend", b.Name(),
		"count", len(metrics))

	// Write to backend
	if err := b.Write(ctx, metrics); err != nil {
		return 0, fmt.Errorf("failed to flush cache to %s: %w", b.Name(), err)
	}

	// Clear cache on success
	if err := c.Clear(); err != nil {
		c.logger.Error("failed to clear cache after flush", "error", err)
	}

	return len(metrics), nil
}

// countLines counts the number of lines in the cache file.
func (c *Cache) countLines() int {
	f, err := os.Open(c.cfg.Path)
	if err != nil {
		return 0
	}
	defer f.Close()

	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		count++
	}
	return count
}
