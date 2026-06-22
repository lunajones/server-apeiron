package dbapeiron

import (
	"context"
	"time"

	"server-apeiron/internal/config"
)

func withRetry(ctx context.Context, cfg config.DBApeironConfig, fn func() error) error {
	attempts := cfg.RetryAttempts
	if attempts < 1 {
		attempts = 1
	}

	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
		} else {
			return nil
		}

		if attempt == attempts {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cfg.RetryBackoff):
		}
	}

	return lastErr
}
