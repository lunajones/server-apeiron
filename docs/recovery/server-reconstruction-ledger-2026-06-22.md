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
