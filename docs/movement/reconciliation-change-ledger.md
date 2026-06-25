# Apeiron Reconciliation Change Ledger

This ledger records movement and skill-movement reconciliation changes that affected rubberbanding, prediction, action root ownership, and automated validation.

## 2026-06-23 - Leap Root Playback Uses Contract Distance

### Symptom

Manual PIE reported player leap still rubberbanding near the end of the fall/landing, with the jump feeling too high and the descent/airborne horizontal motion feeling over-braked. Dodge had already been stabilized, so leap needed an isolated fix without changing dodge, walk/run, turn, or skill movement feel.

### Hypothesis

The dodge fix proved that owned locomotion must present root movement from the same movement action contract the server uses. Leap still differed: Unreal cached leap speed, timing, jump z and gravity, but did not receive/apply the authored horizontal distance into `ApplyAuthoritativeLeapContract`. Local leap prediction therefore remained velocity-driven while generic CharacterMovement and snapshot/ACK correction could perturb the root near the landing window.

### Cause

- The DB/server leap contract had `distance_cm`, but the Unreal contract application path only passed `base_speed_cm_s` into player leap prediction.
- `TickLocalLeapPrediction` drove horizontal leap movement by assigning horizontal velocity instead of replaying an absolute contract distance from the action start root.
- The recovered leap tuning was too high/floaty for the current gameplay target: `jump_z_velocity=620`, vertical peak `180`, and speed curve ending at `0.35`, which felt like strong late-air deceleration.
- The first low/fast retune attempt (`duration_ms=560`, `jump_z_velocity=520`, `gravity_scale=1.15`) was rejected after PIE feedback because it could end the authored action window before landing and produced an abrupt apex-to-ground drop.
- Live LeapFlow monitoring showed `local_elapsed` near the end of leap while `server_elapsed` was still close to action start (`client_lead` around 0.7-0.8s). The server was rebuilding action-motion locomotion with `ActionStartedTick = current tick` every snapshot instead of preserving the original action-start tick.
- After `StartedTick` was fixed, live LeapFlow showed timeline alignment was healthy (`client_lead` around -0.05s), but snapshots still published `server_pos.z` at ground height (`96.9`) while the client was airborne (`local_pos.z` around 190-215). This made the leap look like the server/root had already touched the ground while the visible cylinder kept falling.

### Change

- Unreal leap contract application now receives `HorizontalDistanceCm` from cached contracts and ACK bootstrap, matching dodge contract semantics.
- Local leap prediction now replays horizontal root position absolutely from `LocalLeapPredictionStartRootLocation` using the contract speed curve integral, while preserving Unreal vertical physics.
- During active leap root playback, horizontal CharacterMovement velocity is zeroed after the contract move to prevent double integration between ticks.
- DB canonical `jump_v1_authoritative_grounded_handoff` tuning was lowered while keeping the contract alive through the full landing window:
  - `duration_ms: 620 -> 960`
  - `active/airborne_ms: 560 -> 900`
  - `base_speed_cm_s: 452 -> 292`
  - `jump_z_velocity: 620 -> 480`
  - `gravity_scale: 1.0 -> 1.0`
  - vertical curve peak `180 -> 110`
  - horizontal curve end `0.35 -> 0.62`
- Server fixture contract/curves were mirrored to avoid DB/runtime fixture divergence.
- `db-api` was restarted before `game-server` so the canonical seed was actually reapplied; restarting only the game-server leaves the old DB contract active.
- Leap diagnostics are temporarily enabled by default in Unreal and server bridge, while dodge diagnostics are disabled by default for this leap pass.
- Server `actionMotionState` now stores `StartedTick` and `applyActionMotionLocomotionTiming` republishes that same start tick for every owned locomotion/skill-root snapshot. This keeps client action projection on the real action timeline instead of treating every snapshot as a fresh leap start.
- Movement action progress now resolves vertical displacement through the same authoritative movement contract path:
  - `vertical_motion_model=ballistic` uses `jump_z_velocity`, `gravity_scale`, and `gravity_z_cm_s2` for player jump/leap parity with Unreal CharacterMovement.
  - `vertical_motion_model=curve` keeps curve-authored vertical arcs for actions such as wolf lunge, avoiding a player-leap fix that changes creature leap feel.
- DB seed metadata now declares `vertical_motion_model` and `gravity_z_cm_s2`; player jump expected apex metadata was corrected to match the ballistic contract instead of the recovered stale value.

### Tests

- `PlainTestMapEditor Win64 Development -NoHotReload` build succeeded.
- `go build ./...` in `server-apeiron` succeeded.
- `go build ./...` in `db-apeiron` succeeded.
- `db-api` boot log confirmed `bootstrap\014_action_runtime_contract_seed.sql` reapplied before the game-server restart.
- After live monitoring, `go build ./...` in `server-apeiron` succeeded again with stable `StartedTick` publication.
- After vertical authority fix, `go build ./...` in `server-apeiron` and `db-apeiron` succeeded.

### Guardrail

Do not fix leap rubber by increasing leap deadzones or changing dodge. Leap horizontal movement is an owned root action during the contract window: server owns the final authority, but Unreal presentation must replay the same contract distance from the same start root and only use vertical CharacterMovement for jump physics.

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

## 2026-06-22 - Runtime Commands Reject Missing Movement Contracts

### Symptom

The North Star still classified runtime fallback paths as suspicious. In `gameapi`, dodge and
leap command execution still passed literal fallback distances into action motion resolution.
Even when normal recovered contracts were present, that left a live path where a missing DB/action
contract could be accepted and moved with invented data.

### Hypothesis

Fallback distances are not the direct cause of every rubberband, but they are a root-risk:
they allow command execution to proceed without the same contract manifest that Unreal prediction
and server reconciliation expect. That can hide DB/manifest holes and later present as correction,
missing contract prediction, or post-action handoff mismatch.

### Change

- `SubmitCommand` now validates dodge, leap, and skill movement contracts before executing.
- Missing contracts reject with `missing_movement_contract` or `missing_skill_contract`.
- Invalid movement contracts reject with `invalid_movement_contract`.
- `applyImpulse` no longer accepts literal fallback distances for dodge/leap.
- Added server guards:
  - `TestRuntimeRejectsDodgeAndLeapWhenMovementContractIsMissing`
  - `TestRuntimeRejectsSkillWhenRuntimeContractIsMissing`
  - `TestRuntimeShiftRunRepeatedShieldSkillsReturnForwardMove`

### Effect

Server runtime no longer masks missing movement contracts in the live command path. The remaining
`FallbackDistanceCM` usage in action-motion progress is derived from the already-started action's
stored total distance, not from an invented ability default.

### Guardrail

Do not reintroduce command-level movement distances such as "dodge = 260" or "jump = 280".
If an action needs a distance, it must come from `movement_action_contract` / skill movement
binding data and be present in the runtime contract registry.

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

## 2026-06-22 - Runtime Reconciliation Profile Made Authoritative End To End

### Symptom

The Unreal client still had a rich `FApeironMovementReconciliationProfile` initialized with
client fallback values, and several reconciliation paths used positive-or fallback literals.
That meant a missing DB/server field could look valid in the client and only show up later as
rubberbanding, correction tuning weirdness, or a mismatch between server and local movement.

### Change

- Unreal `FApeironMovementReconciliationProfile` now carries runtime validation metadata.
- Unreal snapshot parsing validates the raw `movement_reconciliation` JSON field list before
  treating the profile as authoritative.
- Unreal rejects incomplete/fallback reconciliation profiles in normal correction paths instead
  of silently applying default values.
- DB `runtime_movement_reconciliation_profile` seed/migration now owns the handoff/landing fields
  that were previously zero and therefore client-fallback-owned.
- Server `ValidateRequiredCoverage` now validates the full Unreal-facing rich movement profile.

### Effect

Missing or partially reconstructed reconciliation profile data should fail loudly in DB/server/client
tests or logs, not become a hidden movement tuning value in the client. The C++ defaults remain only
as inert initialization/editor safety, not as normal gameplay authority.

### Guardrail

Any new movement/reconciliation field must be added in one slice across DB migration, DB seed,
proto/repository mapping, server runtime validation, Unreal parsing, and Unreal profile validation.
Do not add new `PositiveOr(..., literal)` gameplay behavior unless the literal is provably a
non-runtime safety default and missing authoritative data is logged or rejected.

## 2026-06-23 - Core Contract Startup Gate, Player Cooldowns, And Dodge Contract Retention

### Symptom

PIE logs showed `TurnFlow submit_blocked_missing_contract` for early movement/turn input before
the `turn` movement action payload was cached. The same test pass also showed active Bulwark skills
could be spammed because player skill runtime published `CooldownEndMs` but the gameapi command gate
did not enforce a player cooldown map. Dodge could keep a post-stop carry based on stale local
fallback phase values after authoritative contract state had already been loaded.

### Change

- Unreal now refuses normal movement submission, dodge, and leap while core movement contracts
  (`turn`, `dodge`, `leap`) are not ready. This removes the startup window where the client could
  predict yaw/locomotion before the authoritative contract payload arrived.
- `gameapi` now tracks player active-skill cooldowns with `playerCooldownUntil`, mirroring the
  existing creature cooldown ownership but excluding basic attacks.
- Cast ACK metadata now includes `skill_cooldown_ms`, `cooldown_until_ms`, and
  `cooldown_remaining_ms` when a player active skill starts or is rejected by cooldown.
- Shield Bash and Shield Rush canonical cooldowns moved to DB/fixture contracts:
  `R = 26000ms`, `F = 32000ms`.
- Shield Rush movement distance/speed was reduced by 10% in DB and fixture contracts:
  `960cm/1148cm/s -> 864cm/1033.2cm/s`.
- Unreal dodge stop paths no longer reset the loaded dodge phase/curve back to local literal
  defaults after the authoritative contract has been cached.
- Added `TestActiveSkillCooldownBlocksRecastAfterRootMotionCompletes`; updated rubberband stress
  tests to explicitly expire cooldowns when the test purpose is reconciliation rather than cooldown.

### Effect

The curve-rubber startup path is gated by contract readiness instead of fallback yaw behavior.
R/F cooldown is now server-authoritative and visible to the client. Dodge stop keeps the loaded
contract as the live movement truth instead of reverting to default local phase numbers.

### Guardrail

Do not fix movement rubber by changing normal movement feel or lateral sprint caps. First verify
the relevant movement action contract is loaded before prediction, then check whether the server
and client publish the same action/phase/reconciliation fields. Active skill cooldowns belong to
the action runtime gate; basic attacks stay cooldown-free unless a future weapon contract explicitly
changes that rule.

## 2026-06-23 - Dodge Exit Handoff Stops Local Carry

### Symptom

Manual PIE showed player dodge sometimes becoming an infinite horizontal slide, followed by
rubberbanding/teleport back and blocked movement. The same pass showed the restored dodge distance
felt too short.

### Hypothesis

The server could be ending dodge correctly while the Unreal stop path still preserved residual local
velocity. If the client seeds grounded carry from its current velocity after an authoritative
zero-speed dodge handoff, it can keep moving after the server-owned dodge has ended.

### Cause

- Server owned-locomotion snapshots did not publish reliable phase elapsed/remaining values for
  dodge, so the client could lose the authoritative dodge timeline.
- Server did not publish an explicit dodge `exit_handoff` state with a zero exit speed when the
  owned dodge root completed.
- Unreal `StopLocalDodgePrediction` treated an authoritative dodge `exit_handoff` with zero exit
  speed as eligible for generic carry because local horizontal velocity was still nonzero. That
  seeded `GroundedCarryHandoff` with the current local speed and created the infinite slide.
- Unreal `SubmitMove` used a separate `sprint` movement contract hash while server normal movement
  is contract-owned by `move`; sprint is a movement parameter/profile, not a different movement
  action contract in the current DB.

### Change

- `gameapi` now publishes phase elapsed/remaining for owned locomotion through the shared movement
  contract timing path.
- `gameapi` publishes dodge completion as `phase=exit_handoff`,
  `movement_mode=grounded_handoff`, `landing_handoff_active=true`, and
  `landing_exit_speed=0`, then expires it to `complete`.
- `gameapi` clears owned-locomotion action lock/state when dodge completes.
- Unreal `StopLocalDodgePrediction` now treats authoritative dodge `exit_handoff` with no exit
  speed as a command to clear grounded carry and zero horizontal velocity, not as permission to
  reuse local residual speed.
- Unreal `SubmitMove` now submits the `move` contract hash for normal movement; sprint remains a
  movement parameter.
- Dodge distance tuning moved through the DB/fixture movement action contract:
  `260cm / 812.5cm/s -> 360cm / 1125cm/s`, preserving `320ms` duration and the full iframe
  contract.

### Tests

- `go test ./internal/movement ./internal/gameapi`
- `go test ./...` in `server-apeiron`
- `go test ./...` in `db-apeiron`
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- Runtime DB check confirmed `ProfileDataService.GetMovementActionContract(dodge_v1_full_iframe)`
  returns `distanceCm=360`, `baseSpeedCmS=1125`, `reconciliationContractId=dodge_reconciliation`,
  and `inputPolicy=blocked_during_owned_root`.

### Guardrail

Do not reintroduce generic grounded carry on dodge `exit_handoff` unless the server explicitly
publishes a positive `landing_exit_speed`. Dodge is an owned root action: client prediction may
mirror the contract, but the end of the action must be released by the authoritative handoff, not
by residual local velocity.

## 2026-06-23 - Dodge Local Root Prediction Uses Contract Sweep

### Symptom

After the infinite slide fix, manual PIE showed dodge no longer trembling or rubberbanding, but the
local cylinder appeared to stay still during the dodge. When movement resumed, the cylinder snapped
forward by roughly half a body length.

### Cause

The server was advancing the authoritative dodge position, but Unreal local dodge prediction was
velocity-only while normal movement input is intentionally suppressed during the owned dodge root.
That meant the client could acknowledge and later reconcile the server position without visibly
moving the local root during the dodge window.

### Change

- Unreal cached/ACK dodge contracts now pass `HorizontalDistanceCm` into
  `ApplyAuthoritativeDodgeContract`.
- Local dodge prediction stores contract distance, applied distance, and prediction velocity.
- `TickLocalDodgePrediction` now applies swept root displacement with
  `SafeMoveUpdatedComponent`, driven by the same normalized movement curve integration used by
  skill movement.
- The previous zero-speed authoritative `exit_handoff` remains in place, so the dodge still ends
  without residual slide.
- Focused Unreal validation now submits a stationary dodge and fails if the local root does not
  move during the dodge observation window.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- Focused automated Unreal validation passed:
  `focused=dodge=1 dodge_cm=431.9 stationary_basic=6 rf=4/4 interleaved_basic=16 slow_curve=4 root_suppressed=0`.

### Guardrail

Do not make owned dodge root rely only on local velocity while movement input is suppressed. The
client presentation must advance root position from the DB movement action contract, and the server
must remain the authority for the final handoff.

## 2026-06-23 - Dodge Only Validation And Stamina Contract

### Symptom

Manual PIE reported the previous dodge adjustment regressed into a visible go-forward/go-back feel.
The automated validation was also too broad: new dodge checks were being folded into the large
movement flow, making each iteration slow and harder to interpret. Manual PIE also showed dodge not
spending stamina.

### Hypothesis

The local dodge root sweep was correct, but ACK/snapshot reconciliation could still try to converge
the pawn root while the local dodge prediction owned presentation. In parallel, the server had DB
stamina values loaded in combat core profiles but the proto did not expose them to game-server
runtime, so player dodge could not spend authoritative stamina.

### Cause

- `CombatCoreProfile` exposed dodge iframe and posture/block fields, but not `max_stamina`,
  `stamina_regen_per_sec`, or `dodge_stamina_cost`.
- Game-server initialized player stamina from a literal default and did not spend profile stamina
  on `COMMAND_TYPE_DODGE`.
- Unreal had no isolated dodge-only automation suite; dodge coverage lived inside broad/focused
  flows.
- ACK/snapshot correction paths could still evaluate correction while `IsLocalDodgePredictionActive`
  was true, instead of treating dodge snapshots as authoritative feed for state/timing only.

### Change

- DB proto and gRPC mapper now expose combat core stamina fields.
- Game-server initializes player stamina from the combat core profile, spends
  `dodge_stamina_cost` before applying dodge, rejects insufficient stamina, regenerates stamina
  from `stamina_regen_per_sec`, and includes stamina metadata in command ACKs.
- Dev fixture combat core profiles now mirror the DB-backed stamina fields instead of silently
  defaulting to zero-cost dodge.
- Unreal now supports a dedicated `-ApeironRunDodgeMovementInputValidation` suite. It opens the
  game, submits four directional dodges, waits for stamina recovery, submits two more diagonal
  dodges, validates movement/stamina, then exits.
- ACK and generic snapshot correction now defer root correction while local dodge prediction is
  active unless the server explicitly requests a snap/rejection.

## 2026-06-23 - Dodge IFrame Bound To Owned Motion Contract

### Symptom

Manual PIE showed wolf lunge hits landing during player dodge. The client submitted dodge and
published `DodgeActive`, but damage events still arrived as `reason=hit` instead of `evaded`.
Some later snapshots also kept reconciling against `DodgeActive`, making the player feel trapped
or snapped back after being hit around dodge timing.

### Hypothesis

The combat pipeline was relying too much on transient `skillState` / `combatState` strings to
detect dodge iframe. Those strings can be cleared or overwritten by action/impact transitions while
the authoritative owned dodge root motion is still active.

### Change

- Runtime combat adapter now derives dodge defense from active `owned_locomotion` action motion
  whose movement contract action/ability is `dodge`.
- Creature perception uses the same iframe helper, so AI and damage resolution agree about whether
  the player is currently invulnerable.
- Damage events now expose `evaded`, `pipeline_reason`, `target_pipeline_state`, and
  `target_iframe` metadata for live log validation.
- Fatal player damage now immediately respawns the player with full health/stamina/posture and
  clears transient action locks/root motion, keeping live combat validation running without server
  restarts.

### Validation

- `go build ./...` in `server-apeiron` passed.
- Unit tests intentionally not used in this recovery pass; runtime PIE validation is the gate.

### Guardrail

Dodge iframe must be owned by the active movement action contract, not by a fragile display state.
Future changes must not make creature damage, AI perception, and client locomotion infer dodge
defense through separate state strings.

### Tests

- `go test ./internal/gameapi`
- `go test ./...` in `server-apeiron`
- `go test ./...` in `db-apeiron`
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- Dedicated Unreal dodge suite passed:
  `dodge_only=submitted=4 recovered_submitted=2 path=5746.0 max_distance=697.7 stamina_start=100.0 stamina_low=47.4 stamina_recovered=100.0 spent=true regen=true moved=true`.
- Dedicated dodge log contained no `RubberbandProbe`, `snapshot_apply`, or movement rejection lines.

### Guardrail

Do not add future dodge checks to the broad movement validation by default. Keep dodge, leap,
walk/run/turn, and skill-movement suites small and targeted so regressions point to one authority
path. During an active local dodge, snapshots may update authoritative action state, but root
correction must wait for explicit server snap/rejection or the post-dodge handoff.

## 2026-06-23 - Grounded Walk Run Strafe Turn Validation Suite

### Symptom

Manual PIE still reported rubberbanding around base movement: walk/run curves, lateral sprint,
diagonal sprint, backward movement, A/D reversals, and post-dodge grounded movement. The existing
automation mixed leap, dodge, M1, R, F, and grounded movement in one long flow, so failures were
hard to attribute.

### Hypothesis

Before changing movement again, grounded locomotion needed a small dedicated suite that only holds
walk/run/strafe/backward/diagonal/turn inputs and fails on actual authoritative movement damage:
server correction, command rejection, segment movement failure, or incoherent high-speed
`move_stop`.

### Change

- Unreal now supports `-ApeironRunGroundedMovementInputValidation`.
- The suite runs 23 held-input segments: W walk, W sprint, A/D strafe, A/D lateral sprint, W+A/W+D
  sprint, S backward walk, S backward run, smooth W curve, aggressive W sprint curves, same/opposite
  lateral sprint yaw, opposing diagonal sprint yaw, and A/D plus W+A/W+D reversals.
- The bridge records grounded validation correction/rejection/probe/max-error counters.
- `SubmitMovementIfNeeded` records `high_speed_move_stop` when a stop command is submitted while
  local horizontal velocity is still high.
- The PowerShell runner accepts `-GroundedInputPlayback` and scans the focused log for segment
  failures, correction events, rejection events, high-speed stops, and existing rubberband
  signatures.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- Dedicated Unreal grounded suite passed:
  `grounded=segments=23/23 failed=0 path=27546.1 corrections=0 rejections=0 probes=902 high_speed_stops=0 max_error=86.5`.

### Guardrail

`probes` are observations, not automatic failures. A grounded movement run fails on corrections,
rejections, failed segments, incoherent high-speed `move_stop`, or scanner-classified visible
rubberband signatures. Do not hide future failures by loosening correction/rejection detection; if
manual PIE still shows rubber while this suite passes, investigate interference from state that this
suite intentionally excludes: creature contact, action state persistence, post-skill handoff, or
runtime state surviving across sessions.

## 2026-06-23 - Reattach Clears Expired Owned Dodge Root

### Symptom

Dedicated dodge validation could open PIE but never submitted the first dodge. The log showed:
`authoritative_dodge=true`, `auth_phase=active`, `phase_remaining=0.000`, `submitted=0`. Manual PIE
matched this failure mode: dodge sometimes left the player unable to move, and closing/reopening
Unreal could keep the player stuck because the server still exposed the old action state.

### Hypothesis Matrix

- Client/server contract mismatch: unlikely for this symptom because the dodge contract payload was
  confirmed as `dodge_v1_full_iframe`.
- Tuning/deadzone problem: rejected; the player was blocked before movement or correction could
  happen.
- Duplicate position owner: possible secondary risk, but not the first failure because no dodge was
  submitted.
- Stale post-action handoff/action state: confirmed by the zero-remaining authoritative dodge phase.
- Creature/contact interference: excluded in validation because game-server ran with
  `MOVEMENT_VALIDATION=true` and creature runtime disabled.

### Cause

`AttachPlayer` reset command replay state but did not clear an expired owned root motion for the
player. A stale dodge action could therefore survive a new Unreal attach. On the client,
`IsAuthoritativeDodgeStateActive` treated `startup/active/recovery` as active even when the
effective remaining phase time was already zero.

### Change

- Server `AttachPlayer` now clears expired player-owned root motion before resetting replay state.
  The cleanup is limited to expired `owned_locomotion`; it does not clear active actions or creature
  state.
- The cleanup also releases the action lock, clears exit handoff, zeros velocity, and marks matching
  dodge locomotion complete.
- Unreal `IsAuthoritativeDodgeStateActive` now uses effective phase remaining time, so a transient
  authoritative dodge phase with zero remaining time cannot block movement/dodge submission forever.
- PowerShell focused validation scanner now accepts the current focused summary shape with
  `focused=dodge=1 ... stationary_basic=6 ...`.

### Tests

- `go test ./internal/gameapi -run "TestRubberbandGuardAttachClearsExpiredOwnedDodgeRoot|TestRubberbandGuardDodgeExitHandoffStopsLocalCarryAndReleasesLock|TestRubberbandGuardDodgeSnapshotPublishesAuthoritativeTimeline"`
- `go test ./...` in `server-apeiron`
- `go test ./...` in `db-apeiron`
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- Dedicated Unreal dodge suite passed:
  `Apeiron movement validation log scan passed`.
- Dedicated Unreal grounded suite passed:
  `Apeiron movement validation log scan passed`.
- Focused Unreal M1/R/F suite passed:
  `focused=dodge=1 dodge_cm=673.7 stationary_basic=6 rf=4/4 interleaved_basic=16 slow_curve=4 slow_curve_cm=10853.1 distance=5063.8 root_suppressed=0`.

### Guardrail

Do not let `AttachPlayer` revive expired owned-root state. Reattach may preserve a genuinely active
server action, but an expired action plus expired handoff must become complete before the next local
client run. Client-side authoritative action checks must use effective remaining time, not only the
phase label, because old snapshots can otherwise act like permanent locks.

## 2026-06-23 - Dodge Runtime Trace Toggle

### Symptom

Manual PIE still reports dodge sometimes losing player control after M1/basic attacks and sometimes
after being hit during dodge. The visible failure is not only rubberbanding: the client can appear to
complete a dodge, then later movement/skill input snaps the player back or leaves movement blocked.

### Hypothesis Matrix

- Client prediction overrun: possible, because the local dodge can continue or carry after the server
  has already ended/grounded the action.
- Server stale action state: possible, because prior fixes proved expired owned-root state can
  survive attach/replay boundaries.
- Damage pipeline breaking iframe/action root: possible, because the user reproduced stuck dodge
  after hits during dodge and reported dodge iframe not always applying.
- M1/basic action handoff contaminating dodge: possible, because the user reproduced M1 then dodge
  losing control.
- Tuning/deadzone problem: rejected for this round. The goal is to expose authority mismatch, not
  mask it.

### Change

- Added server toggle `APEIRON_DODGE_DEBUG`.
- Added server dodge state trace around submit validation, stamina rejection, owned-locomotion begin,
  owned-locomotion completion, and damage impact resolution against the player.
- Added client toggle `-ApeironDodgeDebug` / `bLogDodgeFlow`.
- Added client dodge trace around input rejection, submit before/after prediction, local prediction
  start/tick/stop, authoritative dodge snapshots, grounded snapshots after dodge, movement suppression,
  move_stop, and first normal movement after dodge.

### Tests

- `go build ./...` in `server-apeiron` passed.
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.

### Guardrail

This is diagnostic-only observability plus the existing owned-locomotion isolation path. Do not use
the resulting logs to justify deadzones, hidden smoothing, disabled lateral movement, or client-only
rollback. The next runtime fix must identify which authority still owns dodge after it should have
released, or why the server rejects/damages during an iframe window.

## 2026-06-23 - Dodge Zero-Speed Exit Handoff Releases Client Ownership

### Symptom

Manual PIE reported dodge improving but still showing rubber/snap at the end. The player could
complete the visible dodge, then the next movement made the capsule appear offset or corrected.

### Hypothesis

The server-side dodge contract ends owned locomotion with an explicit grounded exit handoff and
`LandingExitSpeed = 0`. That means the server is not asking the client to continue moving after
the dodge root finishes. Unreal still had paths that could treat that snapshot as an active
authoritative dodge state or seed local grounded carry from the local curve endpoint.

### Change

- Unreal now treats dodge `exit_handoff` with zero exit speed as a release marker, not active
  movement ownership.
- `StopLocalDodgePrediction` no longer invents a local endpoint carry speed when the authoritative
  dodge exit speed is zero.
- Rich locomotion snapshots and legacy dodge snapshots now share the same zero-speed exit behavior.
- The change is isolated to dodge exit ownership. It does not alter dodge distance, iframe, stamina,
  walk/run, leap, turn, or skill movement tuning.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.

### Guardrail

Do not fix dodge end rubber by adding deadzone or changing dodge distance. If rubber remains, inspect
whether the server is still publishing nonzero exit speed or whether a later grounded snapshot is
anchored behind the client endpoint.

## 2026-06-23 - Dodge Root-Owned Absolute Contract Playback

### Symptom

Manual and focused validation showed dodge no longer sliding infinitely, but the capsule could still
visibly snap backward during active dodge. The log signature was a `client_move_suppressed` location
moving opposite the dodge direction between local prediction ticks, while the server completed the
same dodge at the correct 360cm endpoint.

### Hypothesis

Two presentation problems were active on the client:

- CharacterMovement still had enough horizontal state to move the capsule between root-owned dodge
  ticks after `SafeMoveUpdatedComponent`.
- The local dodge integrator was incremental. If any ACK/snapshot/collision path perturbed the root
  mid-dodge, the next tick continued from the wrong current location instead of re-owning the root
  from the action contract's start point and timeline fraction.

This was not a dodge distance/deadzone tuning issue. The server-owned locomotion state and iframe
window remained correct in the server trace.

### Change

- During local dodge prediction, Unreal now treats the dodge as root-owned action movement:
  `SafeMoveUpdatedComponent` applies the contract movement and the CharacterMovement horizontal
  velocity is zeroed afterward, avoiding double integration between action ticks.
- Dodge playback is now absolute against `LocalDodgePredictionStartRootLocation` and the contract
  speed curve. Each tick computes the target root position for the current timeline fraction and
  moves toward that target, instead of accumulating a delta from whatever root position another
  system left behind.
- The fix is generic for player dodge and does not alter the DB contract, dodge distance, stamina
  cost, iframe timing, walk/run, leap, turn, or skill movement tuning.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- Dedicated Unreal dodge suite passed:
  `Apeiron movement validation log scan passed`.
- Extra log audit projected each dodge frame onto the dodge direction and found:
  `dodge_monotonic_check passed drops=0`.

### Guardrail

Do not reintroduce horizontal CharacterMovement velocity as the driver for active dodge. Dodge root
motion is owned by the movement action contract while active; snapshots may feed phase/direction, but
they must not make the client accumulate action motion from a stale or externally corrected root.

## 2026-06-23 - Leap Contract-Owned Vertical Playback

### Symptom

Manual PIE showed the player leap looking like it touched or completed early on the server while the
visible capsule kept falling. Near the end of the leap, the fall felt like a sudden drop instead of a
natural landing.

### Evidence

Leap debug logs showed two separate problems:

- After the server vertical model was restored, active leap snapshots started publishing airborne
  `server_pos.z`/`server_root.z`, so action start tick and active server Z were no longer the main
  issue.
- The client still let Unreal `CharacterMovement` own vertical physics while the server used the
  contract. At `duration=960ms`, server locomotion completed on the ground while the local pawn was
  still falling for additional frames. Log signature: `LeapFlow post_duration_fall` with local Z
  still far above ground.

### Change

- Server/DB leap contract timing now matches the declared ballistic model:
  `JumpZ=480`, `GravityZ=980`, `duration/airborne=980ms`, `expected_apex=490ms`.
- Unreal local leap prediction now applies vertical root from the same contract each tick. If the
  contract has `JumpZVelocity`, the client evaluates the same ballistic equation; otherwise it uses
  the vertical curve samples.
- The post-duration path no longer becomes a second vertical physics owner; it re-applies contract
  vertical before waiting for the final grounded handoff.

### Tests

- `go build ./...` passed in `server-apeiron`.
- `go build ./...` passed in `db-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- `db-api` restarted, applied bootstrap seeds, and exposed gRPC on `50051`.
- `game-server` restarted with `DB_APEIRON_ENDPOINT=127.0.0.1:50051` and loaded runtime contracts
  from DB.

### Guardrail

Do not let generic Unreal falling physics become the owner of leap Z while the server is resolving
leap from a movement action contract. Leap root has one owner: the shared contract. Landing/handoff
may release to grounded movement only after the contract-owned vertical path reaches the ground and
the client/server handoff agrees.

## 2026-06-23 - Leap Debug Logging Isolated From Global Movement Trace

### Symptom

Manual PIE with leap debugging enabled produced continuous logs while the player was standing still:
`SnapshotTimeline player_apply`, `ApeironMoveTrace unreal_snapshot`, and `MoveTraceCycle` with
`action=move_stop` / `mode=GroundedMove`. Those logs were not leap logs and made live diagnosis noisy.

### Cause

The Unreal bridge reused `bLogLeapReconciliationFlow` as a generic movement/snapshot trace gate.
Because leap debug was enabled by default in recovered C++ fields, normal idle snapshots were printed
even when the current focus was only player leap.

### Change

- Client `LeapFlow` logging is now off by default and enabled by `-ApeironLeapDebug`.
- Bridge leap reconciliation logging is now off by default and enabled by `-ApeironLeapDebug`.
- When `-ApeironLeapDebug` is active, the bridge forces leap-only movement debug filtering and keeps
  global rubberband/player snapshot probes off.
- `SnapshotTimeline` and `MoveTraceCycle` no longer print grounded `move_stop` spam under leap-only
  debug.
- Server `APEIRON_LEAP_DEBUG=true` now logs per-tick owned leap progress (`projected_z`,
  `velocity_z`, elapsed/duration, completion) without enabling global movement logs.

### Tests

- `go build ./...` passed in `server-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.
- `game-server` restarted through `scripts/start_game_server_with_db.ps1 -Build -Restart -LeapDebug`
  and loaded DB runtime contracts.

### Guardrail

Each movement investigation must have a narrow debug flag. Leap debug should emit only leap/action
handoff evidence; dodge debug should emit only dodge evidence; global movement/rubberband traces are
for broad scanner passes only.

## 2026-06-23 - Leap Vertical Root Ownership Isolated From Creature Skill Root

### Symptom

Manual PIE after leap vertical restoration showed two regressions:

- Player leap had less landing rubber, but after touching down the character moved slowly for a
  short window before normal control returned.
- Wolf lunge made the creature visually/physically climb higher on repeated lunges.

### Hypothesis

The restored vertical action model was too broadly attached to action-motion progress. Player
`owned_locomotion` leap needs contract-owned vertical root. Creature `skill_root` lunge needs
authoritative planar root plus temporary visual arc; otherwise its server root can accumulate Z and
the client can keep rendering the creature above the combat plane.

The slow player post-landing feel came from treating any recent leap exit as a grounded transition
that can override sprint/ground speed even when no explicit grounded handoff is active.

### Change

- `internal/movement.ResolveActionMotionProgress` now applies vertical root only when the caller
  explicitly sets `UseVerticalRoot`.
- Server player `owned_locomotion` sets `UseVerticalRoot` from the movement action contract when the
  action type is `leap` and the contract declares vertical motion.
- Creature/player skill root, impact control, and grounded skill movement remain planar on the
  authoritative server root unless a future contract path explicitly opts into vertical root.
- Unreal no longer suppresses sprint or runs grounded-move velocity sync merely because a leap ended
  recently. Leap after touchdown releases to normal grounded input unless a real grounded carry
  handoff is active.

### Tests

- `go test ./internal/movement ./internal/gameapi` passed.
- `go build ./...` passed in `server-apeiron`.
- `go build ./...` passed in `db-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.

### Guardrail

Do not attach player jump/leap vertical ownership to generic skill root motion. Creature lunge can
look airborne through visual arc and temporal hitbox contracts, but its authoritative collision root
must stay planar until a dedicated creature-airborne-root contract exists and is consumed by both
server and client.

## 2026-06-23 - Leap Landing Handoff Cannot Outlive Active Leap Motion

### Symptom

Manual PIE after the vertical-root fix showed the leap itself landing cleanly, but grounded movement
immediately after touchdown felt like slow motion until it recovered.

### Evidence

Server leap debug showed the player `owned_locomotion` leap completing on the ground with zero
velocity. After that completion, Unreal still submitted grounded `move` / `move_stop` commands with
`handoff=leap` and the old leap sequence. The server accepted those late handoffs even when
`player.actionMotion == nil`, which let stale landing metadata overwrite the freshly grounded
movement state.

### Cause

Leap landing handoff was treated as sticky metadata instead of a one-window action transition. Once
the authoritative leap motion completed, the client-side pending handoff became stale, but both the
client and server still allowed it to ride on normal grounded movement.

### Change

- Server `applyMoveHandoffLocked` now accepts leap landing handoff only while a matching active
  `owned_locomotion` leap motion exists.
- Late, mismatched, or non-owned leap handoffs are ignored and logged through leap debug instead of
  mutating player position/velocity.
- Unreal now considers a pending leap landing handoff current only while local leap prediction or an
  authoritative leap state is still active.
- Grounded `move` and `move_stop` paths clear stale pending leap handoff before submitting normal
  movement, preventing `handoff=leap` from contaminating post-landing walk/run.

### Tests

- `go test ./internal/gameapi ./internal/movement` passed.
- `go build ./...` passed in `server-apeiron`.
- `go build ./...` passed in `db-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build succeeded with `-NoHotReload`.

### Guardrail

Landing handoff is an action transition, not a grounded movement mode. It must be consumed while the
matching action is active or discarded. Never let old leap handoff metadata travel on later
walk/run/turn commands.

## 2026-06-23 - Player Leap Ground Root Cannot Preserve Contaminated Airborne Z

### Symptom

Manual PIE still showed player leap kicking before touchdown and sometimes entering a repeated
kick/slowdown loop. Server leap debug showed owned player leap actions starting and completing at
`Z=137.9` instead of the PlainTestMap player actor-root plane (`Z=98.0`). After one contaminated
completion, later leaps reused that elevated Z as their new start root.

### Evidence

- `owned_locomotion_progress` for player `jump` completed with `projected.z=137.9`.
- The next `owned_locomotion_begin_before_clear` for `jump` also started at `position.z=137.9`.
- Unreal snapshots during local falling showed `locomotion_action=defense`,
  `reconciliation_mode=grounded_move_reconciliation`, and server Z still high, proving that defense
  could overwrite the leap locomotion publication while leap/root motion was still active.

### Cause

The movement action contract already declared `ground_z_policy=server_position_is_actor_root`, but
the runtime only preserved the current entity Z when starting/completing owned leap or accepting a
landing handoff. A stale airborne/root Z could therefore become the new grounded authority. Defense
commands also published grounded defense locomotion during active owned locomotion, stealing the
snapshot action from leap/dodge.

### Change

- Player actor-root ground height is now a named runtime constant instead of a spawn-only literal.
- Player grounded move, move_stop, turn, defense, leap start, leap completion, and leap landing
  handoff normalize to the contract root plane.
- Leap landing handoff still accepts the client XY handoff, but no longer trusts client Z for the
  authoritative grounded root.
- Defense/parry/block state can update combat state during owned locomotion, but it no longer
  overwrites locomotion while leap/dodge owns root motion.
- Server leap debug now logs only player jump/leap, not creature lunge.

### Tests

- `gofmt` applied to `internal/gameapi/runtime.go` and `internal/gameapi/leap_debug.go`.
- `go build ./...` passed in `server-apeiron`.

### Guardrail

For player leap, the authoritative root is contract-owned. Never use the current entity Z as the
next grounded root when the action contract declares server actor-root grounding. Combat state
changes may coexist with owned locomotion, but they must not become a competing locomotion publisher.

## 2026-06-23 - Unreal Leap Must Finalize On Contract Ground Contact Even If Landed() Does Not Fire

### Symptom

After the server-side root fix, PIE still showed the player touching the floor and then sliding in
slow motion. Unreal logs showed repeated `Apeiron LeapFlow post_duration_fall_finished` while the
local actor root was already at the ground plane (`Z ~= 98.0`). The authoritative snapshot had
already moved on to the next grounded/action state, but the local character movement component still
reported `falling=true`.

### Evidence

- Server completed player `jump` at `Z=98.0`.
- Unreal local logs showed `local_falling=true` with location `Z=97.9/98.0`.
- Snapshot action could already be `grounded_skill/recovery` while local leap prediction was still
  ticking `post_duration_fall`.

### Cause

The client trusted Unreal `Landed()`/`!IsFalling()` as the only clean exit for local leap prediction.
When contract vertical motion had already returned to the ground root but `Landed()` did not fire,
local leap stayed active, kept suppressing/subverting normal movement, and carried stale horizontal
velocity.

### Change

- Added a contract/floor finalizer for local player leap in Unreal.
- If leap contract duration is done, vertical contract sample is grounded, root is at the leap start
  ground plane, and velocity is not rising, the client forces a grounded leap completion.
- If the server terminal leap snapshot arrives while the client is still incorrectly falling at the
  floor, the client completes local leap instead of waiting indefinitely for `Landed()`.
- Leap gravity now applies only while authoritative leap state is active, not for terminal/complete
  leap snapshots.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.

### Guardrail

Leap lifecycle is contract-owned. `Landed()` is a useful signal, but not the sole authority. When
the shared leap contract says the root has reached ground and the server has completed/replaced the
leap state, the client must finalize local leap immediately instead of remaining in post-duration
fall.

## 2026-06-23 - Grounded Movement Debug Isolation And Backpedal Sprint Profile Guard

### Symptom

Manual PIE moved from leap-specific failure to base movement failure: walk/run/strafe/curves could
still rubberband, especially with held shift, lateral/backward movement, and aggressive camera yaw.
The previous diagnostics were still coupled to `LeapFlow`, making grounded movement logs noisy and
easy to misread.

### Hypothesis

Grounded movement needs its own diagnostic lane before additional authority changes. The client and
server must compare the same evidence: submitted direction, facing yaw used for speed cap, sprint
flag, profile multipliers, expected speed, server-resolved speed, and turn submissions. Leap and
dodge logs should stay off unless those actions are the current target.

### Change

- Unreal now has `bLogGroundedMoveFlow` / `-ApeironGroundedMoveDebug`.
- Leap debug is no longer enabled by default in Unreal; it only turns on with `-ApeironLeapDebug`.
- Move/turn submit logs for grounded movement use the grounded debug lane and include facing dot,
  current/target yaw, expected client speed, actual local horizontal speed, sprint, and handoff
  state.
- Server now has `APEIRON_GROUNDED_MOVE_DEBUG=1` and logs move/stop/turn with resolved speed,
  distance, yaw, sprint flag, and runtime movement profile multipliers.
- Unreal default backpedal sprint multiplier now matches the intended profile fallback (`0.50`);
  the DB profile remains authoritative and already seeds `backpedal_sprint_speed_multiplier=0.50`.

### Tests

- `go build ./...` passed in `server-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.
- `game-server` restarted with grounded movement debug enabled and leap/dodge debug env disabled.

### Guardrail

Grounded movement debugging must not use leap/dodge flags as the primary log switch. Normal movement
authority is `move` plus profile parameters; sprint, strafe, and backpedal are not separate movement
action contracts in the current runtime.

## 2026-06-23 - Grounded Replay Mirrors Server Command Kinematics

### Symptom

With leap fixed, manual PIE still showed rubberbanding during grounded walk/run/strafe and camera
curves. Server logs showed accepted `move` commands with coherent profile speeds (`strafe=0.75`,
`backpedal=0.50`) and no `action_locked` rejection, while Unreal `PlayerRecon` showed
`pending_grounded_replay_deferred` / `pending_grounded_replay_converging` during normal
`GroundedMove`.

### Evidence

Server `applyMove` resolves each accepted command through `movement.ResolveGroundedMove` as a
single command-step: velocity becomes `direction * resolved_speed`, and projected position advances
by `speed / tick_rate`. The Unreal grounded replay path was instead replaying pending inputs with
wall-clock/submit deltas and `VInterpConstantTo` acceleration/braking. During aggressive yaw or
lateral sprint, this made the correction target diverge from the server even though the server had
accepted the commands.

### Cause

Grounded reconciliation replay had a second kinematic model. Local CharacterMovement can still use
acceleration for presentation, but the authoritative correction target replay must match the server
resolver's command projection exactly.

### Change

- `TryReplayPendingGroundedMovement` now replays each pending grounded command using the same
  command-step duration used by the movement submit/profile contract.
- For move inputs, replay velocity is set directly to `direction * ResolveGroundedMoveSpeedForDirection(...)`.
- For stop inputs, replay velocity is zeroed immediately, matching server `move_stop`.
- No deadzone, smoothing, disabled strafe, or skill-specific exception was added.

### Tests

- `go build ./...` passed in `server-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.

### Guardrail

Do not put acceleration/braking simulation inside grounded reconciliation replay unless the server
resolver also owns the same acceleration model. Presentation smoothing belongs to Unreal movement;
replay target calculation must mirror server command kinematics.

## 2026-06-23 - Grounded Move Must Not Root-Converge While Correction Is Deferred

### Symptom

Manual PIE still showed rubberbanding in walk/run/strafe/turn after grounded replay was changed to
match server command kinematics. The prior replay change did not improve the visible issue.

### Evidence

Server logs showed coherent accepted `move` commands and no `action_locked` rejection. Unreal logs
showed `GroundedMove` decisions such as `pending_grounded_replay_deferred`, followed by
`pending_grounded_replay_converging`. The route had `should_apply=false`, but the capsule position
changed toward `correction_target` anyway through `MoveLocalPlayerToServerGroundLocation`.

### Cause

The normal grounded movement path had a hidden correction lane: after deciding to defer correction
because pending input/replay explained the snapshot age, it still performed root convergence when
`CurrentError > ModeDeadZone`. That made ordinary walk/run/strafe visually rubberband even when the
server accepted commands and the snapshot was merely behind active local input.

### Change

- `GroundedMove` is excluded from deferred root convergence.
- Deferred root convergence remains available for non-grounded/action transition modes where a
  bounded handoff may still need settling.
- Normal movement now has only the clean authority choices: keep predicting with pending input, or
  apply explicit correction when the snapshot is not explainable by pending input/profile windows.

### Tests

- `go build ./...` passed in `server-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.

### Guardrail

Never move the player capsule in `GroundedMove` from a path whose decision says correction is
deferred. If normal movement needs correction, it must be explicit and logged as an apply/smooth/snap
decision, not hidden behind a `*_converging` label.

## 2026-06-23 - Sprint Stamina Drain And Post Action Grounded Handoff Micro Rubber

### Symptom

After the `GroundedMove` root-convergence fix, manual PIE confirmed walk/run/curves were stable.
Remaining issues were smaller and scoped: sprint did not spend stamina, zero stamina had no exhausted
regen penalty, leap landing while holding movement had a tiny pullback, and dodge exit could tug
diagonally when the player immediately resumed walk/run.

### Cause

Sprint was not wired into `CombatCoreProfile` stamina spending. The post-action handoff paths could
also preserve a stale leap landing transform or dodge exit direction even when the player already had
active grounded input, causing a minor mismatch between the local next move and the handoff payload.

### Change

- Added DB/proto/gRPC combat-core fields:
  `sprint_stamina_cost_per_sec` and `stamina_zero_regen_multiplier`.
- Server `applyMove` now resolves an `effectiveSprint` after draining sprint stamina per tick.
- Zero stamina sets `staminaSpendLockedUntilFull`; regen uses the DB multiplier until full stamina
  clears the lock.
- Leap landing handoff sends the current local grounded position/velocity when movement input is
  already held at landing.
- Dodge terminal handoff now prefers active grounded input direction instead of carrying stale dodge
  exit direction against the player's current input.
- `GroundedMove` correction/root-convergence logic was not changed.

### Tests

- `go build ./...` passed in `db-apeiron`.
- `go build ./...` passed in `server-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.
- DB bootstrap reset/migrations/seeds completed and applied `bootstrap\\003_combat_core_profile_seed.sql`.
- Game server restarted and loaded runtime contracts successfully.

### Guardrail

Do not fix leap/dodge exit micro-rubber by changing baseline walk/run reconciliation. Post-action
handoff must adapt to active input at the boundary, while normal grounded movement remains governed by
the stable `GroundedMove` prediction path.

## 2026-06-23 - Shift Sprint Suppression During Action Exit

### Symptom

Manual PIE showed that holding `Shift` through dodge/leap exit made the remaining end-of-action
rubber more visible. Sprint stamina also appeared not to drain during run.

### Evidence

Client routing already suppressed sprint while dodge/leap were active and during recent dodge exit,
but recent leap exit was not part of `ShouldSuppressSprintForActionHandoff`. The DB player sprint
drain had also been seeded below stamina regen, which made sprint look free even though the server
path was wired.

### Cause

`Shift` could promote the first grounded submits after leap into sprint before the landing handoff
window had fully settled. For stamina, `sprint_stamina_cost_per_sec` must be higher than passive
regen if sprint should visibly reduce stamina while held.

### Change

- Reverted the prior current-position/current-velocity handoff experiment after manual validation
  showed no improvement and possible regression.
- `ShouldSuppressSprintForActionHandoff` now also suppresses sprint during recent leap exit, using
  the DB-loaded `LeapLandingCorrectionGraceSeconds` / `LeapGroundedCarryHandoffSeconds` profile
  windows.
- Player `sprint_stamina_cost_per_sec` is seeded at `24.0`, above `14.0` stamina regen, so sprint has
  visible net drain while still being contract/profile driven.
- Stable `GroundedMove` root-convergence exclusion was not changed.

### Tests

- `go build ./...` passed in `db-apeiron`.
- `go build ./...` passed in `server-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.
- DB and game server restarted with dodge/leap/grounded debug logs disabled by default.

### Guardrail

Do not let held sprint input override action-exit handoff windows. If action exit needs a longer or
shorter settlement interval, tune the DB reconciliation profile field; do not add hidden C++ timing
fallbacks or alter the normal grounded movement reconciler.

## 2026-06-24 - Dodge Direction Latched Through Exit Handoff

### Symptom

Manual PIE improved after sprint suppression, but dodge still showed a small rubber/pull at the end,
especially when movement input direction and camera/facing direction differed. The visible feel was:
dodge starts in the intended input direction, then the end of the action subtly pulls toward another
direction before normal movement resumes.

### Hypothesis

For a Souls-like dodge, the physical dodge vector must be latched at command start. Camera/facing can
turn during the dodge, but it must not recalculate the physical root direction in the final
handoff. Server already stores the dodge command vector in `actionMotion.Direction`; the suspicious
part was client handoff logic overwriting or prioritizing exit direction during/after prediction.

### Cause

Client dodge exit paths could switch from `LocalDodgePredictionDirection` to
`AuthoritativeLandingExitDirection` during authoritative `exit_handoff` or prediction stop. That
allowed the final frames to use a direction different from the command-start dodge vector.

### Change

- `TickLocalDodgePrediction` now keeps the latched dodge direction through `exit_handoff` instead
  of switching to `AuthoritativeLandingExitDirection`.
- `StopLocalDodgePrediction` now prioritizes the latched local dodge direction, then authoritative
  entry direction, then exit direction as last resort.
- Snapshot reconciliation no longer overwrites an active local dodge direction with landing/exit
  direction when a local prediction direction already exists.
- Server code was inspected and already publishes dodge exit direction from `motion.Direction`; no
  server movement change was needed.

### Tests

- `go build ./...` passed in `server-apeiron`.
- `go build ./...` passed in `db-apeiron`.
- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.

### Guardrail

Dodge direction is command-start authority. Do not use current camera yaw, actor facing, landing
exit direction, or current velocity to replace an active dodge direction. Those may influence what
happens after handoff, but not the dodge root itself.

## 2026-06-24 - Dodge Exit Grounded Carry Must Yield To Held Input

### Symptom

Manual PIE still showed a small pull after dodge even after the physical dodge direction stayed
latched during the action.

### Evidence

`ApeironDodgeDebug` showed the server completing dodge with zero velocity and the client predicting
the dodge in the intended input vector. The first forced grounded submit after dodge, however, was
sent with `carry_handoff=true` and a `submitted_dir` different from `raw_input_dir`. The next move
submit corrected to the held input direction.

### Cause

`ApplyAuthoritativeGroundedMoveSnapshotState` had a separate dodge-stop path that seeded grounded
carry handoff from authoritative grounded direction even when the player already had active grounded
input. That stale carry direction won the first post-dodge move and caused the visible pull.

### Change

- Dodge authoritative-grounded stop now detects active grounded input at the boundary.
- If input is held, it clears grounded carry handoff instead of seeding stale carry direction.
- The first forced grounded submit after dodge is allowed to use current input direction.
- The shared forced grounded submit path now treats recent dodge separately from leap. Post-dodge
  submits use current input and cannot fall through to yaw/current-velocity carry seeding.
- Baseline `GroundedMove`, walk/run reconciliation, leap and skill movement were not changed.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.

### Guardrail

Post-dodge carry is valid only when no current grounded input is competing with it. Held input after
dodge must be the owner of the next grounded move direction; carry handoff must not override it.

## 2026-06-24 - Explicit Dodge/Leap Exit Transition Ownership

### Symptom

Manual PIE improved after post-dodge carry cleanup, but a smaller end-of-dodge pull remained. The
likely remaining artifact was the terminal `DodgeExitHandoff` snapshot applying a near-threshold
position correction while the player was already transitioning back to grounded movement.

### Evidence

Previous debug logs showed `DodgeExitHandoff decision=apply` with current error only slightly above
the mode dead zone. The client and server agreed on dodge direction, but the final terminal snapshot
could still pull the capsule to the server endpoint immediately.

### Cause

The client had distinct logical modes (`DodgeExitHandoff`, `LeapLandingHandoff`), but forced grounded
submit state still used the legacy `bForcePostLeapGroundedSubmit` flag for both dodge and leap. The
bridge also treated `DodgeExitHandoff` as an apply candidate instead of an explicit short settling
phase.

### Change

- Added explicit forced grounded submit source tracking: `Dodge`, `Leap`, or `None`.
- Post-dodge forced submit now remains semantically post-dodge; post-leap remains post-leap.
- `DodgeExitHandoff` snapshots inside the DB/profile transition window now defer and converge only
  to the dead-zone boundary instead of applying the whole terminal correction.
- No baseline grounded walk/run/turn code or speed profile was changed.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.

### Guardrail

Dodge and leap may both transition back to grounded movement, but they must not share an anonymous
handoff state. Dodge exit owns dodge deceleration/settling; leap landing owns leap landing/handoff.

## 2026-06-24 - Delayed Dodge Terminal Snap Must Settle, Not Apply

### Symptom

Manual PIE dodge spam was mostly smooth, but monitoring still caught a rare visible pull. The bad
trace was `1000000:1553:62051`: `DodgeExitHandoff decision=apply` with `pending_after=0`,
`current_error=336.6`, and `should_apply=true`.

### Evidence

The client had already submitted/received an accepted grounded move for sequence `1553`, but the next
snapshot was still a delayed `DodgeExitHandoff action=dodge`. Because no pending movement remained
for replay, the bridge treated that delayed terminal dodge snapshot as a new authoritative root and
pulled the capsule toward the server endpoint.

### Cause

`DodgeExitHandoff` settling only covered recent bridge transitions inside a narrow near-dead-zone
window. A delayed terminal dodge snapshot with no replay could therefore fall through to generic
apply even though it was still part of the dodge exit transition.

### Change

- `DodgeExitHandoff` now recognizes terminal dodge phases (`exit_handoff`, `complete`, `recovery`) as
  dodge-exit settling candidates, even when there is no pending input replay left.
- The settle window is based on the DB/profile dodge exit max plus correction step budget instead of
  the previous narrow near-dead-zone cap.
- Deferred dodge-exit convergence is capped by the profile correction step per snapshot, instead of
  jumping all the way to the dead-zone boundary in one frame.
- Baseline walk/run/turn, leap, dodge command direction, and skill movement were not changed.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.

### Guardrail

Delayed terminal dodge snapshots are not a new movement authority. They may settle through the
dodge-exit reconciliation profile, but they must not snap the player root when the action has already
transitioned back to grounded movement.

## 2026-06-24 - DodgeActive Recovery Is A Terminal Transition

### Symptom

Manual dodge validation looked clean for no-input dodge, but one remaining visible micro-pull was
captured during an input/sprint dodge. The bad signature was `DodgeActive decision=apply` while the
server locomotion phase was already `recovery`, with `current_error=132.1`, `dead_zone=90.0`, and
`should_apply=true`.

### Evidence

The latest no-input dodge traces stayed as `stale_within_deadzone` or transition settling with
`should_apply=false`. The remaining correction was not the no-input path; it was a terminal dodge
snapshot still classified as `DodgeActive` instead of `DodgeExitHandoff`.

### Cause

The bridge already treated delayed `DodgeExitHandoff` terminal snapshots as transition settling, but
`DodgeActive` snapshots in `recovery/complete` could still fall through to generic apply. That made a
late terminal snapshot behave like new position authority.

### Change

- `DodgeActive` snapshots with locomotion `action=dodge` and `phase=recovery/complete` now use the
  same DB/profile transition settle window as terminal dodge-exit snapshots.
- The decision reason is logged as `dodge_terminal_settling_deferred` for this path.
- No baseline walk/run/turn, no-input dodge direction, dodge distance, or server action contract was
  changed.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.
- Manual runtime validation still needed: dodge traces should no longer show
  `DodgeActive decision=apply` in terminal phases.

### Guardrail

Active dodge can still correct when it is truly outside the action envelope. Terminal dodge phases
must settle through the dodge reconciliation profile and must not become generic snapshot snaps.

## 2026-06-24 - Dodge Handoff Must Not Re-enter Sprint Inside Exit Window

### Symptom

No-input dodge looked clean, but dodge while walking/running still had a tiny end snap. The player
reported the endpoint felt slightly offset from the intended dodge vector, especially after holding
movement/sprint into the dodge.

### Hypothesis

The physical dodge endpoint is contract-owned and latched at command start. Normal grounded sprint
must not become the target velocity during the first post-dodge handoff frame, or the client/server
can disagree by a small amount: dodge root ends at X, then grounded sprint replay projects toward
X plus an input/sprint-biased offset.

### Evidence

Logs showed dodge submits with `sprint=true` and input direction, while the remaining visible issue
was not the no-input path. The bridge `TryReplayPendingActionHandoff` used `Input.bSprint` inside
the dodge exit handoff window, and the client first forced grounded submit after dodge also used the
current sprint state.

### Change

- During `DodgeExitHandoff` replay, sprint is suppressed only while inside the dodge handoff window.
- The first forced grounded submit after a local dodge uses current input direction but sends
  `sprint=false`; if Shift remains held, later normal movement submits can sprint as usual.
- Baseline walk/run/turn, lateral sprint, dodge distance, and no-input dodge were not changed.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.
- Manual runtime validation still needed: input/sprint dodge should not show an end snap or
  `DodgeActive/DodgeExitHandoff decision=apply`.

### Guardrail

Sprint is a grounded locomotion state, not part of the dodge root. It may resume after the explicit
dodge handoff window, but it must not alter the action-owned endpoint.

## 2026-06-24 - Dodge Camera Must Not Carry Presentation Correction

### Symptom

After dodge rubber improved, a rapid input inversion still produced a camera detach: press forward,
release, press backward and dodge quickly. The player also reported a very small remaining dodge
micro-snap.

### Evidence

Recent logs showed most dodge snapshots no longer applied hard correction. The remaining traces were
usually `DodgeActive`/`DodgeExitHandoff` with `should_apply=false`, but the client still had camera
presentation correction machinery active. Code audit found that the generic visual correction path
still applied `RootCorrectionDelta` to both mesh and camera, even though the dedicated camera path
already treated Apeiron's Souls-style camera as non-corrected.

### Cause

The camera was allowed to carry root presentation correction debt. That is wrong for this camera
model: the capsule/root is authority, the mesh may smooth visual correction, but the camera boom must
stay attached to the root. Otherwise a small movement correction during rapid dodge input inversion
can look like the camera detached from the character.

### Change

- Generic authoritative visual correction now applies smoothing only to the mesh visual offset.
- Camera visual correction is cleared whenever generic root visual correction is applied.
- Defense/block locomotion snapshots are classified as grounded movement instead of `None`, so block
  state cannot use raw generic reconciliation as if it owned position.
- Dodge physics, dodge distance, walk/run/turn, leap and skill movement were not changed.

### Tests

- Build pending after this ledger entry.
- Manual validation needed: forward, release, backward + Alt quickly should not detach camera.
- Also validate input dodge end snap and compare the same camera/visual correction signatures for
  leap tomorrow.

### Guardrail

Camera ownership is not movement ownership. For the Apeiron third-person combat camera, presentation
correction may smooth the body mesh, but the camera must not store independent root correction debt.

### Follow-up Monitor

One-minute runtime monitor after the camera correction change did not show `CameraFocusDebt`,
`PresentationDrift`, `mode=None decision=apply`, or `should_apply=true` during the captured dodge
window. The remaining visible micro-snap signature is concentrated in `DodgeExitHandoff` with pending
grounded move replay, often clamped exactly at `current_error=65.0` / `dead_zone=65.0`. Next dodge
work should inspect the post-dodge action handoff replay endpoint and input transition timing, not
generic camera correction.

Use the same split tomorrow for leap: first prove whether the capsule/root receives
`should_apply=true`; if not, inspect landing handoff replay and visual/camera correction debt
separately.

## 2026-06-24 - Dodge Exit Replay Uses Action Endpoint And Client Handoff Curve

### Symptom

After the camera correction fix, no-input dodge was clean and the one-minute monitor did not show
hard snapshot apply. The remaining issue was a very small snap after input dodge, concentrated in
`DodgeExitHandoff` traces with pending grounded replay at the exact settle boundary:
`current_error=65.0` / `dead_zone=65.0`.

### Hypothesis

The replay target for pending movement after dodge was being rebuilt from the snapshot root location,
then blended linearly into grounded movement. The dodge endpoint itself is action-owned, and the
snapshot already publishes `ActionProjectedPosition`. Starting the replay from the raw root can make
the replay endpoint differ slightly from the client-owned handoff endpoint. The bridge also used a
linear handoff alpha while `TickAuthoritativeGroundedHandoff` uses eased alpha.

### Change

- `TryReplayPendingActionHandoff` now anchors `DodgeExitHandoff` replay at
  `Locomotion.ActionProjectedPosition` when it is within a DB/profile-derived trust envelope.
- The dodge handoff replay now uses the same `InterpEaseInOut(..., 1.5)` alpha as the client
  authoritative grounded handoff.
- The change is limited to the post-dodge handoff replay. It does not change dodge distance,
  walk/run/turn, leap, sprint suppression, camera correction, or DB contracts.
- When movement rubberband probe logs are enabled, the bridge logs
  `dodge_handoff_anchor source=action_projected` for this path.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.
- Manual runtime validation still needed: input dodge should stop showing the tiny post-dodge snap,
  and `DodgeExitHandoff` should no longer sit repeatedly at the exact deadzone boundary.

### Guardrail

This is not a deadzone increase and not an input disable. It aligns replay authority with the
action timeline endpoint already published by the server, then uses the same transition curve on the
bridge and character paths.

## 2026-06-24 - Dodge Exit Replay Must Not Consume Grounded Sprint Frames

### Symptom

Manual validation after the action endpoint/easing fix showed improvement for walk, lateral run and
backpedal, but forward sprint after dodge still produced a larger visible snap.

### Evidence

Recent `DodgeExitHandoff` logs showed the first replayed input was usually inside the deadzone, but
the same dodge trace could continue replaying two or three pending `move` inputs:
`replay_duration=0.199/0.265/0.298`, then return to `current_error=65.0` / `dead_zone=65.0`.
Those extra frames were normal grounded movement after the handoff window, but the bridge still
projected them from the dodge-exit replay path. Forward sprint made the target overshoot more than
walk, lateral run, or backpedal.

### Cause

`TryReplayPendingActionHandoff` used the dodge-exit handoff window for blending, but it did not limit
the replay duration to that window. As soon as replay consumed inputs beyond the action handoff, the
dodge reconciler became a second owner for normal grounded sprint frames.

### Change

- `DodgeExitHandoff` replay now stops at the explicit handoff window.
- The last replay step is clamped to the remaining handoff time.
- Inputs after the dodge-exit window are no longer projected as dodge-exit replay; they belong to
  the normal grounded movement reconciliation path.
- No sprint speed, dodge distance, deadzone, leap, walk/run/turn, or input availability was changed.

### Tests

- `PlainTestMapEditor Win64 Development` Unreal build passed with `-NoHotReload`.
- Manual runtime validation still needed: forward sprint after dodge should no longer show the larger
  snap, and `DodgeExitHandoff` logs should stop showing `replay_duration` beyond the configured
  dodge carry handoff window.

### Guardrail

Action handoff replay owns only the action transition window. It must never consume later grounded
movement frames just because those commands are pending behind an old action snapshot.

### Reverted

Manual validation reported new rubber in multiple directions after this clamp. The duration cut was
too mechanical: it prevented some forward sprint overshoot, but it also changed ownership timing for
other dodge exit directions that were already acceptable. The code change was reverted in Unreal.

The remaining AAA direction is not to clip replay duration in C++. It is to model the post-dodge
transition as an explicit, DB/profile-backed action-exit locomotion state shared by server and
client: latched dodge endpoint, exit velocity, blend curve, sprint re-entry policy, and handoff
completion. Deadzone remains a reconciliation tolerance, not the gameplay transition itself.

## 2026-06-24 - Explicit Dodge/Leap Exit Transitions Own Root Before GroundedMove

### Symptom

After the dodge endpoint/easing fix, the remaining micro-snap was not the no-input dodge path. It
appeared when input or sprint continued through the end of dodge, especially forward sprint. The
same architectural risk existed for leap landing: action root, pending movement replay, grounded
movement and sprint re-entry could all influence the first post-action frames.

### Hypothesis

The old `TickAuthoritativeGroundedHandoff` was a generic action handoff. Even when DB/profile
fields existed (`dodge_carry_handoff_ms`, `leap_grounded_carry_handoff_ms`, landing handoff
duration, exit direction and exit speed), the code did not make DodgeExit and LeapLanding explicit
runtime owners. Pending movement replay could also treat sprint as normal grounded velocity while
still reconciling an action handoff snapshot.

### Change

- Unreal now routes action exits through explicit functions:
  - `TickDodgeExitTransition`
  - `TickLeapLandingTransition`
  - shared `TickActionExitTransition` only for common mechanics.
- During the transition, the action endpoint/exit vector owns the root. Normal input is read as
  exit intent, but sprint is not applied as grounded sprint until the action transition completes.
- Dodge and leap completion are separated in `CompleteActionExitTransition`, so each action seeds
  or clears carry handoff through its own profile-driven path.
- Bridge pending-action replay now treats sprint as intent for both `DodgeExitHandoff` and
  `LeapLandingHandoff`; sprint resumes through later grounded movement snapshots instead of being
  projected inside the action replay path.
- No deadzone was increased. No replay window was mechanically clipped. No DB schema was added,
  because the canonical profile/proto fields already existed and are consumed by the runtime.

### Files

- `B:\Unreal Projects\PlainTestMap\Source\PlainTestMap\ApeironTestPlayerCharacter.h`
- `B:\Unreal Projects\PlainTestMap\Source\PlainTestMap\ApeironTestPlayerCharacter.cpp`
- `B:\Unreal Projects\PlainTestMap\Source\PlainTestMap\ApeironGameServerBridge.cpp`

### Validation

- `PlainTestMapEditor Win64 Development -NoHotReload` passed.
- `go build ./cmd/game-server` passed.
- `go build ./cmd/db-api` passed.
- Runtime validation is still required in PIE for:
  - dodge + W/Shift continuation;
  - dodge + lateral/backpedal continuation;
  - leap landing while holding W/A/D/Shift;
  - no regression in the already-good walk/run/turn path.

### Guardrail

Dodge and leap may share implementation helpers, but they must not share one ambiguous gameplay
state. The transition is the temporary movement owner; GroundedMove resumes only after explicit
completion.

## 2026-06-24 - Dodge Lateral Exit Keeps Contract Carry Through First Grounded Submit

### Symptom

After explicit dodge/leap transition separation, dodge felt much better but still showed a small
snap at the end when the player held only lateral movement (`A` or `D`) through the dodge exit.

### Hypothesis

The transition tick owned the root correctly, but completion still dropped the carry handoff when
input was held. Immediately after that, the forced post-dodge grounded submit also cleared the carry
path before normal grounded movement resumed. That made lateral input retake ownership without
consuming the final transition direction/speed.

### Change

- `CompleteActionExitTransition` now seeds the dodge carry handoff whenever the dodge contract says
  grounded handoff carries, even if the player is holding lateral input.
- The first forced grounded submit after dodge now preserves an active dodge carry direction instead
  of clearing it.
- This keeps the final action-owned exit vector alive for the first grounded frame and lets normal
  movement blend out through the existing profile-driven carry window.

### Files

- `B:\Unreal Projects\PlainTestMap\Source\PlainTestMap\ApeironTestPlayerCharacter.cpp`

### Validation

- `PlainTestMapEditor Win64 Development -NoHotReload` passed.
- Manual runtime validation needed specifically for: dodge while holding only `A`, dodge while
  holding only `D`, then immediately continuing walk/run.

### Guardrail

This is not a deadzone change and not an input disable. It keeps the explicit dodge-exit transition
as owner until it has handed off its final direction/speed to grounded movement.

## 2026-06-24 - Creature Skill Root Gets Explicit Action Exit Transition Owner

### Symptom

Recovered creature runtime could finish a moving skill root action and immediately let brain tactic
movement publish orbit/flank/pursuit. For wolf dodge/lunge/maul this risks a snap at action end:
the action endpoint/carry is dropped and tactical movement becomes root owner without a release
phase.

### Hypothesis

The player dodge/leap fix showed that action exits need an explicit temporary movement owner. The
creature equivalent is not player input replay; it is brain/tactic handoff. `skill_root` completion
needed a creature-side transition owner before tactical movement could write position again.

### Change

- Added `CreatureActionTransition` runtime state for creatures.
- `skill_root` motion completion now begins `creature_dodge_exit_transition`,
  `creature_leap_landing_transition`, or `creature_skill_exit_transition` based on the DB movement
  action contract.
- Transition duration uses existing contract recovery windows first; no new hardcoded tuning and no
  DB schema change in this slice.
- Tactical movement/root publication is gated while transition is active.
- Wolf AI state exposes `path_state=creature_action_transition` so runtime validation can see the
  owner before the wolf returns to circle/flank/pursuit.

### Files

- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\creature_action_transition.go`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\runtime.go`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\action_runtime_lifecycle.go`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\creature_action_runtime.go`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\creature_policy_publication.go`

### Validation

- `go build ./cmd/game-server` passed.
- Runtime validation still required for repeated wolf dodge/lunge/maul action exits in PIE.

### Guardrail

Creature transition is a movement authority state, not AI memory and not Unreal smoothing. Brain can
choose the next tactic while the transition is active, but it cannot move the creature root until the
transition completes.

## 2026-06-24 - Wolf Dodge/Lunge/Maul Contract Semantics Restored On Top Of Creature Transition

### Symptom

The first creature transition pass created an exit owner, but three skill-specific contract semantics
were still incomplete:

- `wolf_dodge` needed iframe/combat-pipeline state while its `skill_root` dodge motion is active.
- `low_fast_lunge_v1` is DB-authored as `action_type='leap'`, but creature `skill_root` was not using
  vertical root motion.
- `wolf_maul_lateral_counter_v1` uses `lateral_counter_contact`, but generic contact classification
  treated any `contact` string as source-stop, which prevents the maul from carrying through.

### Change

- Generalized dodge combat-pipeline detection from player `owned_locomotion` only to any
  `owned_locomotion` or `skill_root` action motion whose contract action/ability is `dodge`.
- Creature skill root now enables vertical root when the DB movement action contract declares leap
  semantics, airborne duration, vertical curve, or vertical motion model.
- `lateral_counter_contact` now classifies as carry contact instead of source-stop contact, allowing
  maul source movement to continue while the DB-driven control effect owns target displacement.

### Files

- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\impact_pipeline_bridge.go`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\creature_action_runtime.go`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\combat\contactpolicy\policy.go`

### Validation

- `go build ./cmd/game-server` passed.
- `go build ./cmd/db-api` passed.
- PIE validation still required for: wolf dodge iframe under player attacks, lunge arc/landing, and
  maul lateral carry/release.

### Guardrail

These are contract semantic restorations, not wolf-only movement hacks. The source of truth remains
the DB movement action contract and contact/control policy.

## 2026-06-24 - Creature Movement Compatibility Surface Completed For Wolf Dodge And Maul

### Symptom

The normal runtime loader correctly used `skill_movement_action_binding + movement_action_contract`,
but the compatibility API `GetSkillMovementEffect(skill_id)` still returned no rows for `maul` and
`wolf_dodge`. That made the DB look partially reconstructed even though the canonical runtime path
had contracts.

### Change

- Added compatibility rows in DB bootstrap for:
  - `wolf_dodge_lateral_leap_effect_v1`
  - `wolf_maul_lateral_counter_effect_v1`
- Rows mirror canonical `movement_action_contract` values and include metadata pointing back to the
  authoritative contract IDs.
- This does not make `skill_movement_effect` runtime authority; it only keeps legacy/tooling/Unreal
  compatibility surfaces complete.

### Files

- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron\bootstrap\018_skill_movement_effect_compat_seed.sql`

### Validation

- `go build ./cmd/db-api` passed.
- `go build ./cmd/game-server` passed.
- DB API and game server restarted.
- `GetSkillMovementEffect(maul)` returns `wolf_maul_lateral_counter_effect_v1`.
- `GetSkillMovementEffect(wolf_dodge)` returns `wolf_dodge_lateral_leap_effect_v1`.

### Guardrail

Compatibility rows must stay mirrors of canonical movement action contracts. If future runtime logic
needs action-exit tuning not represented by existing contracts, add a typed transition profile rather
than branching on these compatibility rows.

## 2026-06-24 - Wolf Lunge Low Raking Arc And Reduced Repeat Pressure

### Symptom

After creature `skill_root` started honoring the DB `low_fast_lunge_v1` leap contract, the wolf lunge
became physically correct in authority but wrong in feel: the arc climbed too high, landing inertia was
too long, and AI selection pressure made the wolf chain lunge too often.

### Cause

The lunge contract still carried pre-transition vertical and recovery values:

- vertical curve peak `120cm`;
- `jump_z_velocity=700`;
- `expected_apex_ms=320`;
- recovery/compat landing lock `240-520ms`;
- setup metadata `postLandingInertiaMultiplier=1.1/1.2`;
- lunge binding weights and cooldown allowed lunge to dominate bite/maul rotation.

Once server runtime correctly applied vertical root, these values became visible as an exaggerated
high leap instead of the intended fast, low, raking lunge.

### Change

- Tuned canonical DB `low_fast_lunge_v1` to a shallow raking arc:
  - vertical curve peak `44cm` at `t=0.26`;
  - recovery `180ms`;
  - `post_landing_inertia_multiplier=0.7`;
  - `jump_z_velocity=360`;
  - `expected_apex_ms=220`.
- Mirrored the same landing inertia in `GetSkillMovementEffect(lunge)` compatibility data.
- Increased lunge cooldown from `7200ms` to `9000ms`.
- Aligned the wolf behavior seed's supplemental `UPDATE apeiron.skill WHERE id='lunge'` with the same
  `9000ms` cooldown so seed order cannot reintroduce the old value.
- Reduced lunge slot/binding priority and usage weights so lunge remains a threat but does not crowd
  out bite/maul/dodge behavior.
- Aligned dev/reconstruction fixtures with the same lunge arc and recovery to avoid a fallback path
  reintroducing the old high jump.

### Files

- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron\bootstrap\009_skill_seed.sql`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron\bootstrap\014_action_runtime_contract_seed.sql`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron\bootstrap\016_wolf_behavior_contract_seed.sql`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron\bootstrap\018_skill_movement_effect_compat_seed.sql`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\contracts.go`
- `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\internal\gameapi\contracts_test.go`

### Guardrail

This is a contract/policy retune, not a wolf branch in server movement code. Runtime still honors the
canonical movement action contract. If PIE still shows excessive lunge height after this, investigate
Unreal creature visual Z/capsule offset separately before changing player leap or shared vertical
handling.

## 2026-06-24 - Wolf Lunge Long Low Arc And Creature Leap Ground Root Clamp

### Symptom

PIE validation still showed the wolf lunge as too high and too short. A second bug appeared when the wolf lunged while the player dodged: the wolf could remain slightly above the ground after the action.

### Cause

The previous lunge tune was directionally correct but still too tall for the placeholder scale and did not lengthen the full envelope. The floating bug was a runtime authority issue: creature action transition used the current action endpoint Z. For a creature leap, that endpoint can still carry airborne Z during passthrough/contact timing, so the post-action transition can preserve a contaminated height instead of landing on the creature's grounded root.

### Change

- Retuned `low_fast_lunge_v1` to a much lower and longer raking leap:
  - distance `918cm -> 1652cm`;
  - base speed metadata `1310 -> 1920`;
  - vertical curve peak `44cm -> 18cm`;
  - earlier apex `t=0.22`;
  - `jump_z_velocity=180`;
  - `expected_apex_ms=160`.
- Aligned the compatibility `SkillMovementEffect(lunge)` surface with the same distance/speed.
- Aligned skill range, target opportunity range, setup policy commit/preferred ranges, and behavior bindings so the AI does not select a long lunge through an old short-range envelope.
- Added entity-level `groundRootZ` tracking and made creature leap/action transitions land on that root Z instead of preserving airborne endpoint Z.
- Ground-clamped creature vertical-root action completion before starting transition.

### Guardrail

This does not special-case `wolf + lunge` in movement code. The runtime rule is generic: a creature vertical-root action that exits through a transition must use the entity's remembered ground root for the handoff. The wolf-specific part remains only in DB policy/contract tuning.
