package gameapi

import (
	"math"
	"strings"
	"time"

	dbv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/movement"
)

const impactControlReconciliationCategory = "impact_control_reconciliation"

func (r *Runtime) applyRuntimeImpactControlMotionLocked(source *entityState, target *entityState, forward vector, effect *dbv1.SkillControlEffect, now time.Time) {
	if r == nil || source == nil || target == nil || effect == nil || !effect.GetEnabled() {
		return
	}
	contract := runtimeImpactControlActionContract(effect)
	if movement.ActionDistance(contract, 0) <= 0 || movement.ActionDuration(contract) <= 0 {
		return
	}
	dir := runtimeImpactControlDirection(source, target, forward, effect.GetDirectionPolicy())
	if dir == (vector{}) {
		return
	}
	r.interruptTargetActionForImpactControlLocked(target, now)
	fullMotion := movement.ResolveActionMotion(movement.ActionMotionInput{
		Position:  toDomainVector(target.position),
		Direction: toDomainVector(dir),
		Contract:  contract,
	})
	if fullMotion.Stopped || fullMotion.DistanceCM <= 0 {
		return
	}
	target.actionMotion = &actionMotionState{
		SkillID:           effect.GetStatusEffectId(),
		CommandID:         effect.GetId(),
		Sequence:          r.tick,
		ClientTick:        r.tick,
		MotionSource:      "impact_control",
		StartedAt:         now,
		StartPosition:     target.position,
		ProjectedPosition: fromDomainVector(fullMotion.Projected),
		Direction:         dir,
		Contract:          contract,
		NormalInputPolicy: "blocked_during_owned_root",
		TotalDistanceCM:   fullMotion.DistanceCM,
		ContactPolicy:     effect.GetReleasePolicyId(),
	}
	progress := movement.ResolveActionMotionProgress(movement.ActionMotionProgressInput{
		Position:  toDomainVector(target.position),
		Direction: toDomainVector(dir),
		Contract:  contract,
		Elapsed:   0,
	})
	target.velocity = fromDomainVector(progress.Velocity)
	target.movementState = contract.ActionType
	target.skillState = "controlled"
	target.combatState = "controlled"
	target.locomotion = locomotionFromContractWithOverrides(contract, "active", target.position, target.position, r.tick, r.tick, fullMotion.SpeedCMPerSecond, progress.DistanceCM)
	target.locomotion.ActionDistanceTraveled = progress.DistanceCM
	target.locomotion.ActionProjectedPosition = toProto(fromDomainVector(fullMotion.Projected))
}

func (r *Runtime) interruptTargetActionForImpactControlLocked(target *entityState, now time.Time) {
	if r == nil || target == nil {
		return
	}
	r.interruptEntityActionRuntimeLocked(target, now, "impact_control")
}

func runtimeImpactControlActionContract(effect *dbv1.SkillControlEffect) MovementActionRuntimeContract {
	if effect == nil {
		return MovementActionRuntimeContract{}
	}
	durationMS := effect.GetDurationMs()
	distanceCM := effect.GetDistanceCm()
	speedCMS := effect.GetSpeedCmS()
	if distanceCM <= 0 && speedCMS > 0 && durationMS > 0 {
		distanceCM = speedCMS * (float64(durationMS) / 1000.0)
	}
	if durationMS <= 0 && distanceCM > 0 && speedCMS > 0 {
		durationMS = int32(math.Round((distanceCM / speedCMS) * 1000.0))
	}
	if speedCMS <= 0 && distanceCM > 0 && durationMS > 0 {
		speedCMS = distanceCM / (float64(durationMS) / 1000.0)
	}
	return MovementActionRuntimeContract{
		ID:                       "impact_control_" + effect.GetId(),
		AbilityKey:               effect.GetStatusEffectId(),
		ActionType:               "impact_control",
		DurationMS:               durationMS,
		ActiveMS:                 durationMS,
		DistanceCM:               distanceCM,
		BaseSpeedCMS:             speedCMS,
		SpeedCurveSamples:        []movement.MovementActionCurvePoint{{T: 0, Value: 1}, {T: 1, Value: 1}},
		ReconciliationContractID: impactControlReconciliationCategory,
		ReconciliationCategory:   impactControlReconciliationCategory,
		PhaseWindowPolicy:        "server_authoritative",
		PredictionErrorPolicy:    "bounded_smooth_correction",
		RootMotionOwner:          "impact_control",
		ContactPolicy:            effect.GetReleasePolicyId(),
	}
}

func runtimeImpactControlDirection(source *entityState, target *entityState, forward vector, policy string) vector {
	normalized := strings.ToLower(strings.TrimSpace(policy))
	if normalized == "" {
		normalized = "source_forward"
	}
	switch normalized {
	case "source_action_direction":
		if source != nil && source.actionMotion != nil {
			if dir := normalize(source.actionMotion.Direction); dir != (vector{}) {
				return dir
			}
		}
		return normalize(forward)
	case "source_lateral_right", "source_right_lateral":
		dir := normalize(forward)
		return normalize(vector{x: -dir.y, y: dir.x})
	case "source_lateral_left", "source_left_lateral":
		dir := normalize(forward)
		return normalize(vector{x: dir.y, y: -dir.x})
	case "source_to_target", "target_away_from_source":
		if source == nil || target == nil {
			return normalize(forward)
		}
		return normalize(vector{x: target.position.x - source.position.x, y: target.position.y - source.position.y})
	case "source_backward":
		dir := normalize(forward)
		return vector{x: -dir.x, y: -dir.y}
	default:
		return normalize(forward)
	}
}
