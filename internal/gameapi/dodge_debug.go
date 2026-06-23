package gameapi

import (
	"os"
	"sort"
	"strings"
	"time"

	"server-apeiron/internal/logging"
)

func apeironDodgeDebugEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("APEIRON_DODGE_DEBUG")))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func (r *Runtime) logDodgeDebugStateLocked(label string, player *entityState, extra map[string]string) {
	if !apeironDodgeDebugEnabled() || player == nil {
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
	}
	instanceSkill := "<none>"
	instancePhase := "<none>"
	if player.actionInstance != nil {
		instanceSkill = player.actionInstance.SkillID.String()
		instancePhase = string(player.actionInstance.PhaseAt(now))
	}
	runtimeSkill := "<none>"
	runtimeSkillState := "<none>"
	if player.skillRuntime != nil {
		runtimeSkill = player.skillRuntime.GetCurrentSkillId()
		runtimeSkillState = player.skillRuntime.GetState()
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
	logging.WithComponent("dodge_debug").Info().Msgf(
		"ApeironDodgeDebug server_state tick=%d player=%d label=%s movement_state=%s combat_state=%s skill_state=%s pipeline_state=%s iframe=%t motion_source=%s motion_action=%s motion_ability=%s motion_elapsed_ms=%d motion_remaining_ms=%d motion_sequence=%d motion_client_tick=%d action_instance_skill=%s action_instance_phase=%s skill_runtime_skill=%s skill_runtime_state=%s position=(%.1f, %.1f, %.1f) velocity=(%.1f, %.1f, %.1f)%s",
		r.tick,
		player.id,
		label,
		player.movementState,
		player.combatState,
		player.skillState,
		runtimeEntityCombatPipelineStateAt(player, now),
		runtimeEntityHasIFrameStateAt(player, now),
		motionSource,
		motionAction,
		motionAbility,
		motionElapsedMS,
		motionRemainingMS,
		motionSequence,
		motionClientTick,
		instanceSkill,
		instancePhase,
		runtimeSkill,
		runtimeSkillState,
		player.position.x,
		player.position.y,
		player.position.z,
		player.velocity.x,
		player.velocity.y,
		player.velocity.z,
		extraText)
}
