package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"server-apeiron/internal/config"
)

func TestLoadGameRuntimeContractsRejectsImplicitRecoveredFallback(t *testing.T) {
	cfg := &config.Config{
		DBApeiron: config.DBApeironConfig{RequestTimeout: time.Second},
	}

	_, err := loadGameRuntimeContracts(context.Background(), cfg, nil)
	if err == nil {
		t.Fatal("expected runtime contract load to fail without DB or explicit recovery fallback")
	}
	if !strings.Contains(err.Error(), "ALLOW_RECOVERED_RUNTIME_FALLBACK=true") {
		t.Fatalf("error = %v", err)
	}
}

func TestLoadGameRuntimeContractsAllowsExplicitRecoveredFallback(t *testing.T) {
	cfg := &config.Config{
		DBApeiron: config.DBApeironConfig{RequestTimeout: time.Second},
		Runtime:   config.RuntimeConfig{AllowRecoveredRuntimeFallback: true},
	}

	contracts, err := loadGameRuntimeContracts(context.Background(), cfg, nil)
	if err != nil {
		t.Fatalf("explicit recovered fallback failed: %v", err)
	}
	if contracts.Source != "recovered_runtime_fallback" {
		t.Fatalf("source = %q", contracts.Source)
	}
}
