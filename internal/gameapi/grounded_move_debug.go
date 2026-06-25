package gameapi

import (
	"os"
	"sort"
	"strings"

	"server-apeiron/internal/logging"
)

func apeironGroundedMoveDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("APEIRON_GROUNDED_MOVE_DEBUG")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func (r *Runtime) logGroundedMoveDebugStateLocked(label string, player *entityState, extra map[string]string) {
	if !apeironGroundedMoveDebugEnabled() || player == nil || player.entityType != "player" {
		return
	}
	extraText := ""
	if len(extra) > 0 {
		keys := make([]string, 0, len(extra))
		for key := range extra {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, key := range keys {
			parts = append(parts, key+"="+extra[key])
		}
		extraText = " " + strings.Join(parts, " ")
	}
	logging.WithComponent("grounded_move_debug").Info().Msgf(
		"ApeironGroundedMoveDebug server_state tick=%d player=%d label=%s movement_state=%s combat_state=%s skill_state=%s position=(%.1f, %.1f, %.1f) velocity=(%.1f, %.1f, %.1f) yaw=%.1f%s",
		r.tick,
		player.id,
		label,
		player.movementState,
		player.combatState,
		player.skillState,
		player.position.x,
		player.position.y,
		player.position.z,
		player.velocity.x,
		player.velocity.y,
		player.velocity.z,
		player.yaw,
		extraText)
}
