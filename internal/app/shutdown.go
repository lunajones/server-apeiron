package app

import "context"

type ShutdownFunc func(ctx context.Context) error
