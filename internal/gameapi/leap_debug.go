package gameapi

import (
	"os"
	"sort"
	"strings"
	"time"

	"server-apeiron/internal/logging"
)

func apeironLeapDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("APEIRON_LEAP_DEBUG")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func (r *Runtime) logLeapDebugStateLocked(label string, player *entityState, extra map[string]string) {
	if !apeironLeapDebugEnabled() || player == nil {
		return
	}
	now := time.Now()
	motionSource := "<none>"
	motionAction := "<none>"
	motionAbility := "<none>"
	motionElapsedMS := int64(0)
	motionRemainingMS := int64(0)
	motionSequence := uint64(0)
	motionClientTick := uint64(0)
	motionDistance := 0.0
	motionProjected := player.position
	if player.actionMotion != nil {
		motion := player.actionMotion
		motionSource = motion.MotionSource
		motionAction = motion.Contract.ActionType
		motionAbility = motion.Contract.AbilityKey
		motionElapsedMS = now.Sub(motion.StartedAt).Milliseconds()
		if duration := durationFromMS(motion.Contract.DurationMS); duration > 0 {
			motionRemainingMS = (duration - now.Sub(motion.StartedAt)).Milliseconds()
		}
		motionSequence = motion.Sequence
		motionClientTick = motion.ClientTick
		motionDistance = motion.TotalDistanceCM
		motionProjected = motion.ProjectedPosition
	}
	if motionAction != "leap" && motionAbility != "jump" {
		if extra != nil {
			if action := strings.ToLower(strings.TrimSpace(extra["requested_action"])); action == "leap" {
				motionAction = action
			}
			if action := strings.ToLower(strings.TrimSpace(extra["completed_action"])); action == "leap" {
				motionAction = action
			}
			if ability := strings.ToLower(strings.TrimSpace(extra["requested_ability"])); ability == "jump" {
				motionAbility = ability
			}
			if ability := strings.ToLower(strings.TrimSpace(extra["completed_ability"])); ability == "jump" {
				motionAbility = ability
			}
		}
		if motionAction != "leap" && motionAbility != "jump" && label != "landing_handoff_applied" {
			return
		}
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
	logging.WithComponent("leap_debug").Info().Msgf(
		"ApeironLeapDebug server_state tick=%d player=%d label=%s movement_state=%s combat_state=%s skill_state=%s motion_source=%s motion_action=%s motion_ability=%s motion_elapsed_ms=%d motion_remaining_ms=%d motion_sequence=%d motion_client_tick=%d motion_distance=%.1f projected=(%.1f, %.1f, %.1f) position=(%.1f, %.1f, %.1f) velocity=(%.1f, %.1f, %.1f)%s",
		r.tick,
		player.id,
		label,
		player.movementState,
		player.combatState,
		player.skillState,
		motionSource,
		motionAction,
		motionAbility,
		motionElapsedMS,
		motionRemainingMS,
		motionSequence,
		motionClientTick,
		motionDistance,
		motionProjected.x,
		motionProjected.y,
		motionProjected.z,
		player.position.x,
		player.position.y,
		player.position.z,
		player.velocity.x,
		player.velocity.y,
		player.velocity.z,
		extraText)
}
