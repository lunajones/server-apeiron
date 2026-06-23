package combat

import (
	"testing"

	"server-apeiron/internal/domain/ids"
)

func TestMigratedPlayerSkillProfilesNeverUseFallback(t *testing.T) {
	for _, skillID := range []ids.SkillID{
		"player_basic_attack",
		"player_basic_attack_1",
		"player_basic_attack_2",
		"player_basic_attack_3",
		"player_shield_bash",
		"player_shield_rush",
	} {
		if !skillMovementMigratedProfileFallbackBlocked(skillID) {
			t.Fatalf("%s should reject recovered profile fallback", skillID)
		}
	}
}

func TestNonMigratedSkillProfileFallbackCanRemainRecoveryOnly(t *testing.T) {
	if skillMovementMigratedProfileFallbackBlocked("temporary_recovery_skill") {
		t.Fatal("non-migrated recovery skill should not be blocked by the migrated-skill guard")
	}
}
