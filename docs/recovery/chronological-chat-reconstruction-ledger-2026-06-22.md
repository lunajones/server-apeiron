# Chronological Chat Reconstruction Ledger - 2026-06-22

This ledger captures recoverable facts from Codex thread history and cross-checks them against the current `Projetos\mmo` reconstruction. It intentionally ignores old non-`mmo` projects as authoritative code sources.

## Source Order

Threads are read from older project phase toward newer project phase. Some Codex thread pages are returned newest-first, so each extraction records the thread and page cursor.

Current thread IDs:

- DB: `019e76bb-3b35-7b22-8ffe-b2a12484692e`
- server: `019e7944-658a-7980-a575-77905a6d65f2`
- first/map pipeline: `019e808b-ac89-77d0-b9a0-08f669796584`
- server + Unreal reconciliation: `019e92d4-9a1f-7c00-b366-d79db34c9e4d`
- movement handoff: `019e9ac0-22bf-7bc1-8476-5be9f640c0e4`
- recent/current: current recovery conversation

## DB Thread Extracts

### Creature Attack Runtime, Lunge, Bite, And Skill Readiness

Source thread: `019e76bb-3b35-7b22-8ffe-b2a12484692e`

Observed facts from recent DB pages:

- Wolf perception/faction was not the core blocker when the wolf circled but did not attack. Runtime stats showed active player, target acquired, and combat attempts.
- `lunge` existed as skill/hitbox but `SkillMovementEffect(lunge)` returned empty because the DB endpoint looked up the movement effect by `id`, while the row was keyed as `id=leap_default`, `skill_id=lunge`.
- Correct contract behavior: game-server asks by `skill_id`, and DB/cache must return the canonical movement effect for that skill.
- Recovered expected `lunge` movement effect values at that time:
  - `id=leap_default`
  - `skill_id=lunge`
  - `movement_type=leap`
  - `distance=420`
  - `speed=1400`
  - `duration_ms=300`
- Wolf `bite` was filtered out around the real combat range because skill/slot/contract range was too short:
  - runtime orbit range was around `240 cm`
  - `bite.maxRange` / slot distance needed to support that near-melee range, recovered as `260`
- Creature behavior contract needed offensive bindings for `bite` and `lunge` during `approach/reposition` and `circle/reposition`, otherwise the wolf could keep circling with no ready skill.
- `no_ready_skill` is not a real attack miss. It must not poison behavior memory as if the creature attacked and failed.
- Runtime diagnostics were added historically for `candidate_skills` and `cooldown_skills` to distinguish empty binding from cooldown.

### Wolf Hitbox Direction And Geometry

Source thread: `019e76bb-3b35-7b22-8ffe-b2a12484692e`

Observed facts:

- `lunge` and `bite` hitboxes must use `orientation_source=target_direction`, not `movement_direction`.
- Reason: while the wolf circles, `movement_direction` points tangentially and can make the attack fire sideways even though the target is valid.
- Hitbox offsets use:
  - `offset_x` as forward
  - `offset_y` as lateral
- A geometry regression test was historically created with `lunge` at about `241 cm`, `target_direction`, `offsetX=100`, `length=240`, `radius=58`; expected result: hit.
- If that geometry test passes but runtime misses, suspect timing/position/faction/active-window rather than DB shape.

### Wolf Attack Angles And Back Attacks

Source thread: `019e76bb-3b35-7b22-8ffe-b2a12484692e`

Observed facts:

- Wolf commit angle policy was too restrictive for attacking the player from behind.
- A recovered correction raised `commit_angle_max_deg` from `145` to `180` in the wolf behavior contract.
- Correct direction: attacking from lateral/back should be allowed by policy when skill/range/readiness are valid; the wolf should not require perfect melee frontal alignment.

## Reconstruction Implications

- DB must expose skill movement effects by `skill_id`, even if the row's primary `id` is a reusable movement profile like `leap_default`.
  - Restored 2026-06-22 in `SkillDataService.GetSkillMovementEffect`.
- DB service surface must include movement effects, skill hitbox profiles, action runtime/timing, and creature behavior contracts because the server runtime depends on those as data, not hardcoded values.
- Server creature combat diagnostics should distinguish:
  - no candidate skill
  - candidates all in cooldown/action lock
  - hitbox inactive/missing
  - geometry miss
  - faction/target filter miss
- The wolf behavior must remain contract/policy driven. Do not restore these as wolf-only Go branches.

## Server Thread Extracts

### Authoritative Movement And `desired_position`

Source thread: `019e7944-658a-7980-a575-77905a6d65f2`

Observed facts from server movement pages:

- Apeiron movement target architecture is server-authoritative input prediction.
- Client sends input, not authoritative position:
  - `sequence`
  - `client_tick`
  - movement direction
  - buttons/action type
  - camera/facing where relevant
  - optional `desired_position` / predicted position
- `desired_position` must never be used by the server as a destination or teleport directive.
- `desired_position` is still useful and should not be ignored. Its role is client prediction evidence for error measurement, diagnostics, and reconciliation.
- Server simulates using authoritative rules:
  - direction
  - speed/acceleration
  - elapsed time
  - navmesh/collision
  - stamina/resources
  - dodge/leap/skill movement rules
- Server response must provide enough information for client replay/reconciliation:
  - server position
  - velocity
  - rotation/facing when relevant
  - `last_processed_input_sequence`
  - `last_processed_client_tick`
  - correction/error metadata
- Recovered ack metadata contract names:
  - `movement_protocol=server_authoritative_input_prediction_v1`
  - `desired_position_role=client_prediction_only`
  - `movement_sequence`
  - `client_tick`
  - `direction`
  - `desired_position`
- `RuntimeStats.phase_status` historically exposed last movement processed, server position, velocity, error, and correction reason for debugging.

### Movement AAA Status From Earlier Thread

Source thread: `019e7944-658a-7980-a575-77905a6d65f2`

Recovered judgment at the time:

- Server-side movement was considered on the right architectural path after `desired_position` was reclassified as prediction-only.
- Not AAA complete until Unreal implemented matching client prediction/reconciliation by sequence.
- Known risks if Unreal side is wrong:
  - snapping to raw snapshot instead of replaying pending inputs
  - local/world axis mismatch for direction
  - predicted position sourced from visual mesh instead of gameplay capsule/root
  - snapshots applied as present-time truth instead of delayed/interpolated state
  - dodge/jump/basic movement using different prediction paths
  - creature targets using server-authoritative capsule while camera shows predicted local capsule

## Server Reconstruction Implications

- Reconstruct movement code around one invariant: server accepts input and reports authoritative results; it does not follow client position.
- Do not remove `desired_position`; keep it as a diagnostic/reconciliation input.
- Movement tests must assert both sides:
  - predicted position is preserved/available
  - predicted position does not drive authoritative movement
- Snapshot/ack proto surface should eventually expose structured `last_processed_input_sequence` and correction info instead of relying only on metadata/stats.
