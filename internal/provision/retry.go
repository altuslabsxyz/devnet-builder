package provision

import (
	"context"
	"fmt"
	"time"

	"github.com/b-harvest/devnet-builder/internal/output"
)

const (
	// DefaultMaxRetries is the default number of retry attempts.
	DefaultMaxRetries = 3
	// DefaultInitialBackoff is the initial backoff duration.
	DefaultInitialBackoff = 1 * time.Second
	// DefaultMaxBackoff is the maximum backoff duration.
	DefaultMaxBackoff = 8 * time.Second
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Logger         *output.Logger
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:     DefaultMaxRetries,
		InitialBackoff: DefaultInitialBackoff,
		MaxBackoff:     DefaultMaxBackoff,
		Logger:         output.DefaultLogger,
	}
}

// RetryableFunc is a function that can be retried.
type RetryableFunc func() error

// WithRetry executes the given function with exponential backoff retry.
// Returns nil if the function succeeds, or the last error after all retries fail.
func WithRetry(ctx context.Context, cfg *RetryConfig, op RetryableFunc) error {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = output.DefaultLogger
	}

	backoff := cfg.InitialBackoff

	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		err := op()
		if err == nil {
			return nil
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Last attempt, don't wait
		if attempt == cfg.MaxRetries-1 {
			return fmt.Errorf("failed after %d retries: %w", cfg.MaxRetries, err)
		}

		logger.Warn("Attempt %d/%d failed: %v. Retrying in %v...",
			attempt+1, cfg.MaxRetries, err, backoff)

		// Wait with backoff
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		// Exponential backoff with cap
		backoff *= 2
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return fmt.Errorf("retry loop exited unexpectedly")
}

// WithRetrySimple executes the given function with default retry configuration.
func WithRetrySimple(ctx context.Context, op RetryableFunc) error {
	return WithRetry(ctx, DefaultRetryConfig(), op)
}

// RetryResult contains the result of a retry operation.
type RetryResult struct {
	Attempts int
	Success  bool
	LastErr  error
}

// WithRetryResult executes the given function with retry and returns detailed result.
func WithRetryResult(ctx context.Context, cfg *RetryConfig, op RetryableFunc) *RetryResult {
	if cfg == nil {
		cfg = DefaultRetryConfig()
	}

	result := &RetryResult{}
	backoff := cfg.InitialBackoff

	for attempt := 0; attempt < cfg.MaxRetries; attempt++ {
		result.Attempts = attempt + 1

		err := op()
		if err == nil {
			result.Success = true
			return result
		}

		result.LastErr = err

		// Check if context is cancelled
		if ctx.Err() != nil {
			result.LastErr = ctx.Err()
			return result
		}

		// Last attempt, don't wait
		if attempt == cfg.MaxRetries-1 {
			return result
		}

		if cfg.Logger != nil {
			cfg.Logger.Warn("Attempt %d/%d failed: %v. Retrying in %v...",
				attempt+1, cfg.MaxRetries, err, backoff)
		}

		// Wait with backoff
		select {
		case <-ctx.Done():
			result.LastErr = ctx.Err()
			return result
		case <-time.After(backoff):
		}

		// Exponential backoff with cap
		backoff *= 2
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return result
}
