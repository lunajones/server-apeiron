package ai

import (
	"math"

	domainmath "server-apeiron/internal/domain/math"
)

type Brain struct {
	Policy Policy
	Memory Memory
}

func NewBrain(policy Policy) Brain {
	brain := Brain{Policy: policy}
	brain.Memory.ensure()
	return brain
}

func (b *Brain) Decide(input Input) Decision {
	if b == nil {
		return Decision{}
	}
	b.Memory.ensure()
	toTarget, right, rangeCM := targetVectors(input.CreaturePosition, input.TargetPosition)
	threat := AssessThreat(b.Policy, input, toTarget, rangeCM)

	if input.ActiveSkillID != "" {
		action := actionForSkill(input.ActiveSkillID, b.Policy)
		decisionPhase := "active"
		movementTactic := action
		commitment := "committed"
		reason := "active_skill_continuation"
		setup := setupPolicyForContinuation(b.Policy, input.ActiveSkillID, input.ActiveSetupPolicyID)
		dir, speed := movementForAction(action, b.Policy, toTarget, right, b.Memory.OrbitSide)
		setupPolicyID := ""
		if setup.ID != "" && input.ActiveSkillElapsedTicks < setup.MaxSetupTicks {
			dir, speed = movementForSetup(setup, b.Policy, toTarget, right, b.Memory.OrbitSide)
			decisionPhase = "setup"
			movementTactic = setup.MovementTactic
			commitment = "preparing"
			reason = "active_skill_setup:" + setup.ID
			setupPolicyID = setup.ID
		}
		decision := Decision{
			Action:         action,
			SelectedSkill:  input.ActiveSkillID,
			TacticalState:  b.Memory.LastTacticalState,
			DecisionPhase:  decisionPhase,
			MovementTactic: movementTactic,
			CombatTactic:   "harass",
			Commitment:     commitment,
			OrbitSide:      b.Memory.OrbitSide,
			Reason:         reason,
			Score:          1,
			ResourceCost:   skillCost(input.ActiveSkillID, input.SkillCosts),
			ResourceState:  resourceState(input),
			SpeedCMPerSec:  speed,
			Direction:      dir,
			Destination:    destinationForDecision(input.CreaturePosition, dir, b.Policy),
			RangeCM:        rangeCM,
			SetupPolicyID:  setupPolicyID,
			Threat:         threat,
		}
		b.Memory.remember(decision, input.Tick)
		return decision
	}

	tacticalState, decisionPhase := classifyTactic(b.Policy, rangeCM, threat.Pressure)
	selectedSkill := ""
	action := "orbit"
	reason := "orbit_policy"
	score := 0.5
	setupPolicyID := ""
	if binding, bindingScore, bindingReason, ok := selectBinding(b.Policy, b.Memory, input, threat, tacticalState, decisionPhase, rangeCM); ok {
		selectedSkill = binding.SkillID
		action = actionForSkill(binding.SkillID, b.Policy)
		reason = bindingReason
		score = bindingScore / 100
		if setup := setupPolicyForBinding(b.Policy, binding); setup.ID != "" {
			setupPolicyID = setup.ID
		}
	} else if tacticalState == "approach" {
		action = "chase"
		if skillAvailable("lunge", input.UnavailableSkill) && skillAffordable("lunge", input.ResourceCurrent, input.SkillCosts) {
			selectedSkill = "lunge"
			reason = "range_policy_chase"
		} else {
			reason = "range_policy_chase_lunge_unavailable"
		}
	} else if tacticalState == "pressure" {
		if skillAvailable(b.Policy.DodgeSkillID, input.UnavailableSkill) && skillAffordable(b.Policy.DodgeSkillID, input.ResourceCurrent, input.SkillCosts) {
			selectedSkill = b.Policy.DodgeSkillID
			action = "retreat"
			reason = "range_policy_retreat"
		} else {
			reason = "pressure_policy_no_ready_skill"
		}
	} else if rangeCM <= b.Policy.BiteRangeCM && skillAvailable("bite", input.UnavailableSkill) && skillAffordable("bite", input.ResourceCurrent, input.SkillCosts) {
		selectedSkill = "bite"
		action = "bite"
		reason = "range_policy_bite"
	}

	orbitSide := b.nextOrbitSide(input)
	dir, speed := movementForAction(action, b.Policy, toTarget, right, orbitSide)
	movementTactic := movementTacticForAction(action)
	commitment := commitmentForAction(action)
	if setupPolicyID != "" {
		setup := b.Policy.SetupPolicies[setupPolicyID]
		dir, speed = movementForSetup(setup, b.Policy, toTarget, right, orbitSide)
		movementTactic = setup.MovementTactic
		commitment = "preparing"
		reason += ":setup:" + setup.ID
	}
	decision := Decision{
		Action:         action,
		SelectedSkill:  selectedSkill,
		TacticalState:  tacticalState,
		DecisionPhase:  decisionPhase,
		MovementTactic: movementTactic,
		CombatTactic:   "harass",
		Commitment:     commitment,
		OrbitSide:      orbitSide,
		Reason:         reason,
		Score:          score,
		ResourceCost:   skillCost(selectedSkill, input.SkillCosts),
		ResourceState:  resourceState(input),
		SpeedCMPerSec:  speed,
		Direction:      dir,
		Destination:    destinationForDecision(input.CreaturePosition, dir, b.Policy),
		RangeCM:        rangeCM,
		SetupPolicyID:  setupPolicyID,
		Threat:         threat,
	}
	b.Memory.remember(decision, input.Tick)
	return decision
}

func destinationForDecision(origin domainmath.Position, direction domainmath.Vec3, policy Policy) domainmath.Position {
	distanceCM := policy.TacticalDestinationDistanceCM
	if distanceCM <= 0 {
		return origin
	}
	dir := direction.Normalize()
	if dir.IsZero() {
		return origin
	}
	return origin.Add(dir.Scale(distanceCM))
}

func resourceState(input Input) string {
	if input.ResourceMax <= 0 {
		return "untracked"
	}
	if input.ResourceCurrent <= 0 {
		return "empty"
	}
	return "available"
}

func (b *Brain) nextOrbitSide(input Input) string {
	side := b.Memory.OrbitSide
	if side == "" {
		side = "left"
	}
	minTicks := b.Policy.MinOrbitDurationTicks
	cooldownTicks := b.Policy.SideSwitchCooldownTicks
	if minTicks == 0 && cooldownTicks == 0 {
		return side
	}
	elapsed := input.Tick - b.Memory.OrbitSideChangedAtTick
	if elapsed < minTicks+cooldownTicks {
		return side
	}
	if !b.Policy.AllowSideSwitchWhenTargetFaces && b.Policy.LockSideDuringSetup {
		return side
	}
	if !shouldSwitchOrbitSide(b.Policy, input, side) {
		return side
	}
	if side == "left" {
		return "right"
	}
	return "left"
}

func shouldSwitchOrbitSide(policy Policy, input Input, currentSide string) bool {
	chance := math.Max(0, math.Min(1, policy.SideFlipChanceMultiplier))
	if chance <= 0 {
		return false
	}
	if chance >= 1 {
		return true
	}
	window := policy.MinOrbitDurationTicks + policy.SideSwitchCooldownTicks
	if window == 0 {
		return false
	}
	bucket := input.Tick / window
	if bucket == 0 {
		return false
	}
	return deterministicOrbitSideSwitchSample(bucket, currentSide) < chance
}

func deterministicOrbitSideSwitchSample(bucket uint64, currentSide string) float64 {
	const sampleSlots = 10000
	sideSalt := uint64(17)
	if currentSide == "right" {
		sideSalt = 53
	}
	// Fixed integer mixing makes side switching deterministic and reproducible on
	// server/client logs, while the contract still owns the probability threshold.
	mixed := bucket*1103515245 + sideSalt*12345 + 1013904223
	return float64(mixed%sampleSlots) / sampleSlots
}

func movementTacticForAction(action string) string {
	switch action {
	case "chase":
		return "chase"
	case "lunge":
		return "commit"
	case "maul":
		return "counter"
	case "retreat":
		return "retreat"
	default:
		return "flank"
	}
}

func commitmentForAction(action string) string {
	switch action {
	case "lunge", "maul", "bite", "skill":
		return "committed"
	case "retreat":
		return "evasive"
	default:
		return "probing"
	}
}

func PublishesSkill(action string) bool {
	return publishesSkill(action)
}
