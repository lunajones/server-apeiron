package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	App        AppConfig
	Logger     LoggerConfig
	Server     ServerConfig
	Network    NetworkConfig
	Tick       TickConfig
	DBApeiron  DBApeironConfig
	Static     StaticDataConfig
	World      WorldConfig
	Navigation NavigationConfig
	AI         AIConfig
	Validation ValidationConfig
}

type AppConfig struct {
	Name        string
	Environment string
}

type LoggerConfig struct {
	Level  string
	Pretty bool
	File   string
}

type ServerConfig struct {
	ShutdownTimeout time.Duration
}

type NetworkConfig struct {
	Enabled              bool
	GRPCHost             string
	GRPCPort             int
	CommandMaxSkew       time.Duration
	SessionTimeout       time.Duration
	SessionTokenRequired bool
}

type TickConfig struct {
	Rate         int
	SlowPhase    time.Duration
	SlowFrame    time.Duration
	AutoStart    bool
	MaxTicks     int
	StartRuntime bool
}

type DBApeironConfig struct {
	Endpoint             string
	ConnectTimeout       time.Duration
	RequestTimeout       time.Duration
	RetryAttempts        int
	RetryBackoff         time.Duration
	StartupRequired      bool
	StartupWarmupEnabled bool
}

type StaticDataConfig struct {
	PreloadCreatureTemplateIDs []string
	PreloadSkillIDs            []string
	PreloadSkillSetIDs         []string
	PreloadWeaponKitIDs        []string
	PreloadStatusEffectIDs     []string
	PreloadItemTemplateIDs     []string
	PreloadRegionIDs           []string
}

type WorldConfig struct {
	StartupLoadEnabled     bool
	StartupActivateEnabled bool
	PackagePath            string
	StartupPackagePaths    []string
}

type NavigationConfig struct {
	StartupLoadEnabled bool
	NavMeshPath        string
}

type AIConfig struct {
	StrictCreatureBehavior bool
	CreatureRuntimeEnabled bool
}

type ValidationConfig struct {
	MovementValidation bool
}

func LoadConfig() (*Config, error) {
	if err := LoadEnv(); err != nil {
		return nil, err
	}

	logPretty, err := getOptionalBool("LOG_PRETTY", true)
	if err != nil {
		return nil, err
	}

	shutdownTimeoutSeconds, err := getOptionalInt("SHUTDOWN_TIMEOUT_SECONDS", 10)
	if err != nil {
		return nil, err
	}

	tickRate, err := getOptionalInt("TICK_RATE", 30)
	if err != nil {
		return nil, err
	}

	slowPhaseMS, err := getOptionalInt("TICK_SLOW_PHASE_MS", 8)
	if err != nil {
		return nil, err
	}

	slowFrameMS, err := getOptionalInt("TICK_SLOW_FRAME_MS", 16)
	if err != nil {
		return nil, err
	}

	connectTimeoutSeconds, err := getOptionalInt("DB_APEIRON_CONNECT_TIMEOUT_SECONDS", 2)
	if err != nil {
		return nil, err
	}

	requestTimeoutSeconds, err := getOptionalInt("DB_APEIRON_REQUEST_TIMEOUT_SECONDS", 2)
	if err != nil {
		return nil, err
	}

	retryAttempts, err := getOptionalInt("DB_APEIRON_RETRY_ATTEMPTS", 1)
	if err != nil {
		return nil, err
	}

	retryBackoffMS, err := getOptionalInt("DB_APEIRON_RETRY_BACKOFF_MS", 200)
	if err != nil {
		return nil, err
	}

	startupRequired, err := getOptionalBool("DB_APEIRON_STARTUP_REQUIRED", false)
	if err != nil {
		return nil, err
	}

	startupWarmupEnabled, err := getOptionalBool("DB_APEIRON_STARTUP_WARMUP_ENABLED", false)
	if err != nil {
		return nil, err
	}

	worldStartupLoadEnabled, err := getOptionalBool("WORLD_STARTUP_LOAD_ENABLED", false)
	if err != nil {
		return nil, err
	}

	worldStartupActivateEnabled, err := getOptionalBool("WORLD_STARTUP_ACTIVATE_ENABLED", false)
	if err != nil {
		return nil, err
	}
	worldPackagePath := getEnv("WORLD_PACKAGE_PATH")
	worldStartupPackagePaths := getDelimitedList("WORLD_STARTUP_PACKAGE_PATHS")
	if len(worldStartupPackagePaths) == 0 && strings.TrimSpace(worldPackagePath) != "" {
		worldStartupPackagePaths = []string{strings.TrimSpace(worldPackagePath)}
	}

	navmeshStartupLoadEnabled, err := getOptionalBool("NAVMESH_STARTUP_LOAD_ENABLED", false)
	if err != nil {
		return nil, err
	}

	networkEnabled, err := getOptionalBool("GAME_GRPC_ENABLED", true)
	if err != nil {
		return nil, err
	}

	gameGRPCPort, err := getOptionalInt("GAME_GRPC_PORT", 50052)
	if err != nil {
		return nil, err
	}

	commandMaxSkewMS, err := getOptionalInt("COMMAND_MAX_SKEW_MS", 2000)
	if err != nil {
		return nil, err
	}

	sessionTimeoutSeconds, err := getOptionalInt("SESSION_TIMEOUT_SECONDS", 300)
	if err != nil {
		return nil, err
	}

	sessionTokenRequired, err := getOptionalBool("SESSION_TOKEN_REQUIRED", false)
	if err != nil {
		return nil, err
	}
	tickRuntimeEnabled, err := getOptionalBool("TICK_RUNTIME_ENABLED", true)
	if err != nil {
		return nil, err
	}
	strictCreatureBehavior, err := getOptionalBool("AI_STRICT_CREATURE_BEHAVIOR", true)
	if err != nil {
		return nil, err
	}
	creatureRuntimeEnabled, err := getOptionalBool("CREATURE_RUNTIME_ENABLED", true)
	if err != nil {
		return nil, err
	}
	if hasArg("DisableCreatures") {
		creatureRuntimeEnabled = false
	}
	movementValidation, err := getOptionalBool("MOVEMENT_VALIDATION", false)
	if err != nil {
		return nil, err
	}
	if hasArg("MovementValidation") {
		movementValidation = true
	}

	return &Config{
		App: AppConfig{
			Name:        getEnv("APP_NAME"),
			Environment: getEnv("ENVIRONMENT"),
		},
		Logger: LoggerConfig{
			Level:  getEnv("LOG_LEVEL"),
			Pretty: logPretty,
			File:   getOptionalEnv("LOG_FILE", ""),
		},
		Server: ServerConfig{
			ShutdownTimeout: time.Duration(shutdownTimeoutSeconds) * time.Second,
		},
		Network: NetworkConfig{
			Enabled:              networkEnabled,
			GRPCHost:             getOptionalEnv("GAME_GRPC_HOST", "127.0.0.1"),
			GRPCPort:             gameGRPCPort,
			CommandMaxSkew:       time.Duration(commandMaxSkewMS) * time.Millisecond,
			SessionTimeout:       time.Duration(sessionTimeoutSeconds) * time.Second,
			SessionTokenRequired: sessionTokenRequired,
		},
		Tick: TickConfig{
			Rate:         tickRate,
			SlowPhase:    time.Duration(slowPhaseMS) * time.Millisecond,
			SlowFrame:    time.Duration(slowFrameMS) * time.Millisecond,
			StartRuntime: tickRuntimeEnabled,
		},
		DBApeiron: DBApeironConfig{
			Endpoint:             getEnv("DB_APEIRON_ENDPOINT"),
			ConnectTimeout:       time.Duration(connectTimeoutSeconds) * time.Second,
			RequestTimeout:       time.Duration(requestTimeoutSeconds) * time.Second,
			RetryAttempts:        retryAttempts,
			RetryBackoff:         time.Duration(retryBackoffMS) * time.Millisecond,
			StartupRequired:      startupRequired,
			StartupWarmupEnabled: startupWarmupEnabled,
		},
		Static: StaticDataConfig{
			PreloadCreatureTemplateIDs: getCSV("STATICDATA_PRELOAD_CREATURE_TEMPLATE_IDS"),
			PreloadSkillIDs:            getCSV("STATICDATA_PRELOAD_SKILL_IDS"),
			PreloadSkillSetIDs:         getCSV("STATICDATA_PRELOAD_SKILL_SET_IDS"),
			PreloadWeaponKitIDs:        getCSV("STATICDATA_PRELOAD_WEAPON_KIT_IDS"),
			PreloadStatusEffectIDs:     getCSV("STATICDATA_PRELOAD_STATUS_EFFECT_IDS"),
			PreloadItemTemplateIDs:     getCSV("STATICDATA_PRELOAD_ITEM_TEMPLATE_IDS"),
			PreloadRegionIDs:           getCSV("STATICDATA_PRELOAD_REGION_IDS"),
		},
		World: WorldConfig{
			StartupLoadEnabled:     worldStartupLoadEnabled,
			StartupActivateEnabled: worldStartupActivateEnabled,
			PackagePath:            worldPackagePath,
			StartupPackagePaths:    worldStartupPackagePaths,
		},
		Navigation: NavigationConfig{
			StartupLoadEnabled: navmeshStartupLoadEnabled,
			NavMeshPath:        getEnv("NAVMESH_PATH"),
		},
		AI: AIConfig{
			StrictCreatureBehavior: strictCreatureBehavior,
			CreatureRuntimeEnabled: creatureRuntimeEnabled,
		},
		Validation: ValidationConfig{
			MovementValidation: movementValidation,
		},
	}, nil
}

func getEnv(key string) string {
	return os.Getenv(key)
}

func hasArg(name string) bool {
	needle := strings.ToLower(strings.TrimLeft(name, "-/"))
	for _, arg := range os.Args[1:] {
		normalized := strings.ToLower(strings.TrimLeft(strings.TrimSpace(arg), "-/"))
		if normalized == needle {
			return true
		}
	}
	return false
}

func getInt(key string) (int, error) {
	value := os.Getenv(key)

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %w", key, err)
	}

	return parsed, nil
}

func getOptionalInt(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid integer value for %s: %w", key, err)
	}

	return parsed, nil
}

func getBool(key string) (bool, error) {
	value := os.Getenv(key)

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid boolean value for %s: %w", key, err)
	}

	return parsed, nil
}

func getOptionalBool(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid boolean value for %s: %w", key, err)
	}

	return parsed, nil
}

func getOptionalEnv(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	return value
}

func getCSV(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		out = append(out, trimmed)
	}

	return out
}

func getDelimitedList(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
