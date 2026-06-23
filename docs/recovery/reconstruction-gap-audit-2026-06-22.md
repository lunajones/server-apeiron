# Apeiron Reconstruction Gap Audit - 2026-06-22

Purpose: separate "project compiles" from "the pre-deletion Apeiron gameplay architecture is restored".

## P0 Gaps

### Movement Resolver Ownership Is Not Restored

Source threads:
- `recuperacao 5` / `019ed913-f0c2-7960-914d-c3d4ec407072`
- `recuperacao 1` / `019ee718-0dbd-7791-b75e-32015f3ca5d8`

Expected architecture:
- Combat owns skill start, timing, target, direction, hitbox, cooldown, lock, pending state.
- Movement owns locomotion publication: `Action`, `MovementMode`, `Phase`, `ReconciliationMode`, `TargetSpeed`, `EffectiveSpeed`, `PhaseSpeedScale`, `ActionDistanceTraveled`, `ActionProjectedPosition`, `ActionStartPosition`.
- Skill movement should be published through one movement-owned path.

Current recovered code evidence:
- `internal/movement/resolver.go` has been reconstructed and now owns the canonical locomotion field set.
- `internal/movement/kinematics.go` now owns grounded move, committed action motion, and creature constant-step kinematics.
- `internal/gameapi/runtime.go` now delegates normal move, dodge/leap, grounded skill movement, and wolf step motion to `internal/movement` instead of computing speed/distance/projection inline.
- `internal/combat/player_skill_combat_system.go` still applies skill movement and calls `syncPendingPlayerSkillState`.
- `internal/gameapi/runtime.go` still serializes resolver output into `LocomotionState`; that is the wire boundary, not a second movement rule owner.

Risk:
- `internal/combat/player_skill_combat_system.go` still needs a full ownership audit so it does not become a second runtime path for migrated player skills.
- The current gameapi runtime is now safer, but the broader combat package still contains recovered fallback profile paths.

Required reconstruction:
- Continue migrating combat-side skill movement publication to the movement resolver.
- Delete or demote combat-side locomotion publishing once movement owns the full non-vertical-slice path.
- Keep and expand tests for field parity listed above.

### Recovered Runtime Fallback Is Still Production-Reachable

Source threads:
- `recuperacao 11` / `019e97f2-1f69-7222-a875-ff1fa9bf074b`
- current code audit

Status:
- Mitigated in the app/gameapi boot path on 2026-06-22.
- Keep this item open only for the broader combat fallback audit below.

Expected architecture:
- DB/profile contracts are authoritative for tuning shared by server and Unreal.
- Fallbacks can exist only as explicit dev/recovery mode, never as silent production behavior.

Current recovered code evidence:
- `internal/app/lifecycle.go` now loads DB contracts first and rejects startup without DB unless `ALLOW_RECOVERED_RUNTIME_FALLBACK=true` is explicitly set.
- `internal/gameapi/runtime.go` only backfills missing runtime contract groups when the contract source is an explicit recovered fallback source.
- `internal/gameapi/contracts.go` labels complete strict DB loads as `db_contracts`.
- `internal/gameapi/contracts.go` declares source `recovered_runtime_fallback`.

Risk:
- Remaining risk is no longer silent app boot fallback; it is combat-package fallback attack profiles and any vertical-slice code path that bypasses `app.Run`.

Required reconstruction:
- Keep strict startup mode for required DB contract coverage.
- Keep fallback behind explicit recovery/dev flag only.
- Log and fail loudly when required production contracts are missing.
- Add readiness report listing every required action/skill/creature/combat mode contract and its source.

### Basic Attack And Player Skill Fallback Profiles Still Exist

Source threads:
- `recuperacao 6` / `019ed02a-86f2-79d2-bcd6-0a479bd27b81`
- current code audit

Expected architecture:
- Basic attack is a first-class combo alias resolved by server into `player_basic_attack_1/2/3`.
- Each stage is a real skill/profile from DB, with movement/timing/hitbox/defense contract.

Current recovered code evidence:
- `internal/combat/player_skill_combat_system.go` still contains `AllowFallbackAttack` and `fallbackPlayerAttackProfile`.
- Fallback literals include damage, range, cooldown, hitbox shape, length, angle, max targets.

Risk:
- Missing DB profile can still produce a "working" attack that is not the intended combo.
- Tuning can diverge from DB and from Unreal visuals.

Required reconstruction:
- Make fallback attack profiles dev/recovery-only.
- For normal runtime, missing player skill profile must reject command with explicit contract error.
- Add tests proving `player_basic_attack` alias resolves to stage contracts and never to fallback in strict mode.

## P1 Gaps

### Runtime Action State Machine And Action Channels Are Partial

Source thread:
- `recuperacao 11` / `019e97f2-1f69-7222-a875-ff1fa9bf074b`

Expected:
- Formal phases: `requested -> accepted -> startup -> active -> recovery -> handoff -> complete`, plus `rejected`, `interrupted`, `cancelled`.
- Runtime channel occupancy in server: movement, rotation, combat/action, defense, status/CC.
- Static channel policy can come from DB.

Current evidence:
- `internal/movement/action_contract_registry.go` has reconciliation category/contract lookup.
- `internal/combat/actionruntime.Instance` is now used by `internal/gameapi/runtime.go` for cast/basic action identity, phase, cooldown endpoint, and ACK metadata.
- Unreal bridge consumes `action_channel` metadata.
- No complete server runtime channel occupancy/rejection audit has been verified.

Required:
- Audit command gates and action locks against formal channels.
- Add state machine tests for coexistence and rejection.

### Temporal Melee Hit Volumes Need Completion Across All Skills

Source threads:
- `recuperacao 9`
- current temporal hitbox roadmap docs

Expected:
- Directional melee damage should progress over time, not activate a whole static arc instantly.
- Server resolves simplified temporal/swept volumes, not visual mesh collision.

Current evidence:
- Hitbox runtime now has forward target ordering and `MaxTargets`.
- Need full sweep/timeline coverage audit for every current player and wolf skill.

Required:
- Verify basic attack 1/2/3, Shield Bash, Shield Rush, wolf lunge, bite, maul against temporal hit volume model.
- Add per-skill tests for timing, direction, max targets, and one-hit-per-swing behavior.

### Creature Brain/Runtime Parity Needs Audit

Source threads:
- `recuperacao 7`, `recuperacao 8`, `recuperacao 9`

Expected:
- Wolf uses the same action language as player where possible: phase, lock, cooldown, movement intent, recovery/handoff.
- Wolf lunge has pre-run, airborne travel, pass-through behavior, landing inertia.
- Target selection is for AI/facing only; hitbox decides hit.

Current evidence:
- Current simplified `gameapi/runtime.go` contains direct wolf locomotion/action logic.
- Combat package creature system exists, but parity with gameapi runtime has not been fully audited.

Required:
- Decide one authoritative runtime path for creature combat in current recovered server.
- Remove or isolate old/direct wolf behavior if it duplicates combat/AI runtime.
- Add lunge tests for damage timing, pass-through, landing inertia, and post-landing action decision.

## P2 Gaps

### Impact Response Profile Needs End-To-End Confirmation

Source thread:
- `recuperacao 12`

Recovered evidence:
- `CombatDefenseContract` API exists.
- `impact_response_profile` is referenced by roadmap, but end-to-end DB -> server snapshot/event -> Unreal VFX must be revalidated.

Required:
- Confirm creature template field, proto, mapper, snapshot, event metadata, and Unreal selection are all present.
- Add at least one non-flesh creature test once a skeleton/stone template exists.

### HUD/Combat Mode Source Of Truth Needs Completion

Source threads:
- `recuperacao 1`, `recuperacao 2`

Expected:
- HUD follows Apeiron identity, not Witcher clone.
- CTRL toggles active combat mode; HUD shows authoritative mode slots only.
- Empty mode slots show localized notification, not fake skills.

Required:
- Confirm current Unreal HUD has no dev lock text.
- Confirm combat mode ACK is consumed and local fallback cannot pin the wrong mode.
- Add localization keys for empty mode/slot messages.

## Immediate Recovery Plan

1. Finish paging remaining source threads and update `thread-source-index-2026-06-22.md`.
2. Rebuild/restore movement resolver ownership before more rubberband tuning.
3. Harden fallback policy: DB contracts required outside explicit recovery mode. (app/gameapi boot path done; readiness report still pending)
4. Reconnect basic attack/skill strict profile loading and remove final gameplay fallbacks.
5. Audit `gameapi/runtime.go` versus combat/domain packages; decide whether it is a temporary vertical-slice runtime or the real server runtime.
6. Run `go test ./...` in `server-apeiron` and `db-apeiron`, then Unreal build.
