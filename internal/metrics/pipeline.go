package metrics

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Backend defines the interface for metric storage backends.
type Backend interface {
	// Name returns the backend name for logging.
	Name() string

	// Initialize sets up the backend connection.
	Initialize(ctx context.Context) error

	// Write sends a batch of metrics to the backend.
	Write(ctx context.Context, metrics []*Metric) error

	// Close cleanly shuts down the backend.
	Close() error

	// Healthy returns true if the backend is operational.
	Healthy() bool
}

// Pipeline manages metric batching and delivery to backends.
type Pipeline struct {
	backends        []Backend
	batchSize       int
	flushInterval   time.Duration
	retryAttempts   int
	retryDelay      time.Duration
	recoverInterval time.Duration
	cache           *Cache

	mu          sync.Mutex
	buffer      []*Metric
	lastAttempt map[string]time.Time // backend name -> last attempt time
	done        chan struct{}
	wg          sync.WaitGroup
	logger      *slog.Logger
}

// PipelineConfig configures the metric pipeline.
type PipelineConfig struct {
	BatchSize       int
	FlushInterval   time.Duration
	RetryAttempts   int
	RetryDelay      time.Duration
	RecoverInterval time.Duration // How often to retry unhealthy backends
	CachePath       string        // Path for file-backed cache (empty = disabled)
	CacheMaxMetrics int           // Max metrics to cache (0 = default 10000)
	Logger          *slog.Logger
}

// DefaultPipelineConfig returns sensible pipeline defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		BatchSize:       10,
		FlushInterval:   10 * time.Second,
		RetryAttempts:   3,
		RetryDelay:      1 * time.Second,
		RecoverInterval: 30 * time.Second,
		Logger:          slog.Default(),
	}
}

// NewPipeline creates a new metric pipeline.
func NewPipeline(cfg PipelineConfig) *Pipeline {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 10
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 10 * time.Second
	}
	if cfg.RetryAttempts <= 0 {
		cfg.RetryAttempts = 3
	}
	if cfg.RetryDelay <= 0 {
		cfg.RetryDelay = 1 * time.Second
	}
	if cfg.RecoverInterval <= 0 {
		cfg.RecoverInterval = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	// Create cache if path is configured
	var cache *Cache
	if cfg.CachePath != "" {
		cacheCfg := CacheConfig{
			Path:       cfg.CachePath,
			MaxMetrics: cfg.CacheMaxMetrics,
			Logger:     cfg.Logger,
		}
		if cacheCfg.MaxMetrics == 0 {
			cacheCfg.MaxMetrics = 10000
		}
		cache = NewCache(cacheCfg)
	}

	return &Pipeline{
		backends:        make([]Backend, 0),
		batchSize:       cfg.BatchSize,
		flushInterval:   cfg.FlushInterval,
		retryAttempts:   cfg.RetryAttempts,
		retryDelay:      cfg.RetryDelay,
		recoverInterval: cfg.RecoverInterval,
		cache:           cache,
		buffer:          make([]*Metric, 0, cfg.BatchSize),
		lastAttempt:     make(map[string]time.Time),
		done:            make(chan struct{}),
		logger:          cfg.Logger,
	}
}

// AddBackend adds a backend to the pipeline.
func (p *Pipeline) AddBackend(b Backend) {
	p.backends = append(p.backends, b)
}

// Start begins the background flush goroutine.
func (p *Pipeline) Start(ctx context.Context) error {
	// Initialize all backends
	for _, b := range p.backends {
		if err := b.Initialize(ctx); err != nil {
			return err
		}
		p.logger.Info("backend initialized", "backend", b.Name())
	}

	// Flush any cached metrics from previous runs
	if p.cache != nil && p.cache.Enabled() && p.cache.Count() > 0 {
		p.logger.Info("found cached metrics from previous run", "count", p.cache.Count())
		p.flushCache(ctx)
	}

	// Start periodic flush
	p.wg.Add(1)
	go p.flushLoop(ctx)

	return nil
}

// Stop shuts down the pipeline, flushing remaining metrics.
func (p *Pipeline) Stop(ctx context.Context) error {
	close(p.done)
	p.wg.Wait()

	// Final flush
	if err := p.Flush(ctx); err != nil {
		p.logger.Error("final flush failed", "error", err)
	}

	// Close all backends
	var lastErr error
	for _, b := range p.backends {
		if err := b.Close(); err != nil {
			p.logger.Error("backend close failed", "backend", b.Name(), "error", err)
			lastErr = err
		}
	}

	return lastErr
}

// Push adds a metric to the pipeline.
func (p *Pipeline) Push(m *Metric) {
	if err := m.Validate(); err != nil {
		p.logger.Warn("invalid metric dropped", "error", err)
		return
	}

	p.mu.Lock()
	p.buffer = append(p.buffer, m)
	shouldFlush := len(p.buffer) >= p.batchSize
	p.mu.Unlock()

	if shouldFlush {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := p.Flush(ctx); err != nil {
				p.logger.Error("batch flush failed", "error", err)
			}
		}()
	}
}

// Flush sends all buffered metrics to backends.
func (p *Pipeline) Flush(ctx context.Context) error {
	p.mu.Lock()
	if len(p.buffer) == 0 {
		p.mu.Unlock()
		return nil
	}
	batch := p.buffer
	p.buffer = make([]*Metric, 0, p.batchSize)
	p.mu.Unlock()

	p.logger.Debug("flushing metrics", "count", len(batch))

	var lastErr error
	anySuccess := false

	for _, b := range p.backends {
		wasUnhealthy := !b.Healthy()

		if wasUnhealthy {
			// Check if we should attempt recovery
			if !p.shouldAttemptRecovery(b.Name()) {
				p.logger.Debug("skipping unhealthy backend, waiting for recovery interval",
					"backend", b.Name())
				continue
			}
			p.logger.Info("attempting recovery for unhealthy backend", "backend", b.Name())
		}

		p.recordAttempt(b.Name())

		if err := p.writeWithRetry(ctx, b, batch); err != nil {
			p.logger.Error("backend write failed", "backend", b.Name(), "error", err)
			lastErr = err
		} else {
			anySuccess = true
			// If backend recovered, try to flush cached metrics
			if wasUnhealthy && p.cache != nil && p.cache.Count() > 0 {
				p.logger.Info("backend recovered, flushing cached metrics", "backend", b.Name())
				if flushed, err := p.cache.FlushTo(ctx, b); err != nil {
					p.logger.Error("failed to flush cache to recovered backend",
						"backend", b.Name(), "error", err)
				} else if flushed > 0 {
					p.logger.Info("flushed cached metrics to recovered backend",
						"backend", b.Name(), "count", flushed)
				}
			}
		}
	}

	// If all backends failed, cache the metrics
	if !anySuccess && lastErr != nil && p.cache != nil {
		p.logger.Warn("all backends failed, caching metrics", "count", len(batch))
		if err := p.cache.Append(batch); err != nil {
			p.logger.Error("failed to cache metrics", "error", err)
		}
	}

	return lastErr
}

// flushCache attempts to flush cached metrics to any healthy backend.
func (p *Pipeline) flushCache(ctx context.Context) {
	if p.cache == nil || !p.cache.Enabled() || p.cache.Count() == 0 {
		return
	}

	for _, b := range p.backends {
		if !b.Healthy() {
			continue
		}

		flushed, err := p.cache.FlushTo(ctx, b)
		if err != nil {
			p.logger.Error("failed to flush cache on startup",
				"backend", b.Name(), "error", err)
			continue
		}

		if flushed > 0 {
			p.logger.Info("flushed cached metrics on startup",
				"backend", b.Name(), "count", flushed)
			return // Successfully flushed to one backend
		}
	}

	p.logger.Warn("no healthy backends available to flush cache")
}

// shouldAttemptRecovery checks if enough time has passed to retry an unhealthy backend.
func (p *Pipeline) shouldAttemptRecovery(name string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	last, ok := p.lastAttempt[name]
	if !ok {
		return true // Never attempted, try it
	}
	return time.Since(last) >= p.recoverInterval
}

// recordAttempt records when we last attempted to write to a backend.
func (p *Pipeline) recordAttempt(name string) {
	p.mu.Lock()
	p.lastAttempt[name] = time.Now()
	p.mu.Unlock()
}

// writeWithRetry attempts to write to a backend with retries.
func (p *Pipeline) writeWithRetry(ctx context.Context, b Backend, metrics []*Metric) error {
	var lastErr error
	for attempt := 1; attempt <= p.retryAttempts; attempt++ {
		err := b.Write(ctx, metrics)
		if err == nil {
			return nil
		}

		lastErr = err
		if attempt < p.retryAttempts {
			p.logger.Warn("write failed, retrying",
				"backend", b.Name(),
				"attempt", attempt,
				"error", err,
			)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(p.retryDelay):
			}
		}
	}
	return lastErr
}

// flushLoop periodically flushes the buffer.
func (p *Pipeline) flushLoop(ctx context.Context) {
	defer p.wg.Done()
	ticker := time.NewTicker(p.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := p.Flush(ctx); err != nil {
				p.logger.Error("periodic flush failed", "error", err)
			}
		}
	}
}

// BufferLen returns the current buffer length.
func (p *Pipeline) BufferLen() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.buffer)
}

// BackendCount returns the number of configured backends.
func (p *Pipeline) BackendCount() int {
	return len(p.backends)
}
