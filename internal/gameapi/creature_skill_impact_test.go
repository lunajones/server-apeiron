package gameapi

import (
	"context"
	"math"
	"testing"
	"time"

	gamev1 "server-apeiron/gen/apeiron/game/v1"
)

func TestCreatureTemporalSkillImpactDamagesPlayerOncePerInstance(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "creature-temporal-skill-impact"
	attachRuntimePlayer(t, runtime, sessionID)

	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x - 160, y: player.position.y, z: player.position.z}
	wolf.yaw = 0
	player.yaw = 180

	contract := runtime.contracts.skillContract("bite")
	now := time.Now()
	wolf.skillRuntime = &gamev1.SkillRuntimeState{
		CurrentSkillId:   "bite",
		State:            "bite",
		StartedAtMs:      now.Add(-200 * time.Millisecond).UnixMilli(),
		LastResolvedAtMs: now.Add(-200 * time.Millisecond).UnixMilli(),
	}
	beforeHealth := player.health
	beforePosture := player.posture

	if !runtime.enqueueCreatureSkillImpactLocked(wolf, player, contract, now) {
		t.Fatal("creature impact was not enqueued during temporal hitbox window")
	}
	impacts := runtime.runPendingSkillImpactSchedulesLocked(now)
	if len(impacts) != 1 {
		t.Fatalf("impacts = %d, want 1", len(impacts))
	}
	wantHealth := beforeHealth - contract.Damage*0.95
	if math.Abs(player.health-wantHealth) > 0.001 {
		t.Fatalf("player health = %.1f, want %.1f", player.health, wantHealth)
	}
	if player.posture != beforePosture-contract.PostureDamage {
		t.Fatalf("player posture = %.1f, want %.1f", player.posture, beforePosture-contract.PostureDamage)
	}

	if runtime.enqueueCreatureSkillImpactLocked(wolf, player, contract, now.Add(16*time.Millisecond)) {
		t.Fatal("duplicate creature skill instance was enqueued after impact resolution")
	}
	again := runtime.runPendingSkillImpactSchedulesLocked(now.Add(16 * time.Millisecond))
	if len(again) != 0 {
		t.Fatalf("same creature skill instance applied damage twice: %d impacts", len(again))
	}
	if math.Abs(player.health-wantHealth) > 0.001 {
		t.Fatalf("player health changed after duplicate resolution: %.1f", player.health)
	}
}

func TestCreatureTemporalSkillImpactWaitsForHitboxWindow(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	sessionID := "creature-temporal-skill-impact-window"
	if _, err := runtime.OpenSession(context.Background(), &gamev1.OpenSessionRequest{
		Context: &gamev1.RequestContext{SessionId: sessionID},
	}); err != nil {
		t.Fatalf("OpenSession failed: %v", err)
	}
	if _, err := runtime.AttachPlayer(context.Background(), &gamev1.AttachPlayerRequest{
		Context:  &gamev1.RequestContext{SessionId: sessionID},
		PlayerId: "local_player",
	}); err != nil {
		t.Fatalf("AttachPlayer failed: %v", err)
	}

	runtime.mu.Lock()
	defer runtime.mu.Unlock()

	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	wolf.position = vector{x: player.position.x - 160, y: player.position.y, z: player.position.z}
	contract := runtime.contracts.skillContract("bite")
	now := time.Now()
	wolf.skillRuntime = &gamev1.SkillRuntimeState{
		CurrentSkillId:   "bite",
		State:            "bite",
		StartedAtMs:      now.Add(-60 * time.Millisecond).UnixMilli(),
		LastResolvedAtMs: now.Add(-60 * time.Millisecond).UnixMilli(),
	}
	beforeHealth := player.health

	if !runtime.enqueueCreatureSkillImpactLocked(wolf, player, contract, now) {
		t.Fatal("creature impact was not enqueued before bite hitbox window")
	}
	impacts := runtime.runPendingSkillImpactSchedulesLocked(now)
	if len(impacts) != 0 {
		t.Fatalf("impact runner resolved before bite hitbox window: %d impacts", len(impacts))
	}
	if player.health != beforeHealth {
		t.Fatalf("player health changed before hitbox window: %.1f want %.1f", player.health, beforeHealth)
	}
}

func TestCreatureTemporalSkillImpactFollowsAuthoritativeActionDirection(t *testing.T) {
	t.Parallel()

	runtime := NewRuntimeWithContracts(DevFixtureRuntimeContracts())
	player := runtime.ensurePlayerLocked("local_player")
	wolf := runtime.ensureWolfLocked(player)
	player.position = vector{x: 220, y: 0, z: 0}
	wolf.position = vector{x: 0, y: 0, z: 0}
	contract := runtime.contracts.skillContract("lunge")
	startedAt := time.Now().Add(-3700 * time.Millisecond)
	instanceID := "lunge-action-direction-test"
	wolf.actionMotion = &actionMotionState{
		SkillID:         "lunge",
		CommandID:       instanceID,
		StartedAt:       startedAt,
		StartPosition:   wolf.position,
		Direction:       vector{x: 1, y: 0},
		TotalDistanceCM: contract.MovementAction.DistanceCM,
		Contract:        contract.MovementAction,
	}
	beforeHealth := player.health

	schedule := skillImpactSchedule{
		InstanceID:  instanceID,
		StartedAt:   startedAt,
		Source:      wolf,
		Skill:       contract,
		Direction:   vector{x: 0, y: 1},
		ElapsedMS:   3700,
		PreviousMS:  3684,
		RequireTime: true,
		TrackSource: true,
	}
	schedule.Start, schedule.End, schedule.Direction = skillImpactScheduleTrace(schedule)
	impacts := runtime.resolveSkillImpactScheduleLocked(schedule)
	if len(impacts) != 1 {
		t.Fatalf("lunge impact count = %d, want 1; trace dir=%#v start=%#v end=%#v", len(impacts), schedule.Direction, schedule.Start, schedule.End)
	}
	if player.health >= beforeHealth {
		t.Fatalf("lunge impact did not damage player: before %.1f after %.1f", beforeHealth, player.health)
	}
}
