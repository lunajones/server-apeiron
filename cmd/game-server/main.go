package main

import (
	"context"
	"os"

	"server-apeiron/internal/app"
	"server-apeiron/internal/config"
	"server-apeiron/internal/logging"
)

func main() {
	ctx := context.Background()

	cfg, err := config.LoadConfig()
	if err != nil {
		logging.BootstrapError("failed to load config", err)
		os.Exit(1)
	}

	logging.Initialize(cfg.Logger)

	if err := app.Run(ctx, cfg); err != nil {
		logging.Log.Error().
			Err(err).
			Msg("game server stopped with error")
		os.Exit(1)
	}
}
