# Apeiron Reconstruction North Star - 2026-06-22

This document is the current north star for recovering Apeiron after the project deletion.
It does not replace the detailed roadmaps. It cross-references them and decides what is
restored, partial, missing, or suspicious enough to audit before calling the game restored.

## Source Of Truth Order

1. Current live code in:
   - `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron`
   - `C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron`
   - `B:\Unreal Projects\PlainTestMap`
2. Most recent Codex chat recovery ledgers:
   - `docs/recovery/codex-chat-change-map.md`
   - `docs/recovery/chronological-chat-reconstruction-ledger-2026-06-22.md`
   - `docs/recovery/server-reconstruction-ledger-2026-06-22.md`
   - `docs/recovery/reconstruction-gap-audit-2026-06-22.md`
   - `docs/recovery/full-project-reconstruction-roadmap-2026-06-22.md`
   - `docs/recovery/codex-chat-roadmaps/*.md`
3. DB recovery ledgers:
   - `db-apeiron/docs/recovery/chat-recovery-ledger-2026-06-22.md`
   - `db-apeiron/docs/recovery/db-legacy-compatibility-audit-2026-06-22.md`
4. Unreal docs:
   - `PlainTestMap/Docs/unreal-aaa-movement-action-contract-roadmap.md`
   - `PlainTestMap/Docs/apeiron-aaa-hud-visual-identity-roadmap.md`
   - `PlainTestMap/Docs/unreal-client-aaa-project-architecture-roadmap.md`

If a recovered file says `legacy`, `fallback`, `compat`, or `recovered`, treat it as
evidence to audit, not final authority. Normal gameplay must be DB/profile/contract driven.

## Status Legend

- `RESTORED`: structure and runtime path exist, with tests or clear code evidence.
- `PARTIAL`: enough exists to run or test, but ownership/coverage/runtime parity is incomplete.
- `MISSING`: expected architecture from chat/docs is absent or empty.
- `SUSPICIOUS`: code exists but likely contains compatibility/fallback/monolithic debt.

## Current Cross-Project Matrix

| Scope | Status | Evidence Found | Gap / Risk | Next AAA Cut |
| --- | --- | --- | --- | --- |
| Git safety | `PARTIAL` | `server-apeiron` and `db-apeiron` are git repos and currently clean. | Unreal project root has no `.git` in `B:\Unreal Projects\PlainTestMap`. | Add Unreal source/config/content critical files to version control or a safe backup flow before more broad C++/asset edits. |
| DB migrations/protos/service surface | `PARTIAL` | DB has 43 migrations, 20 bootstrap SQL files, generated Go, and proto services for profile/skill/creature/player/world/cache/inventory/observability. Server `gameapi` now has a runtime contract requirement manifest for required movement profile, base actions, M1/R/F, wolf skills, combat core, defense, weapon kit, and wolf brain policy. DB now mirrors that manifest in `TestBootstrapSeedsMirrorServerRuntimeRequirementManifest` and `TestBootstrapSeedsMirrorRequiredSkillActionManifest`, proving bootstrap coverage for required runtime surfaces instead of relying on hollow compilation. | Current numbering is reconstructed/compact; original modern numbering from chats had later numbers. Some compatibility migrations are recovery-only. Unreal still needs equivalent consumer proof for every required field. | Keep compact numbering only with a compatibility map. Prove every Unreal/server-required field via proto, gRPC tests, and Unreal consumer validation. |
| DB action/runtime contracts | `PARTIAL` | Seeds exist for movement action contracts, skill action timing, temporal hitboxes, wolf behavior, weapon kit, combat defense, runtime movement reconciliation profile. Server strict coverage now validates required movement action contracts for id, ability key, action type, duration, reconciliation, phase policy, prediction policy, skill root/contact policy, binding/action id parity, action-manifest parity, and combat-mode slot references. | Some SQL rows still carry `legacy` or `reconstructed` metadata. `skill_movement_effect` remains compatibility-required while `GetSkillMovementEffect` is exposed. | For each runtime consumer, prove whether it uses canonical contract tables or legacy endpoint. Move `skill_movement_effect` to recovery-only only after no consumer needs it. |
| DB legacy compatibility | `PARTIAL` | `db-legacy-compatibility-audit-2026-06-22.md` classifies legacy surfaces; migrations 033-042 contain compatibility/finalization logic. `gameapi` now has a runtime surface classification table and `RuntimeStats` exposes `contracts.surface.*` statuses. Tests prove the normal server contract loader does not expose `GetSkillMovementEffect`. | Legacy DB endpoint still exists for compatibility and Unreal/external tool usage still needs the same consumer proof. | Keep `skill_movement_effect` as compat until Unreal/tool audit is complete, but do not allow it to become normal runtime authority. |
| Server movement resolver | `PARTIAL` | `internal/movement` exists with resolver, action contract registry, action timeline, kinematics, tests. `gameapi` now rejects move/dodge/leap/skill commands with missing or invalid movement contracts instead of using live fallback distances or resolver defaults. `SubmitCommand` now has explicit idempotent command replay protection: duplicate command ids are acknowledged without reapplying movement, and stale sequences are rejected before mutating position. | Need prove every player movement and skill movement path goes through resolver or the shared action contract path, not manual position writes. Unreal still needs the same strictness around profile fallback visibility. | Audit `player_skill_combat_system.go`, `gameapi/runtime.go`, and Unreal bridge for direct position authority and duplicated handoff logic. |
| Runtime movement reconciliation profile | `RESTORED` | DB has `runtime_movement_reconciliation_profile`; server validates the full rich profile; Unreal marks parsed profiles authoritative only when every required field is present and rejects fallback/incomplete profiles. | Unreal still keeps inert struct initialization values for editor/dev safety, but normal correction no longer treats them as authoritative runtime data. Unreal project root still lacks git safety. | Keep client defaults inert. Any new reconciliation field must be added to DB migration/seed, server validation, proto mapping, and Unreal parser validation in the same slice. |
| Skill movement: player F/R/M1 | `PARTIAL` | DB has movement bindings; server combat has `player_skill_combat_system.go` and `skill_movement_helpers.go`; Unreal bridge maps `SkillGroundedAction` and `PostActionGrounded`. Server guard tests now prove M1-1/M1-2/M1-3/R/F keep owned root during skill, lateral sprint input cannot apply normal movement during owned root, stationary skills do not teleport on submit, mid-action movement progresses through snapshots, repeated sprint-forward M1/R/F returns to grounded move, and post-action returns sprint-strafe after handoff. | History shows this regressed repeatedly. Unreal/PIE automation still must prove the same against real client prediction and aggressive yaw. | Run PIE/client validation for the same scenarios and keep server guard tests as CI baseline. Fix common owner, not per-skill patches. |
| Protected leap/dodge/turn baseline | `PARTIAL` | Unreal bridge has explicit dodge/leap/turn contracts and validation. Server has movement contracts/tests. `rubberband_guard_test.go` now proves leap/dodge own action motion, reject skill start while locked, survive rejected skill pressure, and return to grounded move after handoff; aggressive turn+yaw sprint strafe remains grounded move reconciliation. | Recent history shows unrelated changes broke leap/dodge/turn. Unreal/PIE automation still needs equivalent validation against real client prediction. | Keep server guard suite mandatory and add/maintain matching Unreal automation for leap, dodge, turn, sprint-strafe, and post-skill handoff. |
| Combat mode / weapon kit / slots | `PARTIAL` | DB has `weapon_kit`, `weapon_combat_mode`, `weapon_combat_mode_skill_slot`; Unreal and server expose `mode_slots` and combat mode ACK/snapshot fields. Server tests now prove `M1` is a valid authoritative slot, Bulwark owns M1/R/F, recovered Vanguard slots remain present but empty/disabled, and the runtime sequence Bulwark -> empty Vanguard -> Bulwark rejects/accepts basic attack correctly. | Need PIE validation that CTRL toggles snapshot/ACK/HUD exactly as server state, with no local fallback filling wrong mode. | Run PIE/client validation for startup Bulwark slots, CTRL switch to empty Vanguard, CTRL back to Bulwark, and no Q/R/F fallback in wrong mode. |
| HUD visual identity | `PARTIAL` | Unreal HUD docs and C++ HUD exist; skill icon prompt sheet exists. | Visual polish and frame art are not final; Unreal project has no git safety. | Finish HUD frame/token asset plan only after gameplay runtime is stable, or isolate it so HUD cannot affect movement. |
| Temporal melee hitboxes | `PARTIAL` | DB has temporal hitbox tables/seeds; server has `internal/hitbox` runtime/tests, and `gameapi` now evaluates enabled motion profiles through `impact_temporal.go` instead of expanding every sample into one static full-area hit. Player M1/R/F and wolf bite/lunge/maul recovery fixtures mirror DB motion profile IDs and sample geometry. Strict runtime coverage now rejects any damaging required skill that lacks temporal hitbox/motion profile data. Player and creature impacts now share `internal/combat/damagegroup.Runtime`, an instance-keyed pending damage-group runner that owns dedupe, skipped-window evaluation, and expiration outside `gameapi`. `gameapi` still owns entity trace/pipeline adapters. | PIE/debug visualization still needs proof, and static compatibility for canonical melee skills still needs isolation/removal after consumer proof. | Keep moving temporal hitbox geometry/presentation out of ad hoc runtime code while preserving one damage-group runner for player and creature. |
| Damage / defense / posture / stamina | `PARTIAL` | DB has skill damage/posture, combat core, defense contracts; server has `internal/gameapi/impact_pipeline_bridge.go`, temporal impact tests, and creature temporal skill impact tests. `gameapi` now sends runtime impacts through `internal/combat.ImpactResolutionPipeline`, including block/parry/iframe decisions. `RuntimeContracts` now loads player/wolf combat core profiles and player/creature defense contracts from DB/profile gRPC, and strict coverage fails if those contracts are missing. Damaging skills must also carry max targets plus temporal damage group/motion samples. Required skill movement contracts must define start phase, handoff policy, normal-input policy, phase window policy, prediction policy, root owner, and contact policy. The pending impact runner now blocks duplicate damage and duplicate enqueue for the same action instance. 2026-06-23 slice restored impact response profile flow: `creature_template.impact_response_profile` is DB/proto/repository/handler backed, normal server boot loads the wolf template through CreatureDataService, strict coverage requires the wolf impact response profile, runtime impacts carry `ImpactResponseProfile`, and snapshot damage events include `impact_response_profile` for Unreal VFX. 2026-06-23 follow-up restored skill impact control contracts: `skill_impact_profile` exposes `SkillControlEffect`s through proto/gRPC, player Shield Drive/Bash/Rush push controls are seeded in DB, `gameapi` loads them into `SkillRuntimeContract`, strict coverage rejects push/carry contact skills without control effects, and the shared impact pipeline emits applied control metadata in snapshot damage events. 2026-06-23 follow-up added contract-backed control motion: impact profile exposes distance/speed/direction policy, the seed derives control distance from skill movement distance, and `gameapi` starts target `actionMotion` from the applied control effect instead of teleporting or relying on metadata only. 2026-06-23 follow-up made applied impact control interrupt the target's old action root and cancel that action instance's pending delayed damage, so Shield Rush/Bash/Drive cannot leave a lunge/maul/bite damage schedule alive after valid control lands. | Recovery fixtures still exist for dev/test reconstruction only, but normal DB loading no longer invents local guard/core/control values. Damage-group orchestration still lives inside `gameapi`. Player impact response is still a named default until player template/equipment material profiles are introduced. | Keep stamina/defense/damage/action state under one authoritative runtime owner and split out only when the package boundary can preserve that ownership. Next: introduce player/equipment material response profiles and move more event metadata out of ad hoc string maps only if proto event shape becomes insufficient. |
| Player action runtime / ActionInstance | `PARTIAL` | `internal/combat/actionruntime` exists; combat system uses action instances/locks. `actionruntime.ActorKindCreature` now exists and `gameapi` starts creature skill `ActionInstance`s for wolf skill execution. Creature action timing is extended when the movement contract would otherwise outlive windup/active/recovery. | `gameapi` still has a recovered vertical-slice runtime, and queue/cancel/pass-through semantics are not fully unified across player and creature. | Continue centralizing phase/lock/cooldown/queue semantics for player and creature. Avoid separate creature lock/cooldown maps as final architecture unless they are wrapped behind the shared runtime owner. |
| Creature brain package | `PARTIAL` | `internal/ai` owns skill binding ranking, setup-policy movement tactics, cooldown unavailability, resource budget, repeat-skill penalty, orbit side memory, active-skill setup/continuation, and target perception/threat assessment. `gameapi` passes DB-loaded stamina/repeat/setup/pressure policy, skill costs, target velocity, defensive state, action commitment, stamina, and posture into the brain and spends creature stamina only when a creature action instance starts. Creature locomotion/AI publication and action lifecycle are now split out of `updateWolfPolicyLocked`. Creature skill root motion now owns position during the configured movement phase, while windup setup movement remains tactical/setup-owned. Creature action movement now has an explicit envelope for movement start/end, airborne window, landing inertia, passthrough, and stop-at-contact semantics. Orbit side switching now honors `side_flip_chance_multiplier` as deterministic policy chance instead of flipping every eligible window. Threat pressure weights for committed/closing/defensive/fleeing/low-resource target reads are now loaded from DB `pressure_policy_json` and required by strict coverage instead of living as Go tuning constants. Applied impact control now interrupts creature action root through the shared lifecycle helper and cancels pending delayed damage for that interrupted action instance, while normal creature completion does not cancel already scheduled damage-group windows. | Full queue/cancel parity is still incomplete outside impact-control interruption. Wolf grounding/visual plane still needs Unreal/runtime validation. | Continue from this module, not from `gameapi` branches: complete creature action-root runtime parity and contact/control response before calling creature runtime fully restored. |
| Wolf behavior contracts | `PARTIAL` | DB seed `016_wolf_behavior_contract_seed.sql` includes lunge/maul/bite/dodge, behavior runtime contract, target opportunity, orbit, evasion, setup, bindings. Unreal placeholder has creature visual skill motion. Server tactical creature movement now flattens decision vectors and preserves actor-root Z for grounded orbit/chase/setup movement. | Wolf floating/overlap still needs PIE validation because Unreal visual offsets and airborne presentation can still be wrong even when server grounded motion is correct. | Finish creature action-root/pass-through/landing semantics, then fix any remaining wolf ground plane/visual offset using authoritative locomotion data, not visual hacks. |
| Creature skill movement | `PARTIAL` | DB has movement action contracts for wolf skills and legacy compatibility for lunge. Unreal has predictive creature skill visual motion. Server tests now prove creature temporal damage waits for hitbox window and resolves once per skill instance. Server tests also prove wolf lunge windup uses setup movement before root motion, active lunge uses skill root motion, creature action timing cannot complete before movement action completion, lunge allows pass-through by contract, and post-airborne landing inertia remains root-motion-owned. Wolf AI skill movement presentation now derives takeoff/landing/distance/duration from the movement contract instead of fixed numbers. Contact policy classification is now centralized in `internal/combat/contactpolicy` and consumed by the creature movement envelope instead of local string checks in `gameapi`. Creature action contact response now uses the same classification at root-motion runtime: passthrough lunge does not stop on target body, while contact skills such as maul stop before overlap using hitbox/motion-profile geometry rather than a fixed body distance. | Target carry/control response and visual grounding still need PIE validation. | gRPC-check lunge/maul contracts, add server tests for carry/control interruption, then Unreal visual validation. |
| World/map/session pipeline | `PARTIAL` | Unreal has world volumes/exporter and map docs; server docs describe map/runtime startup integration. | Multi-region/map future is not final; current focus is PlainTestMap vertical slice. | Do not expand world streaming until combat/movement recovery is stable. Keep contracts ready but avoid broad scope. |
| Automated rubberband validation | `PARTIAL` | Server movement tests exist; Unreal bridge has validation routines and docs mention scanner. Server `rubberband_guard_test.go` now covers run + aggressive yaw, W+Shift M1 chain, stationary M1/F/R, post-F/R sprint handoff, lateral sprint during skill root, leap/dodge lock pressure, and turn-yaw strafe with strict locomotion/reconciliation assertions. | User-observed rubber can still escape if Unreal automation does not drive real continuous input/camera/skill sequences faithfully. Leap-hit/landing still needs runtime/PIE validation. | Mirror the server guard suite in Unreal automation and keep scanner strict; do not soften thresholds to hide correction. |
| Recovery fixtures/fallbacks | `PARTIAL` | `RecoveryFixtureRuntimeContracts()` is now explicitly named/commented as a dev/test reconstruction fixture. Runtime contract sources are canonical constants (`db_contracts`, `db_contracts_incomplete`, `recovered_runtime_fallback`, `unconfigured_runtime_contracts`) instead of ad hoc strings. `NewRuntime()` is tested as blocked/unconfigured with no movement manifest payloads, fixture readiness is tested as explicit opt-in only, and DB loads promote to `db_contracts` only after strict complete load. `gameapi` command execution rejects dodge/leap/skill movement with missing contracts. Unreal rejects incomplete/fallback runtime reconciliation profiles for normal corrections. `RuntimeContracts.CoverageReport` now classifies readiness blockers by category instead of relying on one flat error string. | Fixture still exists for dev/test reconstruction, and other legacy DB compatibility surfaces still need Unreal/tool consumer proof. | Keep fixture out of app boot. Continue classifying legacy surfaces as final authority, compat required, recovery-only, or dead; do not allow test fixture data to become normal runtime behavior. |

## Immediate Priority Order

1. `P0`: Restore creature brain architecture in `server-apeiron/internal/ai`.
   - Status: `PARTIAL`.
   - Done: rebuilt local brain/tactic/setup/memory/region brain system module and wired wolf decision selection through it.
   - Done: setup policies from DB are loaded into `WolfRuntimePolicy`, mapped into `internal/ai`, and used to drive lunge/maul setup movement during windup.
   - Done: 2026-06-23 slice added creature `ActionInstance` lifecycle for wolf skills and split creature locomotion/AI publication from action lifecycle.
   - Done: 2026-06-23 slice added creature skill root motion ownership by movement phase and extended creature action timing when movement contracts outlive action timing.
   - Done: 2026-06-23 slice added explicit creature movement envelope and contract-derived lunge presentation timings.
   - Done: 2026-06-23 slice added target perception/threat assessment to the creature brain and wired target velocity, block/parry/iframe, action commitment, stamina, and posture from `gameapi`.
   - Done: 2026-06-23 slice added creature root-motion contact response using contract policy plus hitbox geometry, preserving lunge passthrough and stopping maul before overlap.
   - Remaining: complete carry/control response parity and Unreal grounding validation.
2. `P0`: Re-audit player movement / skill movement authority.
   - Verify no duplicate position authority, no stale recovery handoff, no normal input fighting skill movement.
   - Protected baselines: leap, dodge, turn.
   - Done: server guard test proves lateral sprint input during current Bulwark skill root cannot apply normal-move displacement.
   - Done: 2026-06-23 slice added dedicated `rubberband_guard_test.go` covering stationary M1/R/F, sprint-forward M1/R/F loops, aggressive yaw sprint-strafe, lateral sprint during skill root, leap/dodge lock pressure, and post-action handoff.
   - Done: 2026-06-23 slice made normal `MOVE` require an explicit movement action contract before applying position or publishing locomotion, matching dodge/leap/skill strictness.
3. `P0`: Convert runtime fallback suspicion into hard proof.
   - Status: `PARTIAL`.
   - Done: runtime movement, skill movement, combat mode, combat core, and defense contracts now load through DB-backed runtime contracts with strict missing-contract validation; recovered guard/core values are confined to `RecoveryFixtureRuntimeContracts`.
   - Done: contract readiness now has a structured coverage report by movement, skills, wolf brain, combat core, defense, combat modes, and load issues.
   - Done: 2026-06-23 slice added runtime surface classification and tests proving `GetSkillMovementEffect` is not part of the normal server `ContractSource`.
   - Done: 2026-06-23 slice added DB bootstrap manifest tests that mirror the server runtime requirement manifest and required skill/action binding set.
   - Done: 2026-06-23 slice added source-guard tests proving default runtime is blocked/unconfigured, recovery fixture is explicit opt-in only, and incomplete DB loads never promote to complete DB contract source.
   - Remaining: finish Unreal/tool consumer proof for exposed compatibility endpoints and prove no normal boot path consumes recovery-only fixture data.
4. `P1`: Complete temporal melee hitboxes across all current player and wolf skills.
   - M1-1, M1-2, M1-3, Shield Bash, Shield Rush, bite, lunge, maul.
   - Done: player and creature temporal impacts now use the same pending runner; submit/update no longer apply damage through separate immediate shortcuts.
5. `P1`: Make combat mode source of truth airtight.
   - Bulwark owns current M1/R/F.
   - Vanguard is empty until real Vanguard skills are implemented.
   - No local fallback slot injection.
   - Done: server integration test covers Bulwark startup, Vanguard empty rejection, and Bulwark restore.
6. `P1`: Split or modularize combat resolution where the current `gameapi` concentration blocks correctness.
   - Do not split for aesthetics.
   - Split when DB-driven stamina/defense/damage/action state needs one authoritative runtime owner.
7. `P2`: HUD/VFX polish after movement and action runtime are stable.

## Recovery Work Breakdown

This is the detailed execution map. The immediate priority list above only says what to
attack first; this section says what "restored" means for every major recovered scope.

### A. Movement And Reconciliation

#### A1. Authoritative Player Movement

Status: `PARTIAL`

Expected final architecture:

- Server owns authoritative position, velocity, grounded/falling state, action phase, and movement mode.
- Unreal may predict locally, but must reconcile against server snapshots using DB-backed reconciliation profile data.
- Normal movement must not know skill-specific rules. It can use action/reconciliation categories, not skill ids.
- `leap`, `dodge`, and `turn` are protected baseline actions.

Evidence to cross-check:

- Server:
  - `internal/movement/resolver.go`
  - `internal/movement/action_contract_registry.go`
  - `internal/movement/action_timeline.go`
  - `internal/gameapi/runtime.go`
  - `internal/combat/player_skill_combat_system.go`
- DB:
  - `bootstrap/014_action_runtime_contract_seed.sql`
  - `bootstrap/020_runtime_movement_reconciliation_profile_seed.sql`
  - `migrations/027_action_runtime_contracts.sql`
  - `migrations/043_runtime_movement_reconciliation_profile.sql`
- Unreal:
  - `Source/PlainTestMap/ApeironGameServerBridge.cpp`
  - `Source/PlainTestMap/ApeironTestPlayerCharacter.cpp`
  - `Source/PlainTestMap/ApeironGameClientTypes.h`
- Recovered roadmaps:
  - `recuperacao-01-reconciliation-and-hud-roadmap.md`
  - `recuperacao-05-skill-movement-authority-roadmap.md`
  - `recuperacao-06-action-locks-dodge-leap-roadmap.md`
  - `recuperacao-11-movement-action-contract-roadmap.md`
  - `unreal-aaa-movement-action-contract-roadmap.md`

Acceptance criteria:

- Walk, sprint, lateral strafe, diagonal sprint, turn, dodge, and leap pass automated validation.
- Aggressive yaw + movement does not produce rubberband.
- Runtime profile values are received from DB/server snapshot, not invented in Unreal.
- Movement command replay protection is active and tested: duplicate command ids do not reapply movement, and stale sequences do not mutate player position.
- Any missing required movement contract fails visibly during startup/readiness.
- Runtime command execution rejects dodge/leap/skill movement if the required movement action contract is missing or invalid; it must not invent fallback distance.

#### A2. Skill Movement Separation

Status: `PARTIAL`

Expected final architecture:

- Combat owns skill intent, target, timing, cooldown, damage, and action instance.
- Movement owns movement resolution and locomotion publication.
- Skill movement is represented as a movement action contract and reconciliation category.
- Normal movement input during skill phases obeys the skill contract:
  - windup: optional aim/micro movement by contract;
  - active/cast: skill movement owns root when configured;
  - recovery: normal movement returns only through explicit handoff.

Acceptance criteria:

- M1-1, M1-2, M1-3, Shield Bash, Shield Rush, wolf lunge, and maul do not publish locomotion from two owners.
- `SkillGroundedAction` remains active while the action is still in recovery if the server still owns the action.
- `PostActionGrounded` appears only after action completion/handoff.
- No direct transform/position write bypasses resolver for moving entities.
- Server guard: `TestRuntimePostSkillHandoffReturnsSprintStrafeForCurrentBulwarkSkills` covers M1-1, M1-2, M1-3, Shield Bash/R, and Shield Rush/F.

#### A3. Rubberband Regression Suite

Status: `PARTIAL`

Required test scenarios:

- Stationary M1 chain: click 1, click 2, click 3.
- W + Shift + repeated M1 chain.
- A/D + Shift + camera yaw in same direction.
- A/W + Shift + camera yaw to the opposite side, then D/W + Shift inverse.
- F and R stationary.
- F and R while sprinting forward.
- F/R into immediate strafe/sprint handoff.
- Server coverage exists for post-skill sprint-strafe handoff on M1-1, M1-2, M1-3, R, and F.
- Server coverage exists for repeated `Shift+W+R/F` returning to grounded move after each skill.
- Leap stationary and leap running.
- Leap while hit.
- Dodge into movement and dodge into skill attempt.
- Turn while moving and turn while sprinting.

Acceptance criteria:

- Scanner is strict. Do not lower thresholds to hide correction.
- Every failed scenario produces enough log context to classify server authority, DB contract, Unreal prediction, and snapshot/reconciliation.
- A future HUD/DB/skill change cannot pass CI if it breaks protected movement baseline.

### B. Player Combat And Weapon Kit

#### B1. Basic Attack Combo

Status: `PARTIAL`

Expected final architecture:

- Basic attack is separate from generic active skills.
- M1 combo belongs to the active combat mode/loadout.
- Combo stage timing, movement, hitbox, damage, posture, and recovery are DB-backed.
- No cooldown on normal basic attack unless a specific stage contract explicitly defines a combo cadence/lock.

Recovered design target:

- M1-1: forward wave/strike starting from player, advancing roughly one and a half player cylinders forward, about one cylinder wide.
- M1-2: directional slash sweeping across a 90-degree window, not an instant full arc.
- M1-3: shield/body push movement, narrower than earlier wide version, with push/contact behavior and smooth post-action handoff.

Acceptance criteria:

- Combo state advances reliably with the intended input window.
- M1 does not require spam to reach stage 3.
- No rubberband on any stage stationary or while sprinting/strafe handoff.
- Temporal hitbox matches swing progression.

#### B2. Shield Bash / Shield Rush

Status: `PARTIAL`

Expected final architecture:

- Current Bulwark slots:
  - `R`: Shield Bash.
  - `F`: Shield Rush.
  - `Q`: empty for now.
- Both skills use DB movement action contracts.
- Shield Rush damage/contact begins around half a player cylinder ahead, so contact feels attached to the shield/body instead of disconnected.
- Shield Bash/Rush can push multiple targets when the contract allows it.

Acceptance criteria:

- R and F load only in Bulwark.
- R and F have skill movement, damage, temporal hitbox/debug visualization, cooldown UI, and post-action handoff.
- No local fallback slot causes R/F to appear in Vanguard.

#### B3. Combat Modes / Weapon Kit / Slots

Status: `PARTIAL`

Expected final architecture:

- Weapon kit owns available combat modes.
- Combat mode owns selected skill slots.
- Player build/loadout eventually chooses slots per mode.
- CTRL toggles active combat mode with server ACK/snapshot as authority.
- Vanguard can be empty now; empty means empty, not local fallback.

Acceptance criteria:

- Startup mode and hotbar are consistent.
- CTRL Bulwark -> Vanguard -> Bulwark works repeatedly.
- Empty Vanguard shows empty skill slots.
- Bulwark restores M1/R/F.
- `mode_slots` from server are the source of truth when combat mode is enforced.

#### B4. Heavy Attack / Fatality Slot

Status: `MISSING`

Expected final architecture:

- Heavy attack is a basic weapon action triggered by hold input, not a generic active skill.
- Fatality slot uses `G`, high cooldown, selected by build, per weapon kit/combat mode.
- No placeholder skill should be selectable until implemented.

Acceptance criteria:

- DB has schema/contract room for heavy and fatality without fake gameplay rows.
- Unreal HUD can display empty/reserved slots without binding invalid skills.
- Server rejects missing fatality skill as unselected/unavailable, not `action_locked` ambiguity.

### C. Hitbox, Damage, Defense, And Resource Runtime

#### C1. Temporal Melee Hitboxes

Status: `PARTIAL`

Expected final architecture:

- Server resolves melee damage using temporal/swept volumes, not a static full-area toggle.
- Each skill defines damage group, motion profile, samples/segments, and profile shape.
- Client debug draws the advancing damaging region, not old static sphere/noise symbols.

Recovered/proven in current code:

- `internal/gameapi/impact_temporal.go` evaluates an enabled motion profile at a hitbox-window-normalized time.
- `internal/gameapi/impact.go` no longer expands reach/lane/angle from every motion sample when a temporal profile exists.
- Runtime temporal hitboxes no longer invent a recovered default half-lane when radius/width is absent. Strict coverage validates temporal motion sample geometry, and `TestStrictRuntimeCoverageRejectsTemporalMotionSampleWithoutGeometry` blocks canonical skill hitboxes without contract-defined width/radius.
- `bootstrap/015_temporal_hitbox_seed.sql`, `bootstrap/016_wolf_behavior_contract_seed.sql`, and recovered server fixtures use one coordinate convention: `offset_x` is forward and `offset_y` is lateral.
- Tests prove future capsule/arc samples do not hit at the first active slice for player and wolf skills.
- `internal/combat/damagegroup.Runtime` owns pending temporal impact windows, dedupe, skipped-window evaluation, and expiration for both player and creature scheduled impacts.

Current required conversions:

- `player_basic_attack_1`
- `player_basic_attack_2`
- `player_basic_attack_3`
- `player_shield_bash`
- `player_shield_rush`
- `bite`
- `lunge`
- `maul`

Acceptance criteria:

- A target only takes damage when it intersects the active volume at that moment.
- Same target takes one hit per swing unless multi-hit is contract-enabled.
- Max targets and max hits per target are distinct and DB-backed.
- Static arc/shape compatibility cannot override canonical temporal profiles.
- DB seed tests lock Shield Rush front-contact geometry using the same forward/lateral convention as the server runtime.

#### C2. Damage Pipeline

Status: `PARTIAL`

Expected final architecture:

- One damage pipeline handles player and creature skills.
- Pipeline resolves: hit confirmation, dodge/iFrame, block, parry, posture, stamina, health, control/push, death.
- Damage and posture values come from skill/runtime contracts.
- Resistances are future-ready but not required to block current vertical slice.

Recovered/proven in current code:

- Player and creature runtime contracts carry DB/recovered damage and posture values for current M1/R/F and wolf bite/lunge/maul.
- Creature temporal skill impact waits for the hitbox window and marks a skill runtime instance after a hit so bite/lunge/maul cannot apply duplicate damage every tick.
- `gameapi` impact resolution now adapts runtime entities into `internal/combat.ImpactResolutionPipeline`; block, parry, iframe, final health damage, and posture damage are resolved by the shared combat pipeline instead of a local `if blocked` branch.
- Damage event feedback is now source-of-truth from server snapshot events: `runtimeSkillImpact` carries `ImpactType` and `ImpactResponseProfile`, `GetSnapshot` emits `ENTITY_EVENT_TYPE_DAMAGE_APPLIED`, and Unreal consumes `impact_response_profile` before falling back to generic impact type.
- Creature target material feedback comes from `creature_template.impact_response_profile` loaded through DB/proto/repository/CreatureDataService into runtime contracts. Strict runtime coverage rejects DB boot if the wolf profile is absent.
- Skill impact control feedback now comes from DB-backed `SkillImpactProfile.ControlEffects`: Shield Drive, Shield Bash, and Shield Rush carry push/carry control effects through gRPC into `SkillRuntimeContract`, `gameapi` passes them into `internal/combat.ImpactResolutionPipeline`, and snapshot damage events publish `control_applied`, `status_applied`, `control_type`, and `control_release_policy`.
- Control response movement is no longer only a status marker: `SkillControlEffect` carries distance/speed/direction policy, and an applied control effect creates target `actionMotion` so push/carry progresses through the same action-motion timeline used by other authoritative movement.

Acceptance criteria:

- Creature can die.
- Blocked hits obey defense contract.
- Parry/block are not artificial side checks outside the pipeline.
- `gameapi` loads `combat_core_player_sword_shield_v1`, `combat_core_steppe_wolf`, `player_shield_guard_v1`, and `wolf_attack_vs_guard_v1` through `RuntimeContracts`; missing DB combat contracts must fail strict coverage instead of leaking recovered fixture data.
- Damage events give Unreal enough data for feedback/VFX, including `impact_response_profile`, `impact_type`, damage/posture amounts, block/parry flags, and source/target profiles.

#### C3. Block / Parry / Posture / Stamina

Status: `PARTIAL`

Expected final architecture:

- Block has directional arc, stamina drain while held, and stamina damage on block.
- Dodge has full iFrame from input accepted to dodge end.
- Creature dodge/evasion also has guaranteed invulnerability through its action window.
- Stamina exhaustion blocks stamina spending until full regen, with regen penalty while exhausted.
- Running, dodge, holding block, and configured effects consume stamina; normal hits do not randomly spend stamina unless contract says so.

Acceptance criteria:

- DB owns defense/stamina contract values.
- Server owns enforcement.
- Unreal only presents resource bars and lock reasons.
- No default block angle or stamina number is final gameplay authority unless loaded from DB.

#### C4. Impact Response / Push / Knockback

Status: `PARTIAL`

Expected final architecture:

- Push/drag/knockback are contract-backed impact responses.
- Shield Rush/Bash and M1-3 push targets using body/capsule-aware contact rules.
- Wolf landing/airborne passthrough is respected unless hit by a valid control skill.

Acceptance criteria:

- Push does not rubberband source or target.
- Multiple targets can be pushed if contract allows.
- Wolf airborne lunge can pass through target unless a control skill applies.
- Hit impact and movement authority remain one pipeline, not two conflicting corrections.
- Material/visual response is no longer inferred only by runtime type for creatures; creature template owns the response profile and damage events carry it to Unreal.
- Player shield/drive push controls are now contract-backed; push/carry contact policies require explicit control effects in strict runtime coverage.
- Shield Drive/Bash/Rush target movement is now contract-backed via `SkillControlEffect` motion fields; the runtime creates target `actionMotion` for applied controls instead of applying a one-tick position snap.
- Strict runtime coverage now validates each enabled `SkillControlEffect` as a full motion contract: control type, release policy, direction policy, duration, distance, and speed are required. `TestStrictRuntimeCoverageRejectsIncompleteControlEffectMotion` prevents a half-seeded push/control row from reaching normal runtime and falling into local defaults.
- Applied impact control now has higher motion authority than the target's previous skill root: it interrupts the target action instance through `interruptEntityActionRuntimeLocked`, cancels that instance's pending delayed damage schedule, runs as `impact_control` action motion, and returns the target to grounded/post-impact locomotion on completion. `TestCreatureActionCompletionDoesNotCancelPendingImpactSchedule` and `TestCreatureActionClearDuringActiveCancelsPendingImpactSchedule` lock the difference between normal completion and interruption.
- The old `gameapi` directional-block helper with a fallback 180-degree arc was removed; block/parry resolution now remains in `internal/combat` and requires DB/proto `CombatDefenseContract.FrontalArcDeg`.

### D. Creature Runtime

#### D1. Creature Brain

Status: `PARTIAL`

Expected final architecture:

- `internal/ai` owns creature decision making:
  - perception target;
  - tactic selection;
  - movement tactic;
  - commitment state;
  - skill setup policy;
  - skill ranking;
  - evasion policy;
  - memory/repeat penalties;
  - cooldown/resource budget.
- Combat system executes chosen skill using the same action/runtime language as player.
- DB behavior contracts configure creature identity.

Required files to reconstruct or reintroduce:

- `internal/ai/creature_brain.go` - `RESTORED`
- `internal/ai/skill_setup.go` - `RESTORED`
- `internal/ai/tactics.go` - `RESTORED`
- `internal/ai/memory.go` - `RESTORED`
- `internal/ai/region_brain_system.go` - `RESTORED`
- server-side provider/wiring for DB behavior runtime contracts

Current restored behavior:

- `gameapi` converts DB-loaded `WolfRuntimePolicy` and `CreatureSkillBehaviorRuntimeBinding` into `ai.Policy`.
- The regional brain system chooses bite/lunge/maul/dodge/orbit from policy/bindings and preserves orbit side memory independently per creature.
- Creature setup policies from DB (`wolf_lunge_flank_windup_v1`, `wolf_lunge_chase_windup_v1`, `wolf_maul_pressure_counter_v1`) are now part of `WolfRuntimePolicy`, strict coverage, and the `internal/ai` policy. Lunge windup can circle/curve by contract instead of jumping straight to target from a `gameapi` branch.
- `gameapi` now wraps creature skill execution in `actionruntime.Instance` with `ActorKindCreature`; `SkillRuntimeState.State` publishes action phase instead of the old action-name string, cooldown/stamina spend happen only on action start, and completed actions clear back to idle through `completeCreatureActionRuntimeLocked`.
- Creature tactical movement publication is separated from action lifecycle, and grounded creature decision motion preserves Z so orbit/chase/setup cannot accidentally lift the actor root.
- Creature action root motion now starts from the movement binding phase (`starts_at_phase`), uses the movement action contract as physical source of truth, and prevents tactical brain movement from applying in the same tick once skill root motion owns position.
- Creature action timing is extended from contract data if movement start offset plus movement action duration would otherwise outlive action windup/active/recovery, preventing early completion/handoff during lunge/maul movement.
- Creature skill cooldowns are tracked by the runtime and passed to the brain as unavailable skills; the brain skips cooldown-blocked bindings instead of repeating them.
- Creature skill stamina costs come from DB-loaded skills. Behavior `stamina_policy_json` controls max stamina, dodge cost multiplier, and regeneration. The brain skips unaffordable skill bindings before selection, and `gameapi` spends stamina only when the skill starts.
- `target_memory_ms` and behavior `pressure_policy_json.repeatSkillPenaltyMultiplier` feed repeat-skill penalty in `internal/ai`, so lunge/dodge/maul repetition pressure is policy-driven instead of hardcoded by wolf branch.
- `internal/ai.ValidatePolicy` now owns creature brain contract completeness. Strict runtime coverage fails if DB/fake/runtime policies omit range/speed tuning, threat weights, vulnerability multipliers, tactical destination distance, evasion lateral/backstep/pressure values, setup policies, or enabled skill bindings. Wolf range/speed values come from `range_policy_json`; threat tuning comes from `pressure_policy_json`; dodge direction bias comes from `creature_evasion_policy`, not package constants. The brain no longer silently substitutes chase/orbit/bite/default values for missing lunge, retreat, setup, range, or speed tuning; missing data is a contract failure, not runtime improvisation.
- Creature action lifecycle clearing is centralized in `action_runtime_lifecycle.go` instead of being split between creature completion, non-skill clear, and impact-control interruption. Completion, interruption, pending impact cancellation, and terminal skill runtime publication now have one code path.
- `gameapi` applies movement and publishes snapshot state; it no longer owns the lunge/maul/dodge decision schedule.
- Recovered runtime contracts now include behavior bindings so recovery/dev fixtures exercise the same decision route as DB-backed runtime.

Acceptance criteria:

- Wolf attacks, dodges, lunges, bites, mauls, flanks, circles, retreats, and punishes through policy.
- No wolf-only hardcoded dodge count or lunge behavior outside behavior contracts/policies.
- Creature action state uses phase/lock/cooldown/queue/movement intent consistently with player action runtime.

#### D2. Wolf Behavior

Status: `PARTIAL`

Expected final behavior:

- Wolf is difficult to hit but not impossible.
- Dodge is lateral/back diagonal, low, fast, and stamina/policy governed.
- Lunge setup can circle/run during windup and commit into a natural leap.
- Lunge travels through/over target unless valid shield/control response interrupts.
- Maul appears as pressure/counter when player overcommits.
- Bite motivates close-range pressure.
- Flank/circle side should persist naturally, not flip every few seconds.
- Side switching must be policy-driven by orbit duration/cooldown/chance, not a periodic always-flip when the chance field is nonzero.
- Wolf must stay on ground plane except during intended leap/dodge arc.

Acceptance criteria:

- Wolf no longer floats/orbits above ground.
- Wolf does not enter player capsule like broken overlap.
- Lunge damage timing matches active contact, not early damage.
- Post-landing inertia exists and then returns to tactical brain.

### E. DB / Proto / gRPC Reconstruction

#### E1. DB Contracts, Migrations, Seeds

Status: `PARTIAL`

Expected final architecture:

- DB owns tunable gameplay data:
  - movement profiles;
  - movement action contracts;
  - reconciliation profiles;
  - skill timing;
  - skill movement bindings;
  - temporal hitboxes;
  - damage/posture/stamina/defense;
  - weapon kits/combat modes/slots;
  - creature behavior/evasion/setup/orbit/opportunity;
  - world/spawn/templates.

Acceptance criteria:

- Fresh DB migration + seed gives complete vertical slice.
- Recovered DB compatibility does not define final gameplay.
- Every server required contract has a gRPC/proto endpoint or static preload path.
- Seeds are not "placeholder gameplay" unless explicitly disabled/unselectable.

#### E2. Proto / Server DB Consumption

Status: `PARTIAL`

Expected final architecture:

- Proto exposes exactly what server and Unreal need.
- Server consumes DB contracts through clear loaders.
- Missing required contract is startup/readiness failure.
- Optional/future contracts are nullable/absent without silently inventing behavior.

Acceptance criteria:

- Server readiness reports missing contract categories. `gameapi.Readiness` now runs `RuntimeContracts.ValidateRequiredCoverage`; unconfigured/partial DB runtimes return blockers instead of `Ready: true`.
- Strict coverage rejects skill movement binding/action mismatches, skill action manifest mismatches, and enabled combat-mode slots pointing at unloaded skills.
- RuntimeStats exposes `contracts.required.*` per required runtime contract from a single manifest, so missing/disabled/mismatched runtime slices are visible without reading seed files.
- Runtime logs show contract ids/hashes.
- No gameplay fallback path is reachable during normal boot.

#### E3. Legacy Compatibility Audit

Status: `PARTIAL`

Expected final architecture:

- Legacy compatibility exists only to migrate/recover old DB shapes.
- Runtime never prefers legacy compatibility over canonical contract.
- Every legacy object is classified:
  - `final_authority`
  - `compat_runtime_required`
  - `recovery_only`
  - `dead`

Acceptance criteria:

- `skill_movement_effect` has a clear exit plan.
- Compatibility migrations have comments explaining why they remain.
- Dead legacy paths are removed only after proof and tests.
- Server runtime stats expose `contracts.surface.*` so compatibility status is visible at runtime.

### F. Unreal Bridge, HUD, VFX, And Validation

#### F1. Unreal Bridge / ACK / Snapshot Runtime

Status: `PARTIAL`

Expected final architecture:

- Unreal sends movement/skill/combat mode commands with sequence, tick, contract hash, yaw/input space when relevant.
- Server ACK and snapshot override local assumptions.
- Combat mode state, slot state, cooldown, lock reason, windup/cast/recovery, resources, and reconciliation profile are presented from server data.

Acceptance criteria:

- `action_locked`, `STA`, mode-lock, cooldown, and empty-slot messages are distinct.
- Client never blocks/permits a skill locally in a way that contradicts server source of truth.
- Snapshot timeline and correction scanner catch visible rubberband.

#### F2. HUD / Hotbar / Visual Identity

Status: `PARTIAL`

Expected final architecture:

- HUD is clean, stylized, Apeiron-owned visual identity.
- Bars show health, stamina, posture without text labels.
- Hotbar shows slot key labels and icons, not debug text.
- Windup/commitment meter appears briefly for player skill timing.
- Mode swap is visible but compact.
- Consumable bar shares style.

Acceptance criteria:

- HUD cannot affect movement runtime.
- Skill icons follow prompt sheet and gender/weapon visual variant rules.
- Empty slots are visually intentional.
- Debug text can be disabled separately from gameplay HUD.

#### F3. Skill VFX / Debug Visualization

Status: `PARTIAL`

Expected final architecture:

- Debug hitbox visualization can show temporal progression of damaging volume.
- Old spheres/static orange zones can be disabled.
- VFX is presentation only; damage remains server-authoritative.

Acceptance criteria:

- Player can see advancing strike line/volume during tests.
- Debug render matches server temporal hitbox samples.
- Runtime can disable debug visuals for normal gameplay.

### G. Recovery Safety / Ops

#### G1. Recovery Hardening / Git / No Delete

Status: `PARTIAL`

Expected final architecture:

- `server-apeiron` and `db-apeiron` are always committed before risky reconstruction.
- Unreal project has protection for source/config/content.
- Destructive actions require dry-run, narrow target list, quarantine/recycle preference, and sentinel audit.
- Recovery docs stay in repo and are updated after each slice.

Acceptance criteria:

- No broad delete/cleanup commands.
- No normal development immediately after destructive operation without audit.
- Every reconstruction slice has Git status checked before/after.

## Thread Roadmap Coverage Map

| Thread Roadmap | Primary Scope | Must Be Reflected In Live Code |
| --- | --- | --- |
| `recuperacao-01` | Reconciliation + HUD | Snapshot timeline, correction scanner, HUD state without movement side effects. |
| `recuperacao-02` | Combat mode + command queue + leap handoff | CTRL ACK/snapshot authority, command queue, leap landing handoff. |
| `recuperacao-03` | Sword/shield skills + VFX | Current sword/shield skill identity, icon prompts, visual feedback. |
| `recuperacao-04` | AAA roadmap governor | Process rules, no hardcoded/fallback final behavior, roadmap discipline. |
| `recuperacao-05` | Skill movement authority | Combat intent vs movement ownership; no duplicate locomotion publishers. |
| `recuperacao-06` | Action locks / dodge / leap | Skill/action gates must not trap player or break protected movement. |
| `recuperacao-07` | Wolf lunge post-landing | Lunge setup, leap, passthrough, landing inertia. |
| `recuperacao-08` | Parry + wolf lunge VFX | Directional defense and creature telegraph/VFX. |
| `recuperacao-09` | Hitbox target selection | Multi-target, max-target vs hit-count, fair server hit selection. |
| `recuperacao-10` | Directional block | Front arc block/parry rules through damage pipeline. |
| `recuperacao-11` | Movement action contract | DB-backed action contracts, hash sync, no magic defaults. |
| `recuperacao-12` | Impact response profile | Push/knockback/contact response and skill impact movement. |
| `recuperacao-13` | Bridge session / leap / despawn | Session stability, leap ACK, creature despawn grace. |

## Execution Strategy

1. Restore missing architecture before tuning behavior.
   - Start with creature brain because live server currently lacks the expected AI package.
2. For every scope, prove the DB contract first.
   - Migration exists.
   - Seed exists.
   - Proto/gRPC exposes it.
   - Server loader consumes it.
   - Unreal receives only what it should present/predict.
3. For movement/rubberband, list all failing scenarios before fixing.
   - Compare hypotheses.
   - Pick common denominator.
   - Protect leap/dodge/turn.
4. For legacy/fallback, classify before removal.
   - Remove only after proving normal runtime no longer consumes it.
5. For gameplay-feel changes, validate with both automated tests and PIE/manual runtime.

## Non-Negotiable Reconstruction Rules

- Do not delete or cleanup broad folders. If cleanup is needed, quarantine with a dry-run path list first.
- Do not call anything AAA while duplicate authority remains active for the same behavior.
- Do not invent gameplay values in Go or Unreal when DB contracts should own them.
- Do not fix skill movement by changing normal movement feel.
- Do not fix creature behavior with wolf-only branches when a behavior policy can express it.
- Do not trust recovered compatibility rows as final design without runtime consumer proof.
- Do not consider a slice done without focused tests and, when gameplay feel is affected, runtime/PIE validation.

## Working Definition Of Restored Apeiron

Apeiron is considered restored when:

- DB boots from migrations/seeds and exposes all required contracts through proto/gRPC.
- Server starts only from DB contracts in normal mode and fails loudly on missing required contracts.
- Player movement, leap, dodge, turn, sprint/strafe, M1/F/R skill movement, and post-action handoff are smooth.
- Player current Bulwark kit works: M1 combo, Shield Bash on R, Shield Rush on F; Vanguard remains intentionally empty.
- Temporal hitbox runtime matches directional/advancing damage intent for all current player and wolf melee skills.
- Wolf uses a restored creature brain: tactic, setup, evasion, skill ranking, memory, cooldown/resource budget, lunge/maul/bite/dodge behavior.
- Creature stays grounded visually and physically except during intended airborne skill phases.
- HUD displays DB-backed mode/slots/cooldowns/resources without affecting movement runtime.
- Server and DB tests pass, Unreal builds, and runtime validation covers the user-observed rubberband cases.

## Next Document Update Required

After each reconstruction slice, update this document with:

- status transition for affected scope;
- evidence path or test name;
- whether any fallback/legacy path remains;
- manual runtime validation still needed.
