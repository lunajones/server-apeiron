package dbapeiron

import (
	"context"
	"time"

	"server-apeiron/internal/config"
)

func requestContext(ctx context.Context, cfg config.DBApeironConfig) (context.Context, context.CancelFunc) {
	timeout := cfg.RequestTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return context.WithTimeout(ctx, timeout)
}
