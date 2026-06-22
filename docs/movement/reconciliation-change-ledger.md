# Apeiron Reconciliation Change Ledger

This ledger records movement and skill-movement reconciliation changes that affected rubberbanding, prediction, action root ownership, and automated validation.

## 2026-06-22 - Recovery Baseline After Project Deletion

### Symptom

The recovered projects had enough code to compile, but the historical movement/rubberband roadmaps were partially corrupted and the expected movement ledger file was missing.

### Source Of Truth

- Server authoritative movement remains the source of gameplay capsule position.
- DB/server action contracts are the source of movement durations, distances, speed curves, and reconciliation categories.
- Unreal may predict and present movement, but it must not invent missing action contracts.
- The strict Unreal movement validation scanner must not be weakened to make a run pass.

### Validated Baseline

- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build ./cmd/game-server`
- `db-apeiron`: `go test ./...`
- `db-apeiron`: `go build ./cmd/db-api`
- `PlainTestMapEditor`: C++ build passed.
- `Scripts/test_movement_validation.ps1 -InputPlayback -TimeoutSeconds 240`: passed.
- `Scripts/test_movement_validation.ps1 -FocusedInputPlayback -TimeoutSeconds 240`: passed.

## 2026-06-22 - Grounded Skill Identity Fix

### Symptom

Unreal logs showed:

- `action=grounded_skill`
- `reconciliation_mode=grounded_skill_action`
- `prediction_blocked_missing_contract skill=grounded_skill`

That meant snapshots were using the generic action channel as if it were a concrete skill id. The client then failed to find the real movement action contract for basic attack steps, Shield Bash, or Shield Rush.

### Hypothesis

The common denominator for basic/F/R rubber was not a skill-specific distance. It was loss of concrete skill identity between server snapshot and Unreal local prediction/replay.

### Change

- Unreal reconciliation string parsing now recognizes DB/server names such as `grounded_skill_action`, `grounded_skill`, `skill_grounded_action`, and `post_action_handoff`.
- Active grounded skill replay resolves the effective action key from `ability_key` when `action` is generic.
- Authoritative grounded skill snapshot application also prefers `ability_key` when `action` is `grounded_skill` or `grounded_skill_action`.

### Effect

Both automated movement validation suites passed after this slice when the game-server ran with movement validation isolation.

### Guardrail

Do not fix future skill rubber by increasing deadzones per skill or disabling lateral/diagonal movement. If `prediction_blocked_missing_contract` returns, inspect skill identity, contract manifest/payload, and snapshot locomotion fields first.

## 2026-06-22 - Movement Validation Isolation

### Symptom

The automated movement scanner failed with:

- `movement input playback ran with creature runtime active`

The player-only rubberband validation was running while a creature placeholder was spawned and updating.

### Change

- Added `gameapi.RuntimeOptions{MovementValidation}`.
- Added config support for `MOVEMENT_VALIDATION=true` and `-MovementValidation`.
- In movement validation mode, player attach does not spawn the wolf and snapshot ticks do not run creature policies.
- `RuntimeStats` reports actual spawned creature count.
- Added a server unit test proving the validation runtime does not spawn creatures.

### Effect

The scanner can now validate player movement/rubber without creature AI/contact noise.

### Guardrail

Normal gameplay server must run without movement validation enabled. Movement validation mode is a test/runtime isolation flag, not a gameplay rule.

## Next Required Scenarios

- Manual gameplay validation with wolf active after automated tests pass.
- Basic attack 1/2/3 stationary and after sprint/strafe.
- Shield Bash and Shield Rush during and after movement.
- Leap while hit and landing handoff.
- Dodge exit to movement.
- Aggressive `Shift + W/A/D` curves with mouse yaw.
- Creature lunge/maul/bite using the same action identity and contract publication language.

## 2026-06-22 - Movement Kinematics Back Under Movement Package

### Symptom

Recovery code still had several runtime-local movement formulas:

- grounded move computed speed/projection in `gameapi/runtime.go`;
- dodge/leap used local distance fallback plus an arbitrary velocity divisor;
- grounded skills used a separate velocity divisor;
- wolf movement stepped inline in the creature policy loop.

That shape can reintroduce exactly the old rubberband class: normal movement and skill movement look similar in snapshots but are not produced by the same movement authority.

### Change

- Added `internal/movement/kinematics.go`.
- `ResolveGroundedMove` owns walk/run/strafe speed caps and one-tick projection.
- `ResolveActionMotion` owns committed action distance/speed/projection for dodge, leap, basic attacks, Shield Bash, Shield Rush, and creature skills.
- `ResolveConstantStep` owns simple creature step motion.
- `gameapi/runtime.go` now applies those motion results instead of computing speed/distance/projection inline.
- Added unit tests for directional caps, action speed derivation, and creature step movement.

### Effect

The exposed game runtime has fewer local movement producers. Movement package now owns both locomotion policy and the core kinematics used by the current gameapi path.

### Guardrail

Do not add new movement formulas to `gameapi/runtime.go` or combat systems. If a future skill needs different movement, add a movement contract/profile and resolver behavior in `internal/movement`, then publish it through the same `LocomotionState` fields.

## 2026-06-22 - ActionInstance Restored In Game API Runtime

### Symptom

The recovered gameapi cast path created `SkillRuntimeState{State:"active"}` directly and ACK metadata did not carry a real action instance. That made the runtime look active, but not phase-owned.

### Change

- `entityState` now stores `combat/actionruntime.Instance`.
- `applySkill` creates an action instance for basic attacks and active skills using DB/recovered timing.
- `GetSnapshot` refreshes action phase and returns `complete/idle/ready` when the instance ends.
- Cast ACK metadata now includes `action_instance_id`, `action_kind`, `action_phase`, `movement_action_contract_id`, and the real contract hash.
- Added tests for ACK metadata and snapshot phase completion.

### Effect

The gameapi path now speaks the same action-instance language as the reconstructed combat runtime package, instead of inventing a one-word skill state.

### Guardrail

Future command gating must build on the action instance/channel model. Do not bring back ad hoc "locked" state strings or per-skill cooldown branches in the gameapi runtime.
