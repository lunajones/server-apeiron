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
		t.Fatal("expected runtime contract load to fail without DB")
	}
	if !strings.Contains(err.Error(), "game runtime contracts require db-apeiron") {
		t.Fatalf("error = %v", err)
	}
}
