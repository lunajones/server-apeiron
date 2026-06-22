# Full Project Reconstruction Roadmap - 2026-06-22

This roadmap exists because the current recovery cannot trust one source blindly. The goal is to rebuild `db-apeiron`, `server-apeiron`, and the Unreal bridge from the latest Apeiron MMO trajectory, not from the old `Projetos\apeiron` attempts.

## Scope

Reconstruct the current Apeiron MMO stack under:

- DB: `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron`
- Server: `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron`
- Unreal: `B:\Unreal Projects\PlainTestMap`

Old projects outside `Projetos\mmo` are not source of truth. They can only be used as conceptual hints when a current-chat fact, current roadmap, or current recovered file independently confirms the direction.

## Source Precedence

1. Current projects in `Projetos\mmo` after the recovery commits.
2. Recent Codex threads for current `mmo` projects:
   - DB: `019e76bb-3b35-7b22-8ffe-b2a12484692e`
   - Server: `019e7944-658a-7980-a575-77905a6d65f2`
   - Server and Unreal reconciliation: `019e92d4-9a1f-7c00-b366-d79db34c9e4d`
   - Movement handoff: `019e9ac0-22bf-7bc1-8476-5be9f640c0e4`
   - Recent combined movement/combat thread: current conversation.
3. Roadmaps and reviews already recovered under current `server-apeiron\docs` and `db-apeiron\docs`.
4. `B:\ApeironRecoveredWorkspace_20260622_010210` parse-clean files.
5. WinFR output that is parse-clean and points to current `Projetos\mmo` paths.
6. VS Code history only when it is tied to current paths and not contradicted by newer chat/runtime facts.

Any recovered file containing NUL/binary garbage or foreign package content is not trusted as source. It may only prove that a path/name existed.

## Current Status

### DB

- Git remote registered locally: `https://github.com/lunajones/db-apeiron.git`.
- `go test ./...` currently passes.
- Generated gRPC files were restored from current protos.
- The current DB has only these proto services restored:
  - `CacheService`
  - `CreatureDataService`
- Repository packages exist for broader domain data:
  - skills
  - profiles
  - world
  - player
  - inventory
  - creature
- Therefore the DB data layer is wider than the restored gRPC service layer. That is an active reconstruction gap.

### Server

- Git remote registered locally: `https://github.com/lunajones/server-apeiron.git`.
- `go test ./...` currently passes.
- Many recovered server Go files were corrupt and were quarantined instead of copied into the project.
- Current server compiles in a reduced/reconstructed form. It still needs reconstruction of real app, DB clients, runtime region, game API, snapshot, movement, AI, combat runtime, and session/network surfaces from current chats and roadmaps.

### Unreal

- Current Unreal project remains at `B:\Unreal Projects\PlainTestMap`.
- Recent chat facts mention real C++ changes for:
  - `ApeironTestPlayerCharacter`
  - `ApeironGameServerBridge`
  - `ApeironCreaturePlaceholder`
- These must be audited against current files before assuming the bridge still has all post-recovery behavior.

## Recovered Critical Facts

### Authoritative Movement

- Server is authoritative.
- Client sends input, `sequence`, `client_tick`, direction/buttons/facing, and optional `desired_position` as prediction evidence only.
- Server must not obey `desired_position` as destination.
- Server ack/snapshot must expose enough information for client replay/reconciliation.
- Normal movement, sprint/strafe, turn, leap, dodge, post-action handoff, and skill movement need distinct reconciliation modes when their rules differ.
- Handoff must use server phase timeline (`phase_elapsed_ms`, `phase_remaining_ms`) instead of client-invented total duration.
- Dodge/leap state must be routed and cleaned separately; one cannot leave a ghost state that blocks walk.
- Camera/mesh visual correction must not move gameplay capsule unless the capsule really needs correction.

### Skill Movement

- Combat owns skill intent/timing/target/hit/cooldown.
- Movement owns locomotion/action state publication.
- Combat must not manually publish competing locomotion state for skill movement.
- Movement action contracts need absolute values, not only curves:
  - horizontal distance
  - base speed
  - timing windows
  - reconciliation category
- Shield Rush front contact starts close to the player, around half a player cylinder, so the push reads as body contact.
- Skill movement must not break leap, dodge, turn, or normal movement.

### Temporal Hitboxes

- Static full-arc activation is not final AAA melee.
- Server should resolve temporal/swept volumes from simple authoritative shapes, not mesh collision.
- Basic attack 1: forward strike from player body to about 1.5 player cylinders ahead.
- Basic attack 2: right-to-left temporal sword sweep across about 90 degrees.
- Basic attack 3: shield drive/contact carry.
- Shield Bash/Rush: front contact/push volumes following the player/action.
- Wolf lunge: target-facing leap/impact volume.

### Creature/Wolf

- Creature behavior is contract/policy driven, not wolf-only hardcode.
- `any` in creature skill bindings is a wildcard.
- `commit_attack` is offensive commitment.
- Wolf lunge needs `SkillMovementEffect` lookup by `skill_id`, because row `id=leap_default` can represent `skill_id=lunge`.
- Wolf bite/lunge hitboxes must use `target_direction`, not movement direction, while circling.
- Hitbox offsets use `offset_x` as forward and `offset_y` as lateral.
- Orbit side switching must be stable and policy-driven; do not switch every couple seconds by accident.
- Saved creature runtime territory from DB must be validated against current zone to avoid `aggressive` plus `return_home`.

### Player Weapon Kit

- Sword/shield has combat modes:
  - `Bulwark`
  - `Vanguard`
- Current intended loadout:
  - `Bulwark`: M1 basic combo, R shield bash, F shield rush, Q empty, G fatality placeholder disabled.
  - `Vanguard`: M1 basic combo only for now; Q/R/F empty until real skills exist.
- Mode switch is Ctrl and should be fast, about half of the earlier implementation.
- UI must show selected skills for the active mode only.

### Defense/Stamina

- Normal unblocked hits do not damage stamina.
- Stamina pressure applies through block/guard and stamina spenders.
- Dodge invulnerability should cover the whole dodge window from accepted input through end.
- Parry/block should be resolved through the damage/defense pipeline, but must be data-driven by defense contracts.

## Reconstruction Slices

### Phase 1 - Freeze And Validate Sources

Status: in progress.

- Keep both projects under Git before each recovery slice.
- Do not copy corrupt WinFR Go over current source.
- Record current chat facts into this roadmap and the existing ledgers.
- Find any `recuperacao 1..13` folders if the user provides the exact path.
- Run `go test ./...` on DB and server after each slice.

### Phase 2 - Complete DB Service Surface

Status: in progress.

Target: DB repositories, proto services, handlers, cache warmup, and generated Go must agree.

Restore or reconstruct:

- `SkillDataService` - restored 2026-06-22 for core skill, skill set, loadout, hitbox profile, and impact profile reads.
- `ProfileDataService` - restored 2026-06-22 for movement profile and combat core profile reads.
- `SkillDataService.GetSkillMovementEffect` - restored 2026-06-22 as compatibility endpoint keyed by `skill_id`, matching the recovered `lunge -> leap_default` contract from the DB thread.
- `WorldDataService` - restored 2026-06-22 for region, biome, and spawn-zone reads from the existing world repository/cache.
- `PlayerDataService`
- `InventoryDataService`
- `ObservabilityService`

2026-06-22 slice notes:

- Core API structs formerly in manual `gen/apeiron/v1/types.go` were moved into protobuf definitions in `proto/apeiron/v1/common.proto`.
- `types.go` now only keeps compatibility helpers such as `Skill.GetComboIndex`.
- DB `go test ./...` and server `go test ./...` both pass after regeneration.
- `CreatureDataService` handler was restored and registered after this note, using the template cache and deriving runtime profile IDs from the template.
- `GetSkillMovementEffect(skill_id)` was restored after chronological chat extraction showed this endpoint was the historical fix for wolf lunge movement. It returns DB `skill_movement_effect` rows by `skill_id` and maps them to `SkillMovementProfile` without inventing missing contract numbers.
- `WorldDataService` was restored after the map/world chat extraction confirmed the server needs authoritative exported world data, not visual Unreal assets.
- Remaining gap: `Player/Inventory/Observability` services are still absent.
- Remaining movement gap: this restored legacy endpoint is not the final skill movement contract model; Phase 4/6 still need the named movement action contract and temporal hitbox contract services reconstructed.

Use repository structs and current bootstrap/migrations as the source. Do not invent fields that are not in SQL or recovered runtime facts.

Done when:

- Protos generate cleanly.
- Handlers compile.
- `go test ./...` passes.
- Server can import generated DB clients without manual compatibility stubs for canonical service messages.

### Phase 3 - Rebuild Server DB Clients And App Runtime

Status: pending.

Target: game-server can consume DB services through explicit clients, not stubs.

Restore or reconstruct:

- DB client interfaces for skill/profile/world/player/inventory/creature/readiness.
- App lifecycle and readiness gates.
- Static data warmup.
- Runtime region/session/game API surfaces.

Done when:

- `go build ./cmd/game-server` passes.
- Server startup fails loudly on missing required DB contracts instead of inventing defaults.

### Phase 4 - Rebuild Movement/Reconciliation Architecture

Status: pending.

Target: movement architecture returns to the post-rubberband AAA direction.

Restore or reconstruct:

- Input command mapping with sequence/client tick.
- Movement resolver and action contracts.
- Named reconciliation modes:
  - grounded move
  - combat strafe
  - turn
  - leap airborne
  - leap landing handoff
  - dodge active
  - dodge exit handoff
  - grounded skill action
  - post-action handoff
- Analyzer/scanner for rubberband tests.

Done when:

- Tests cover sprint curves, A/D+Shift, W+A/W+D+Shift, leap, hit during leap, dodge, M1 combo, F, R, and movement after each.
- Client/server contract does not depend on magic C++ fallbacks for required movement data.

### Phase 5 - Rebuild Combat Action Runtime

Status: pending.

Target: one action instance language for player and creature.

Restore or reconstruct:

- Skill action timing, action lock, cooldown, queue, recovery.
- Basic attack separated from active skills where needed.
- Defense/parry/block/stamina pipeline.
- Creature attacks using the same phase/action vocabulary as player actions.

Done when:

- No two live systems own the same cooldown/lock/action state for migrated skills.
- Basic attack combo and active skills do not block each other beyond contract-defined windup/cast/recovery.

### Phase 6 - Rebuild Temporal Hitbox Runtime

Status: pending.

Target: melee damage follows the attack over time.

Restore or reconstruct:

- Motion samples and swept volume evaluation.
- One-hit-per-swing group rules.
- Runtime/debug events for progressive hitbox visualization.
- DB seeds for basic attacks, shield bash/rush, wolf lunge/maul/bite as current design requires.

Done when:

- Static full-arc hitboxes are not used for directional sword swings where temporal profiles exist.
- DB bootstrap and server runtime agree on profile IDs and shape semantics.

### Phase 7 - Rebuild Creature Brain

Status: pending.

Target: wolf feels intelligent without hardcoded wolf-only behavior.

Restore or reconstruct:

- Behavior contract loader.
- Skill setup policies.
- Evasion policy and stamina budget.
- Orbit/flank/retreat side stability.
- Lunge windup movement, airborne movement, landing inertia.
- Bite/maul counter opportunities under pressure.

Done when:

- Wolf can attack from correct ranges, lunge with target-facing movement/hitbox, dodge laterally/backward, and resume movement after landing.
- `no_ready_skill` is observability, not a fake miss that poisons behavior memory.

### Phase 8 - Audit Unreal Bridge

Status: pending.

Target: Unreal presentation and prediction match the recovered server contract.

Verify or reconstruct:

- Ctrl mode switch and hotbar update.
- HUD visual cleanup.
- Skill icons/assets references.
- Snapshot timeline, camera/mesh/capsule handoff, dodge/leap route cleanup.
- Automated movement tests and scanner thresholds.

Done when:

- Unreal build passes.
- Automated tests visibly move the character in the intended scenarios.
- Manual test confirms no regression in leap, dodge, turn, sprint strafe, F/R, and basic attacks.

## Active Risks

- Some recovered docs under current `server-apeiron\docs\roadmap` contain binary/NUL corruption and cannot be trusted as content.
- Server currently passes tests but is not yet proof of full historical runtime completeness.
- DB current service layer is thinner than its repositories and seeds.
- Git remotes exist locally, but pushes require GitHub authentication.

## Non-Negotiable Safety Rules

- No broad delete or cleanup. Quarantine instead.
- No project-root deletion for logs.
- No permanent deletion of source, migrations, seeds, protos, generated code, Unreal source/content/config, skills, or recovery output.
- No commit immediately after any destructive action until sentinel audit proves no unintended damage.
- Do not use hardcoded runtime fallbacks where a required DB/proto contract should exist.
