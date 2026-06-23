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

func TestWolfLungeMovementEnvelopeKeepsLandingInertiaAfterAirbornePhase(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	contract := runtime.contracts.skillContract("lunge")
	startedAt := time.Now()
	instance := runtime.newCreatureActionInstance(wolf, "lunge", contract, wolf.position, startedAt)
	rootStart := creatureSkillMovementStartAt(instance, contract)
	airborneEnd := rootStart.Add(creatureSkillAirborneDuration(contract))
	landingTick := airborneEnd.Add(40 * time.Millisecond)

	envelope := creatureActionMovementEnvelopeAt(instance, contract, landingTick)
	if !envelope.RootMotionActive {
		t.Fatalf("lunge root motion inactive during landing inertia: %#v", envelope)
	}
	if envelope.AirborneActive {
		t.Fatalf("lunge still airborne during landing inertia: %#v", envelope)
	}
	if !envelope.LandingInertiaActive {
		t.Fatalf("lunge did not expose landing inertia window: %#v", envelope)
	}
	if !envelope.AllowsPassthrough {
		t.Fatalf("lunge contact policy should allow passthrough: %#v", envelope)
	}
	if envelope.StopsAtContact {
		t.Fatalf("lunge should not stop at normal body contact: %#v", envelope)
	}
}

func TestWolfLungePassthroughDoesNotStopRootMotionAtPlayerBody(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	contract := runtime.contracts.skillContract("lunge")
	startedAt := time.Now().Add(-durationFromMS(contract.WindupMS) - 420*time.Millisecond)
	instance := runtime.newCreatureActionInstance(wolf, "lunge", contract, wolf.position, startedAt)
	wolf.actionInstance = &instance
	wolf.position = vector{x: player.position.x + 260, y: player.position.y, z: player.position.z}

	runtime.startCreatureSkillRootMotionLocked(wolf, player, creatureai.Decision{Action: "lunge", SelectedSkill: "lunge"}, contract, instance, creatureSkillMovementStartAt(instance, contract))
	if wolf.actionMotion == nil {
		t.Fatal("lunge did not start root motion")
	}
	if !wolf.actionMotion.AllowsPassthrough || wolf.actionMotion.StopsAtContact {
		t.Fatalf("lunge contact flags = passthrough:%v stop:%v", wolf.actionMotion.AllowsPassthrough, wolf.actionMotion.StopsAtContact)
	}
	before := wolf.position
	runtime.advanceActionMotionLocked(wolf, time.Now())

	if wolf.actionMotion == nil {
		t.Fatal("passthrough lunge stopped action motion at target contact")
	}
	if wolf.position.x >= before.x {
		t.Fatalf("passthrough lunge did not continue through target: before=%#v after=%#v", before, wolf.position)
	}
}

func TestWolfMaulContactStopsBeforeOverlappingTargetUsingContractGeometry(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	contract := runtime.contracts.skillContract("maul")
	start := vector{x: player.position.x + 160, y: player.position.y, z: player.position.z}
	wolf.position = start
	startedAt := time.Now().Add(-760 * time.Millisecond)
	instance := runtime.newCreatureActionInstance(wolf, "maul", contract, wolf.position, startedAt)

	runtime.startCreatureSkillRootMotionLocked(wolf, player, creatureai.Decision{Action: "maul", SelectedSkill: "maul"}, contract, instance, startedAt)
	if wolf.actionMotion == nil {
		t.Fatal("maul did not start root motion")
	}
	if !wolf.actionMotion.StopsAtContact || wolf.actionMotion.AllowsPassthrough {
		t.Fatalf("maul contact flags = passthrough:%v stop:%v", wolf.actionMotion.AllowsPassthrough, wolf.actionMotion.StopsAtContact)
	}
	if wolf.actionMotion.ContactStopCM <= 0 {
		t.Fatalf("maul contact stop distance was not derived from contract geometry: %#v", wolf.actionMotion)
	}
	stopDistance := wolf.actionMotion.ContactStopCM

	runtime.advanceActionMotionLocked(wolf, time.Now())

	if wolf.actionMotion != nil {
		t.Fatalf("maul contact stop should complete root motion: %#v", wolf.actionMotion)
	}
	if wolf.position.z != start.z {
		t.Fatalf("maul contact response changed ground plane: %.2f -> %.2f", start.z, wolf.position.z)
	}
	targetDistance := distance(start, player.position)
	remainingDistance := distance(wolf.position, player.position)
	if remainingDistance < stopDistance-0.001 {
		t.Fatalf("maul overlapped target: remaining %.2f stop %.2f targetDistance %.2f", remainingDistance, stopDistance, targetDistance)
	}
}

func TestCreatureContactStopDistanceComesFromHitboxGeometryOnly(t *testing.T) {
	contracts := RecoveryFixtureRuntimeContracts()
	maul := contracts.skillContract("maul")
	if got := creatureSkillContactStopDistanceCM(maul); got <= 0 {
		t.Fatalf("maul stop distance = %.2f, want contract-derived geometry", got)
	}

	maul.Hitboxes = nil
	if got := creatureSkillContactStopDistanceCM(maul); got != 0 {
		t.Fatalf("empty hitbox contract invented stop distance %.2f", got)
	}
}

func TestWolfLungeMovementPresentationIsContractDerived(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	contract := runtime.contracts.skillContract("lunge")
	presentation := creatureSkillMovementPresentationFromContract(contract)
	wantStart := durationMillis(creatureSkillMovementStartOffset(creatureActionTimingFromSkillContract(contract), contract))
	wantLanding := durationMillis(movement.ActionDuration(contract.MovementAction) - creatureSkillAirborneDuration(contract))

	if presentation.MovementStartMS != wantStart {
		t.Fatalf("movement start ms = %d, want contract offset %d", presentation.MovementStartMS, wantStart)
	}
	if presentation.TakeoffMS != wantStart {
		t.Fatalf("takeoff ms = %d, want movement start %d", presentation.TakeoffMS, wantStart)
	}
	if presentation.LandingLockMS != wantLanding {
		t.Fatalf("landing lock ms = %d, want movement duration-airborne %d", presentation.LandingLockMS, wantLanding)
	}
	if presentation.MovementDuration != contract.MovementAction.DurationMS {
		t.Fatalf("movement duration = %d, want contract duration %d", presentation.MovementDuration, contract.MovementAction.DurationMS)
	}
	if presentation.MovementDistance != contract.MovementAction.DistanceCM {
		t.Fatalf("movement distance = %.1f, want contract distance %.1f", presentation.MovementDistance, contract.MovementAction.DistanceCM)
	}
	if presentation.StopAtContactRate != 1 {
		t.Fatalf("passthrough lunge stop-at-contact rate = %.2f, want 1", presentation.StopAtContactRate)
	}
}

func TestWolfPublishedAIStateUsesContractMovementPresentation(t *testing.T) {
	runtime := NewRuntimeWithContracts(RecoveryFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)

	wolf.position = vector{x: player.position.x + 520, y: player.position.y, z: player.position.z}
	runtime.tick = 328
	runtime.updateWolfPolicyLocked(wolf, player)

	if wolf.creatureAI == nil {
		t.Fatal("wolf AI state was not published")
	}
	contract := runtime.contracts.skillContract(wolf.creatureAI.GetSelectedSkillId())
	presentation := creatureSkillMovementPresentationFromContract(contract)
	if wolf.creatureAI.GetSkillMovementTakeoffMs() != presentation.TakeoffMS {
		t.Fatalf("published takeoff = %d, want %d", wolf.creatureAI.GetSkillMovementTakeoffMs(), presentation.TakeoffMS)
	}
	if wolf.creatureAI.GetSkillMovementLandingLockMs() != presentation.LandingLockMS {
		t.Fatalf("published landing lock = %d, want %d", wolf.creatureAI.GetSkillMovementLandingLockMs(), presentation.LandingLockMS)
	}
	if wolf.creatureAI.GetSkillMovementStartMs() != presentation.MovementStartMS {
		t.Fatalf("published movement start = %d, want %d", wolf.creatureAI.GetSkillMovementStartMs(), presentation.MovementStartMS)
	}
	if wolf.creatureAI.GetSkillMovementDistanceCm() != presentation.MovementDistance {
		t.Fatalf("published movement distance = %.1f, want %.1f", wolf.creatureAI.GetSkillMovementDistanceCm(), presentation.MovementDistance)
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
