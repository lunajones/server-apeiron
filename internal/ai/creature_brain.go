package ai

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

	if input.ActiveSkillID != "" {
		action := actionForSkill(input.ActiveSkillID, b.Policy)
		dir, speed := movementForAction(action, b.Policy, toTarget, right, b.Memory.OrbitSide)
		decision := Decision{
			Action:         action,
			SelectedSkill:  input.ActiveSkillID,
			TacticalState:  b.Memory.LastTacticalState,
			DecisionPhase:  "active",
			MovementTactic: action,
			CombatTactic:   "harass",
			Commitment:     "committed",
			OrbitSide:      b.Memory.OrbitSide,
			Reason:         "active_skill_continuation",
			Score:          1,
			SpeedCMPerSec:  speed,
			Direction:      dir,
			Destination:    input.CreaturePosition.Add(dir.Normalize().Scale(180)),
			RangeCM:        rangeCM,
		}
		b.Memory.remember(decision, input.Tick)
		return decision
	}

	tacticalState, decisionPhase := classifyTactic(b.Policy, rangeCM, input.Pressure)
	selectedSkill := "bite"
	action := "orbit"
	reason := "orbit_policy"
	score := 0.5
	if binding, ok := selectBinding(b.Policy, tacticalState, decisionPhase, rangeCM, input.LineOfSight); ok {
		selectedSkill = binding.SkillID
		action = actionForSkill(binding.SkillID, b.Policy)
		reason = "skill_behavior_binding:" + binding.ID
		score = float64(binding.Priority) * firstPositive(binding.UsageWeight, 1) / 100
	} else if tacticalState == "approach" {
		selectedSkill = "lunge"
		action = "chase"
		reason = "range_policy_chase"
	} else if tacticalState == "pressure" {
		selectedSkill = b.Policy.DodgeSkillID
		action = "retreat"
		reason = "range_policy_retreat"
	}

	orbitSide := b.nextOrbitSide(input)
	dir, speed := movementForAction(action, b.Policy, toTarget, right, orbitSide)
	decision := Decision{
		Action:         action,
		SelectedSkill:  selectedSkill,
		TacticalState:  tacticalState,
		DecisionPhase:  decisionPhase,
		MovementTactic: movementTacticForAction(action),
		CombatTactic:   "harass",
		Commitment:     commitmentForAction(action),
		OrbitSide:      orbitSide,
		Reason:         reason,
		Score:          score,
		SpeedCMPerSec:  speed,
		Direction:      dir,
		Destination:    input.CreaturePosition.Add(dir.Normalize().Scale(180)),
		RangeCM:        rangeCM,
	}
	b.Memory.remember(decision, input.Tick)
	return decision
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
	if b.Policy.SideFlipChanceMultiplier <= 0 {
		return side
	}
	if side == "left" {
		return "right"
	}
	return "left"
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
