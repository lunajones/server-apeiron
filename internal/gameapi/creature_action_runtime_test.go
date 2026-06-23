package gameapi

import (
	"math"
	"testing"
	"time"

	creatureai "server-apeiron/internal/ai"
	"server-apeiron/internal/combat/actionruntime"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/movement"
)

func TestWolfSkillStartsCreatureActionInstanceAndRuntimePhase(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	runtime.tick = 310
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI.GetSelectedSkillId() != "lunge" {
		t.Fatalf("selected skill = %q, want lunge", wolf.creatureAI.GetSelectedSkillId())
	}
	if wolf.actionInstance == nil {
		t.Fatal("wolf lunge did not create a creature action instance")
	}
	if wolf.actionInstance.ActorKind != actionruntime.ActorKindCreature {
		t.Fatalf("actor kind = %q, want creature", wolf.actionInstance.ActorKind)
	}
	if wolf.actionInstance.ActionKind != actionruntime.ActionKindActiveSkill {
		t.Fatalf("action kind = %q, want active_skill", wolf.actionInstance.ActionKind)
	}
	if wolf.actionInstance.SkillID.String() != "lunge" {
		t.Fatalf("action skill = %q, want lunge", wolf.actionInstance.SkillID.String())
	}
	if wolf.skillRuntime == nil {
		t.Fatal("wolf lunge did not publish skill runtime state")
	}
	if wolf.skillRuntime.GetCurrentSkillId() != "lunge" {
		t.Fatalf("runtime skill = %q, want lunge", wolf.skillRuntime.GetCurrentSkillId())
	}
	wantPhase := string(wolf.actionInstance.PhaseAt(time.Now()))
	if wolf.skillRuntime.GetState() != wantPhase {
		t.Fatalf("runtime state = %q, want action phase %q", wolf.skillRuntime.GetState(), wantPhase)
	}
	if wolf.skillRuntime.GetState() == "lunge" {
		t.Fatal("creature runtime state leaked old action-name state instead of action phase")
	}
	if wolf.skillRuntime.GetStartedAtMs() != wolf.actionInstance.StartedAt.UnixMilli() {
		t.Fatalf("runtime start = %d, action start = %d", wolf.skillRuntime.GetStartedAtMs(), wolf.actionInstance.StartedAt.UnixMilli())
	}
	if wolf.skillRuntime.GetCooldownEndMs() <= wolf.skillRuntime.GetStartedAtMs() {
		t.Fatalf("cooldown end = %d start = %d", wolf.skillRuntime.GetCooldownEndMs(), wolf.skillRuntime.GetStartedAtMs())
	}
}

func TestWolfActionRuntimeDoesNotRestartActiveSkillLifecycle(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	wolf.stamina = 40
	runtime.tick = 320
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.actionInstance == nil || wolf.skillRuntime == nil {
		t.Fatal("wolf lunge did not start action runtime")
	}
	instanceID := wolf.actionInstance.InstanceID
	startedAt := wolf.actionInstance.StartedAt
	cooldownEnd := wolf.skillRuntime.GetCooldownEndMs()
	staminaAfterStart := wolf.stamina

	runtime.tick = 321
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.actionInstance == nil {
		t.Fatal("active lunge lost action instance")
	}
	if wolf.actionInstance.InstanceID != instanceID {
		t.Fatalf("active lunge restarted action instance: %q -> %q", instanceID, wolf.actionInstance.InstanceID)
	}
	if !wolf.actionInstance.StartedAt.Equal(startedAt) {
		t.Fatalf("active lunge start time changed: %v -> %v", startedAt, wolf.actionInstance.StartedAt)
	}
	if wolf.skillRuntime.GetCooldownEndMs() != cooldownEnd {
		t.Fatalf("cooldown changed during active skill: %d -> %d", cooldownEnd, wolf.skillRuntime.GetCooldownEndMs())
	}
	wantStamina := staminaAfterStart + runtime.contracts.WolfPolicy.StaminaRegenPerSecond/tickRate
	if math.Abs(wolf.stamina-wantStamina) > 0.001 {
		t.Fatalf("active lunge spent stamina twice: got %.2f want %.2f", wolf.stamina, wantStamina)
	}
}

func TestWolfLungeWindupUsesSetupMovementBeforeSkillRootMotion(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	start := wolf.position
	runtime.tick = 325
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI.GetSelectedSkillId() != "lunge" {
		t.Fatalf("selected skill = %q, want lunge", wolf.creatureAI.GetSelectedSkillId())
	}
	if wolf.actionInstance == nil {
		t.Fatal("lunge did not create action instance")
	}
	if phase := wolf.actionInstance.PhaseAt(time.Now()); phase != actionruntime.PhaseWindup {
		t.Fatalf("initial lunge phase = %q, want windup", phase)
	}
	if wolf.actionMotion != nil {
		t.Fatalf("lunge root motion started during windup: %#v", wolf.actionMotion)
	}
	if moved := distance(start, wolf.position); moved <= 0 {
		t.Fatalf("lunge windup did not use setup movement, moved %.2f", moved)
	}
	if wolf.position.z != start.z {
		t.Fatalf("lunge setup changed grounded z: %.2f -> %.2f", start.z, wolf.position.z)
	}
}

func TestWolfLungeActivePhaseUsesSkillRootMotionOwner(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	runtime.tick = 326
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.actionInstance == nil || wolf.skillRuntime == nil {
		t.Fatal("lunge did not start action runtime")
	}
	contract := runtime.contracts.skillContract("lunge")
	activeElapsed := durationFromMS(contract.WindupMS) + 220*time.Millisecond
	startedAt := time.Now().Add(-activeElapsed)
	wolf.actionInstance.StartedAt = startedAt
	wolf.skillRuntime.StartedAtMs = startedAt.UnixMilli()
	wolf.actionMotion = nil
	before := wolf.position

	runtime.tick = 327
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.actionMotion == nil {
		t.Fatal("active lunge did not create skill root motion")
	}
	if wolf.actionMotion.SkillID != "lunge" {
		t.Fatalf("action motion skill = %q, want lunge", wolf.actionMotion.SkillID)
	}
	if wolf.actionMotion.CommandID != wolf.actionInstance.InstanceID {
		t.Fatalf("action motion command id = %q, want action instance %q", wolf.actionMotion.CommandID, wolf.actionInstance.InstanceID)
	}
	if moved := distance(before, wolf.position); moved <= 0 {
		t.Fatalf("active lunge root motion did not advance, moved %.2f", moved)
	}
	if wolf.locomotion == nil || wolf.locomotion.GetActionDistanceTraveled() <= 0 {
		t.Fatalf("active lunge locomotion did not publish action distance: %#v", wolf.locomotion)
	}
	if wolf.position.z != before.z {
		t.Fatalf("active lunge changed server root z: %.2f -> %.2f", before.z, wolf.position.z)
	}
}

func TestCreatureActionTimingExtendsUntilSkillRootMotionCompletes(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	contract := runtime.contracts.skillContract("maul")
	startedAt := time.Now()

	instance := runtime.newCreatureActionInstance(wolf, "maul", contract, wolf.position, startedAt)
	required := creatureSkillMovementStartOffset(instance.Timing, contract) + movement.ActionDuration(contract.MovementAction)

	if got := instance.Timing.Windup + instance.Timing.Active + instance.Timing.Recovery; got < required {
		t.Fatalf("maul action duration = %s, want at least movement start + movement duration", got)
	}
	if phase := instance.PhaseAt(startedAt.Add(required - 80*time.Millisecond)); phase == actionruntime.PhaseComplete {
		t.Fatalf("maul completed before movement contract could finish")
	}
	if phase := instance.PhaseAt(startedAt.Add(required + 10*time.Millisecond)); phase != actionruntime.PhaseComplete {
		t.Fatalf("maul phase after root completion = %q, want complete", phase)
	}
}

func TestWolfCompletedActionRuntimeClearsBeforeNextBrainDecision(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	runtime.tick = 330
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.actionInstance == nil || wolf.skillRuntime == nil {
		t.Fatal("wolf lunge did not start action runtime")
	}
	past := time.Now().Add(-5 * time.Second)
	wolf.actionInstance.StartedAt = past
	wolf.skillRuntime.StartedAtMs = past.UnixMilli()

	runtime.tick = 331
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.actionInstance != nil {
		t.Fatalf("completed creature action instance still active: %#v", wolf.actionInstance)
	}
	if wolf.skillRuntime.GetState() != "idle" {
		t.Fatalf("completed creature runtime state = %q, want idle", wolf.skillRuntime.GetState())
	}
	if wolf.creatureAI.GetSelectedSkillId() == "lunge" {
		t.Fatalf("completed lunge immediately reselected despite cooldown: %#v", wolf.creatureAI)
	}
}

func TestGroundedCreatureDecisionMotionPreservesGroundPlane(t *testing.T) {
	creature := &entityState{position: vector{x: 100, y: 200, z: 98}}
	decision := creatureai.Decision{
		Action:        "chase",
		SpeedCMPerSec: 300,
		Direction:     domainmath.V3(1, 0, 4).Normalize(),
	}

	resolved := resolveGroundedCreatureDecisionMotion(creature, decision)
	projected := fromDomainVector(resolved.Motion.Projected)
	velocity := fromDomainVector(resolved.Motion.Velocity)

	if projected.z != creature.position.z {
		t.Fatalf("grounded creature projected z = %.2f, want %.2f", projected.z, creature.position.z)
	}
	if velocity.z != 0 {
		t.Fatalf("grounded creature velocity z = %.2f, want 0", velocity.z)
	}
	if resolved.Motion.Direction.Z != 0 {
		t.Fatalf("grounded creature direction z = %.2f, want 0", resolved.Motion.Direction.Z)
	}
	if projected.x <= creature.position.x {
		t.Fatalf("grounded creature did not move horizontally: start=%#v projected=%#v", creature.position, projected)
	}
}
