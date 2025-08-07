package resilience

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds retry configuration
type RetryConfig struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffMultiplier float64
	Jitter          bool
}

// DefaultRetryConfig returns a sensible default retry configuration
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxAttempts:      3,
		InitialDelay:     time.Second,
		MaxDelay:         30 * time.Second,
		BackoffMultiplier: 2.0,
		Jitter:           true,
	}
}

// Retry executes a function with retry logic
func Retry(ctx context.Context, config *RetryConfig, fn func() error) error {
	var lastErr error
	
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		
		// Execute the function
		if err := fn(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		
		// Don't sleep on the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}
		
		// Calculate delay
		delay := config.calculateDelay(attempt)
		
		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	
	return lastErr
}

// calculateDelay calculates the delay for the given attempt
func (rc *RetryConfig) calculateDelay(attempt int) time.Duration {
	delay := float64(rc.InitialDelay) * math.Pow(rc.BackoffMultiplier, float64(attempt))
	
	// Add jitter if enabled
	if rc.Jitter {
		jitter := rand.Float64() * 0.1 * delay // 10% jitter
		delay += jitter
	}
	
	// Cap at max delay
	if delay > float64(rc.MaxDelay) {
		delay = float64(rc.MaxDelay)
	}
	
	return time.Duration(delay)
}

// RetryWithResult executes a function that returns a result with retry logic
func RetryWithResult[T any](ctx context.Context, config *RetryConfig, fn func() (T, error)) (T, error) {
	var lastErr error
	var zero T
	
	for attempt := 0; attempt < config.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		default:
		}
		
		// Execute the function
		if result, err := fn(); err == nil {
			return result, nil
		} else {
			lastErr = err
		}
		
		// Don't sleep on the last attempt
		if attempt == config.MaxAttempts-1 {
			break
		}
		
		// Calculate delay
		delay := config.calculateDelay(attempt)
		
		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}
	
	return zero, lastErr
} 