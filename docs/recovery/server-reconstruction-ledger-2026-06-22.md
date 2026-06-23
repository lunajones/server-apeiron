# Server Reconstruction Ledger - 2026-06-22

This file records the server-apeiron reconstruction after data loss.
Use it together with the DB ledger at `../db-apeiron/docs/recovery/chat-recovery-ledger-2026-06-22.md`.

## Source Precedence

1. Current thread and latest runtime-tested reports.
2. Recent Codex server threads:
   - `continuar daqui dia 10` (`019e92d3-b2e2-7162-b129-c1c4a681f5a2`)
   - `Separar handoff de movimento` (`019e9ac0-22bf-7bc1-8476-5be9f640c0e4`)
   - `SERVER + unreal combinado` (`019e92d4-9a1f-7c00-b366-d79db34c9e4d`)
3. DB thread facts where server behavior depends on DB contracts.
4. WinFR recovered files that parse cleanly.
5. Old VS Code History files from `Projetos/apeiron`, only as conceptual scaffolding when the modern server source is missing.

Do not let an older recovered file override a later chat fact that was tested in runtime.

## Git Checkpoint

- Remote: `https://github.com/lunajones/server-apeiron.git`
- First local recovery commit: `70e656d restore recoverable server apeiron sources`

Push requires GitHub authentication through `gh auth login` or a PAT. Do not use a password in scripts.

## WinFR Recovery Result

- WinFR source: `B:\ApeironWinFR_Server`
- Files under original `server-apeiron` path: `173`
- Go files under original path: `149`
- Go files passing `gofmt -e`: `63`
- Go files rejected/corrupt: `86`

The manifest files are:

- `docs/recovery/server-winfr-inventory-2026-06-22.tsv`
- `docs/recovery/server-winfr-go-parse-2026-06-22.tsv`

Only parse-clean Go files were restored into the project. Corrupt files must not be copied over
without manual reconstruction.

## Current Restored Server State

Restored and committed:

- `cmd/game-server/main.go`
- `internal/config/config.go`
- `internal/app/shutdown.go`
- `internal/clock/*` partial
- `internal/combat/player_skill_combat_system.go` and small support files
- `internal/controllers/*` partial
- `internal/dbapeiron/connection.go`, `retry.go`, mappers, errors
- `internal/errors/*`
- `internal/gamefsm/core_fsms.go`
- `internal/hitbox/motion.go`, `errors.go`
- partial navigation/pvp/result/systems packages
- roadmaps/reviews/system docs recovered from WinFR

Known corrupt or missing areas:

- `internal/app/lifecycle.go`, `Run`
- `internal/logging/logger.go`
- `internal/config/env.go`
- `internal/clock/fixed_tick_loop.go`, `tick_config.go`, `TickContext`
- `internal/fsm/fsm.go`
- `internal/domain/ids`, `internal/domain/math`, `internal/domain/entity`
- most of `internal/movement`, including resolver, contract registry, action locomotion state, sync, command mapper, event log
- `internal/runtime/region`
- `internal/skill`
- `internal/spatial`
- `internal/snapshot`
- `internal/gameapi`, `internal/network`, `internal/session`
- most DB clients in `internal/dbapeiron`
- generated DB proto package `db-apeiron/gen/apeiron/v1`

## Cross DB/Server Reconstruction Rules

- DB migrations/seeds define contracts and static data.
- Server consumes the DB contracts and must not invent fallback values when a contract is required.
- If DB lacks generated proto code, regenerate or reconstruct `db-apeiron/gen/apeiron/v1` before expecting server build to pass.
- If server requires a field that appears in chat but not in reconstructed DB migrations, restore the DB migration/seed first.

## Runtime Facts Recovered From Threads

### Build/Test Target

The recent server thread confirms the project previously reached:

- `go test ./...` passing
- `go build ./cmd/game-server` passing
- Unreal build passing
- DB on `50051`
- Server on `50052`

The recovery target is to return to that state, not merely to compile partial packages.

### Defense / Stamina / Parry

From `continuar daqui dia 10`:

- Normal unblocked hit must not damage stamina.
- Stamina pressure only applies through valid block/guard.
- Block direction should be symmetric around defender facing, not attack trace aim.
- Server publishes `target_parry_window_delta_ms`.
- Unreal consumes/logs:
  - `stamina_damage`
  - `parry_failure`
  - `source_reaction`
  - `target_defense_provider`
  - `target_parry_window_delta_ms`
- Valid block should mitigate HP damage according to defense contract.

Files observed in that runtime:

- `internal/combat/pipeline.go`
- `internal/combat/pipeline_test.go`
- `internal/combat/creature_combat_system.go`
- `internal/combat/types.go`
- `internal/snapshot/events.go`
- `internal/snapshot/types.go`

### Movement / Reconciliation

From `Separar handoff de movimento`, `SERVER + unreal combinado`, and current thread:

- Server movement is authoritative.
- Unreal prediction must be reconciled against server snapshots/acks.
- Snapshot bridge must carry locomotion/action state end-to-end.
- `ActionMove` zero direction and stale `client_position` cannot remain authoritative after actions.
- Normal movement, dodge, leap, turn, and skill movement need distinct reconciliation ownership when rules differ.
- DB movement action contracts carry absolute magnitudes, not only curves:
  - `horizontal_distance_cm`
  - `base_speed_cm_per_sec`
- Known important state:
  - `ActionRootHistory`
  - `action_distance_traveled`
  - `action_projected_position`
  - visual-only correction separated from gameplay capsule
  - mesh/camera correction split
  - `CameraFocusHandoff`

### Creature Behavior

From DB/server chats:

- Wolf lunge needs movement effect/action contract and forward hitbox orientation.
- `any` in creature skill binding matching is wildcard.
- `commit_attack` is offensive commitment.
- Orbit side switching must be runtime policy-driven, not deterministic `TargetID%2`.
- Wolf behavior contract includes opportunity policy and orbit locomotion mode.

### Weapon Kit / Combat Modes

From current thread:

- Sword/shield has at least `Vanguard` and `Bulwark`.
- Current active loadout:
  - `Bulwark`: `R = player_shield_bash`, `F = player_shield_rush`, `Q = empty`, `M1 = basic combo`
  - `Vanguard`: `Q/R/F` empty, `M1 = basic combo`
- Combat mode switch target is about `250ms`.

## Immediate Reconstruction Order

1. Rebuild/generated DB proto package from reconstructed DB proto definitions.
2. Restore foundational server types:
   - `domain/ids`
   - `domain/math`
   - `domain/entity`
   - `logging`
   - `config/env`
   - `clock` loop/config/context
   - `fsm`
3. Restore runtime region/snapshot/skill/spatial enough for `player_skill_combat_system.go`.
4. Restore movement architecture from roadmaps and chat facts.
5. Restore DB API clients matching reconstructed DB service surfaces.
6. Run `go test ./...`; use the errors as the authoritative missing-symbol checklist.
7. Only after server and DB compile, restart services and verify runtime readiness.
# 2026-06-22 - Runtime contract recovery slice

## Decision

The recovered game API must not keep movement, skill movement, and wolf behavior as scattered literals inside `internal/gameapi/runtime.go`.

Current source-of-truth order:

1. `db-apeiron` gRPC contract endpoints, when `DB_APEIRON_ENDPOINT` is configured and ready.
2. `gameapi.RecoveredRuntimeContracts()` only as explicit recovery fallback so the game can still boot while DB is offline or still being reconstructed.

## Implemented

- `db-apeiron` now exposes:
  - `SkillDataService/GetSkillActionTiming`
  - `SkillDataService/GetSkillMovementActionBinding`
  - `ProfileDataService/GetMovementActionContract`
  - `ProfileDataService/GetMovementReconciliationContract`
  - `ProfileDataService/GetCreatureBehaviorRuntimeContract`
  - `ProfileDataService/GetCreatureEvasionPolicies`
  - `ProfileDataService/GetCreatureSkillSetupPolicies`
- `db-apeiron/bootstrap/014_action_runtime_contract_seed.sql` includes base movement action contracts for `move`, `turn`, `dodge`, and `jump`, plus recovered player/wolf skill movement contracts.
- `server-apeiron/internal/gameapi/contracts.go` centralizes recovered and DB-loaded runtime contracts.
- `server-apeiron/internal/gameapi/runtime.go` now consumes `RuntimeContracts` for movement manifests, locomotion payloads, skill distances, dodge/jump distances, movement reconciliation profile, combat mode slots, and wolf policy values.
- Basic attack ACK metadata now reports the resolved combo step (`player_basic_attack_1`, etc.) instead of the generic `player_basic_attack` request alias.
- Movement action manifest ordering is deterministic: `move`, `turn`, `dodge`, `jump`, basic combo, shield skills, then any extra sorted keys.

## Validated

- `db-apeiron`: `go test ./...`
- `db-apeiron`: `go build ./cmd/db-api`
- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build ./cmd/game-server`

# 2026-06-22 - Movement validation isolation and grounded skill identity slice

## Finding

The recovered movement validation script was still running against a live creature runtime. That made player rubberband validation ambiguous and caused the strict scanner to abort with `creature_placeholders=[1-9]`.

The latest Unreal logs also showed grounded skill snapshots arriving as generic `action=grounded_skill` while the client tried to find a local movement contract for that generic action. That produced `prediction_blocked_missing_contract skill=grounded_skill` and could reintroduce visible pullback on basic attack, Shield Bash, and Shield Rush even though the real skill contracts existed.

## Implemented

- Added `gameapi.RuntimeOptions{MovementValidation}`.
- Added `MOVEMENT_VALIDATION=true` and `-MovementValidation` config support.
- In movement validation mode, `AttachPlayer` no longer spawns the wolf and snapshot ticks skip creature policy updates.
- `RuntimeStats` now reports the real spawned creature count instead of a literal recovered placeholder count.
- Added unit coverage proving movement validation runtime does not spawn creatures.
- Updated Unreal reconciliation parsing to recognize server contract names such as `grounded_skill_action`, `grounded_skill`, and `post_action_handoff`.
- Updated Unreal grounded skill reconciliation to resolve the effective skill key from `ability_key` when `action` is a generic grounded skill channel.

## Validated

- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build ./cmd/game-server`
- `PlainTestMapEditor`: C++ build passed.
- `Scripts/test_movement_validation.ps1 -InputPlayback -TimeoutSeconds 240`: passed strict scan.
- `Scripts/test_movement_validation.ps1 -FocusedInputPlayback -TimeoutSeconds 240`: passed strict scan.

## Guardrail

Do not weaken the rubberband scanner to make validation pass. If `PredictionDrift`, `RubberbandProbe`, command rejection, or missing contract signatures return, fix the authority/prediction contract path and rerun both automated suites.

# 2026-06-22 - Temporal hitbox guard slice

## Implemented

- Added direct unit coverage for `hitbox.ShapeFromMotionProfile` using a forward `capsule_strip` timeline sweep.
- The test verifies normalized time progression, motion profile ID, damage group ID, forward capsule advancement, and radius interpolation.

## Why This Matters

This protects the recovered "hit follows the swing over time" system from silently regressing back to static full-shape activation for temporal melee profiles.

## Validated

- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build ./cmd/game-server`
- Live `game-server` on `127.0.0.1:50052`:
  - `ObservabilityService/Health`
  - `SessionService/OpenSession`
  - `SessionService/AttachPlayer`
  - `CommandService/SubmitCommand` for `player_shield_rush`
  - `SnapshotService/GetSnapshot`

## Remaining Recovery Notes

- The currently running server is using `recovered_runtime_fallback` because `DB_APEIRON_ENDPOINT` is not configured in this shell.
- Once `db-api` and Postgres are running with seeds applied, restart `game-server` with `DB_APEIRON_ENDPOINT=127.0.0.1:50051` to validate the DB-backed path.
- This slice restores runtime shape and contract delivery; it does not prove the full historical combat implementation is completely recovered yet.

# 2026-06-22 - Movement contract registry recovery slice

## Recovery Source Audit

`B:\ApeironRecoveredWorkspace_20260622_010210` and both WinFR server passes contain many files with valid historical names but corrupted contents. Examples include `.go` paths under `internal/ai`, `internal/combat`, and `internal/movement` containing VS Code extension grammar/package data plus NUL bytes instead of Go code.

Because of that, missing recovered files must not be copied by path name. Recovery rule from this point:

1. Accept recovered source only when it has no NUL bytes, contains the expected language header, and passes the language parser.
2. Prefer the current git checkpoint when recovered content is older or smaller than current reconstructed code.
3. Rebuild missing runtime behavior through small tested slices rather than overwriting project files with WinFR output.

## Implemented

- Added `internal/movement.ActionContractRegistry`.
- Moved action contract ordering, family classification, contract hash, and reconciliation-mode resolution into `internal/movement`.
- Updated `internal/gameapi` to consume those movement helpers instead of duplicating contract classification logic locally.
- Added unit tests for preferred manifest ordering and skill-vs-movement contract classification.

## Validated

- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build ./cmd/game-server`

## Why This Matters

The historical rubberbanding work repeatedly failed when movement authority was split across multiple paths. This slice restores a single movement-owned contract classification point so `gameapi` can publish manifests and locomotion snapshots without owning reconciliation semantics itself.

# 2026-06-22 - Strict runtime contract boot slice

## Finding

The app boot path still initialized `gameapi.RecoveredRuntimeContracts()` before trying DB-backed runtime contracts. That meant a missing DB endpoint or partial recovered startup could look playable while using recovery-only values for movement, skill movement, wolf policy, and combat mode slots.

## Implemented

- Removed `ALLOW_RECOVERED_RUNTIME_FALLBACK` / `-AllowRecoveredRuntimeFallback` as a boot path.
- `internal/app/lifecycle.go` now refuses to load game runtime contracts without DB.
- Complete strict DB loads are labeled `db_contracts`.
- `gameapi.NewRuntimeWithOptions` no longer backfills missing contract groups from recovered runtime.
- `RuntimeContracts.contractForAbility` and `skillContract` no longer invent missing contracts for strict DB sources.
- `LoadRuntimeContractsFromDB` starts from an empty strict contract container, not from `RecoveredRuntimeContracts()`.

## Validated

- `server-apeiron`: `GOMAXPROCS=2 go test -p 1 ./...`
- `server-apeiron`: `go build -o bin/game-server.exe ./cmd/game-server`

## Remaining Recovery Notes

- `movement_reconciliation_contract` remains the per-action ownership category contract. The rich Unreal-facing profile is now tracked separately as `runtime_movement_reconciliation_profile`.
- `RecoveredRuntimeContracts()` remains only as a test/recovery fixture for old unit scenarios. It is not reachable from app boot.

# 2026-06-22 - DB-authoritative runtime movement reconciliation profile slice

## Finding

`gameapi.RecoveredRuntimeContracts()` still owned the rich `MovementReconciliationProfile` values sent in player snapshots. DB had `movement_reconciliation_contract`, but that table only describes per-action reconciliation ownership and cannot represent the full Unreal-facing profile fields such as grounded deadzones, leap/dodge handoff tolerances, submit intervals, visual smoothing, and strafe/backpedal sprint multipliers.

## Implemented

- `db-apeiron` now has `runtime_movement_reconciliation_profile`.
- `db-apeiron` proto now exposes `RuntimeMovementReconciliationProfile`.
- `ProfileDataService/GetRuntimeMovementReconciliationProfile` serves the rich profile.
- Seed `player_default_movement_profile` carries the restored values previously embedded in the recovered server runtime.
- `server-apeiron` strict DB contract loading now requires `player_default_movement_profile`.
- `server-apeiron` maps the DB profile into `apeiron.game.v1.MovementReconciliationProfile`.
- `gameapi.Runtime.movementSpeedProfile` no longer fills zero DB profile fields from `recoveredMovementProfile()`.

## Validated

- `db-apeiron`: `go test ./...`
- `db-apeiron`: `go build -o bin/db-api.exe ./cmd/db-api`
- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build -o bin/game-server.exe ./cmd/game-server`

## Runtime Validation Needed

- Restart DB API and game server so bootstrap applies `020_runtime_movement_reconciliation_profile_seed.sql`.
- Confirm `ProfileDataService/GetRuntimeMovementReconciliationProfile` returns `player_default_movement_profile`.

# 2026-06-22 - Migrated player skill fallback guard slice

## Finding

The combat package already blocked recovered fallback profiles for Shield Bash, Shield Rush, and basic attack 3, but basic attack 1/2 and the generic `player_basic_attack` alias were not in that migrated-skill guard. If `AllowFallbackAttack` was enabled for recovery, those migrated combo stages could still fabricate a generic melee profile.

Follow-up on 2026-06-22 removed the whole player attack profile fallback path instead of maintaining a per-skill blocklist.

## Implemented

- Removed `AllowFallbackAttack`.
- Removed `fallbackPlayerAttackProfile`.
- Missing skill/profile/hitbox runtime data now records `contract_missing` and returns the incomplete profile instead of fabricating damage, hitbox, cooldown, impact, source core, or target core data.
- Removed the migrated-skill fallback guard and its tests because there is no longer a generic player attack fallback to guard.

## Validated

- `server-apeiron`: `GOMAXPROCS=2 go test -p 1 ./internal/combat`
- `server-apeiron`: `GOMAXPROCS=2 go test -p 1 ./...`

## Decision

Combat profile values must come from DB-backed skill/profile/hitbox/impact/core data. If that data is missing, the system must surface the missing contract instead of making the attack appear functional.

## Implemented

- Extended the migrated profile fallback guard to include:
  - `player_basic_attack`
  - `player_basic_attack_1`
  - `player_basic_attack_2`
  - `player_basic_attack_3`
  - `player_shield_bash`
  - `player_shield_rush`
- Added unit coverage proving migrated player skills never use recovered profile fallback, while non-migrated temporary recovery skills can still use the explicit recovery path.

## Validated

- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build -o bin/game-server.exe ./cmd/game-server`

# 2026-06-22 - Wolf maul contract recovery slice

## Finding

The reconstructed DB already had `maul` as a wolf skill, timing, skill slot, and setup policy, but it did not have a movement action binding. That left the game runtime able to select `maul` conceptually while still publishing lunge-shaped movement fields in `CreatureAIState`.

## Implemented

- Added DB seed `wolf_maul_lateral_counter_v1` as a grounded skill movement contract.
- Bound `maul` to `wolf_maul_lateral_counter_v1` through `skill_movement_action_binding`.
- Added `wolf_bite_melee_commit_v1` so bite also has explicit timing/movement contract language even without displacement.
- Updated `gameapi` DB contract loading to include `maul`.
- Added recovered runtime fallback contracts for `bite`, `lunge`, `wolf_dodge`, and `maul`.
- Updated wolf snapshot publication so selected creature skill timing/movement fields come from the selected skill contract instead of always using lunge values.

## Validated

- `db-apeiron`: `go test ./...`
- `db-apeiron`: `go build ./cmd/db-api`
- `server-apeiron`: `go test ./...`
- `server-apeiron`: `go build ./cmd/game-server`
