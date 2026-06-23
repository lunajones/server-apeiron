package app

import (
	"context"
	"strings"

	"server-apeiron/internal/config"
	"server-apeiron/internal/dbapeiron"
	"server-apeiron/internal/gameapi"
	"server-apeiron/internal/logging"
)

func Run(ctx context.Context, cfg *config.Config) error {
	log := logging.WithComponent("app")

	dbClient, err := connectDBApeiron(ctx, cfg)
	if err != nil {
		return err
	}
	if dbClient != nil {
		defer func() {
			if err := dbClient.Close(); err != nil {
				log.Warn().Err(err).Msg("db-apeiron client close failed")
			}
		}()
	}

	runtimeContracts, err := loadGameRuntimeContracts(ctx, cfg, dbClient)
	if err != nil {
		return err
	}
	log.Info().Str("source", runtimeContracts.Source).Msg("game runtime contracts loaded")

	runtimeOptions := gameapi.RuntimeOptions{
		MovementValidation: cfg.Validation.MovementValidation,
		DisableCreatures:   !cfg.AI.CreatureRuntimeEnabled,
	}
	log.Info().
		Bool("movement_validation", runtimeOptions.MovementValidation).
		Bool("creature_runtime_enabled", cfg.AI.CreatureRuntimeEnabled).
		Msg("game server bootstrap completed")
	return gameapi.ServeRuntime(ctx, cfg.Network, gameapi.NewRuntimeWithOptions(runtimeContracts, runtimeOptions))
}

func loadGameRuntimeContracts(ctx context.Context, cfg *config.Config, dbClient *dbapeiron.Client) (gameapi.RuntimeContracts, error) {
	if dbClient != nil {
		loadCtx, cancel := context.WithTimeout(ctx, cfg.DBApeiron.RequestTimeout)
		contracts := gameapi.LoadRuntimeContractsFromDB(loadCtx, dbClient.Skills, dbClient.Profiles, dbClient.Creatures)
		cancel()
		if err := contracts.ValidateRequiredCoverage(true); err != nil {
			return gameapi.RuntimeContracts{}, err
		}
		return contracts, nil
	}

	return gameapi.RuntimeContracts{}, dbapeiron.ErrRequiredUnavailable(
		"game runtime contracts require db-apeiron; set DB_APEIRON_ENDPOINT and keep DB_APEIRON_STARTUP_REQUIRED=true",
	)
}

func connectDBApeiron(ctx context.Context, cfg *config.Config) (*dbapeiron.Client, error) {
	log := logging.WithComponent("dbapeiron")
	dbCfg := cfg.DBApeiron

	if strings.TrimSpace(dbCfg.Endpoint) == "" {
		if dbCfg.StartupRequired {
			return nil, dbapeiron.ErrRequiredUnavailable("db-apeiron endpoint is empty")
		}
		log.Info().Msg("db-apeiron endpoint not configured; skipping optional startup connection")
		return nil, nil
	}

	client, err := dbapeiron.Connect(ctx, dbCfg)
	if err != nil {
		if dbCfg.StartupRequired {
			return nil, dbapeiron.ErrRequiredUnavailable(err.Error())
		}
		log.Warn().Err(err).Msg("db-apeiron optional startup connection failed")
		return nil, nil
	}

	if err := client.CheckReadiness(ctx, dbCfg); err != nil {
		if closeErr := client.Close(); closeErr != nil {
			log.Warn().Err(closeErr).Msg("db-apeiron client close failed after readiness error")
		}
		if dbCfg.StartupRequired {
			return nil, dbapeiron.ErrRequiredUnavailable(err.Error())
		}
		log.Warn().Err(err).Msg("db-apeiron optional readiness check failed")
		return nil, nil
	}

	log.Info().Str("endpoint", dbCfg.Endpoint).Msg("db-apeiron readiness confirmed")
	return client, nil
}
