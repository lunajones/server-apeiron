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
