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
| DB migrations/protos/service surface | `PARTIAL` | DB has 43 migrations, 20 bootstrap SQL files, generated Go, and proto services for profile/skill/creature/player/world/cache/inventory/observability. | Current numbering is reconstructed/compact; original modern numbering from chats had later numbers. Some compatibility migrations are recovery-only. | Keep compact numbering only with a compatibility map. Prove every Unreal/server-required field via proto and gRPC tests. |
| DB action/runtime contracts | `PARTIAL` | Seeds exist for movement action contracts, skill action timing, temporal hitboxes, wolf behavior, weapon kit, combat defense, runtime movement reconciliation profile. | Some SQL rows still carry `legacy` or `reconstructed` metadata. `skill_movement_effect` remains compatibility-required while `GetSkillMovementEffect` is exposed. | For each runtime consumer, prove whether it uses canonical contract tables or legacy endpoint. Move `skill_movement_effect` to recovery-only only after no consumer needs it. |
| DB legacy compatibility | `SUSPICIOUS` | `db-legacy-compatibility-audit-2026-06-22.md` classifies legacy surfaces; migrations 033-042 contain compatibility/finalization logic. | Legacy columns can hide missing canonical contracts if consumed accidentally. | Add runtime usage proof table: `final_authority`, `compat_runtime_required`, `recovery_only`, `dead`. Fail normal boot if required canonical contract is missing. |
| Server movement resolver | `PARTIAL` | `internal/movement` exists with resolver, action contract registry, action timeline, kinematics, tests. `gameapi` now rejects dodge/leap/skill commands with missing or invalid movement contracts instead of using live fallback distances. | Need prove every player movement and skill movement path goes through resolver or the shared action contract path, not manual position writes. Unreal still needs the same strictness around profile fallback visibility. | Audit `player_skill_combat_system.go`, `gameapi/runtime.go`, and Unreal bridge for direct position authority and duplicated handoff logic. |
| Runtime movement reconciliation profile | `PARTIAL` | DB has `runtime_movement_reconciliation_profile`; server loads it; Unreal has `FApeironMovementReconciliationProfile`. | Unreal still has numeric struct defaults and `ApeironPositiveOr(...)` fallback-style values. Some are safe C++ defaults, but not final gameplay authority if DB profile missing. | Convert missing profile in normal runtime into explicit failure/report, not silent gameplay defaults. Keep C++ values only as inert initialization or editor/dev safety. |
| Skill movement: player F/R/M1 | `PARTIAL` | DB has movement bindings; server combat has `player_skill_combat_system.go` and `skill_movement_helpers.go`; Unreal bridge maps `SkillGroundedAction` and `PostActionGrounded`. Server guard test now proves M1-1/M1-2/M1-3/R/F keep owned root during skill and return sprint-strafe after handoff. | History shows this regressed repeatedly. Unreal/PIE automation still must prove the same against real client prediction and aggressive yaw. | Extend movement regression suite for stationary M1, M1 chain, F/R, post-action sprint/strafe, and aggressive yaw. Fix common owner, not per-skill patches. |
| Protected leap/dodge/turn baseline | `PARTIAL` | Unreal bridge has explicit dodge/leap/turn contracts and validation. Server has movement contracts/tests. | Recent history shows unrelated changes broke leap/dodge/turn. | Make a guard test suite: any skill/HUD/DB change must run leap, dodge, turn, sprint-strafe, and post-skill handoff validation. |
| Combat mode / weapon kit / slots | `PARTIAL` | DB has `weapon_kit`, `weapon_combat_mode`, `weapon_combat_mode_skill_slot`; Unreal and server expose `mode_slots` and combat mode ACK/snapshot fields. | Current design says Vanguard is intentionally empty and Bulwark owns M1/R/F for now. Must prove no local fallback fills wrong mode. | Make DB-backed mode slot test: startup Bulwark slots, CTRL switch to empty Vanguard, CTRL back to Bulwark, no Q/R/F fallback in wrong mode. |
| HUD visual identity | `PARTIAL` | Unreal HUD docs and C++ HUD exist; skill icon prompt sheet exists. | Visual polish and frame art are not final; Unreal project has no git safety. | Finish HUD frame/token asset plan only after gameplay runtime is stable, or isolate it so HUD cannot affect movement. |
| Temporal melee hitboxes | `PARTIAL` | DB has temporal hitbox tables/seeds; server has `internal/hitbox` runtime and tests. | `internal/gameapi/impact.go` still has simplified profile containment logic; need confirm all player/wolf skills use temporal sweep over time, not static full arcs. | Make temporal hitbox canonical for M1-1/M1-2/M1-3/F/R and wolf bite/lunge/maul; remove or clearly isolate static arc compatibility. |
| Damage / defense / posture / stamina | `PARTIAL` | DB has skill damage/posture, combat core, defense contracts; server has `internal/gameapi/defense.go`, `impact.go`, tests. | `internal/damage`, `internal/defense`, `internal/stamina` packages are absent. Logic is concentrated in `gameapi`; `defaultBlockArcDeg` exists. | Split only when useful: create authoritative combat-resolution modules or packages so DB contracts own values and runtime owns flow. Remove default gameplay numbers from normal paths. |
| Player action runtime / ActionInstance | `PARTIAL` | `internal/combat/actionruntime` exists; combat system uses action instances/locks. | Server still has separate `gameapi` runtime for recovered vertical slice. Need ensure player and creature use the same action language. | Centralize action instance phase/lock/cooldown/queue semantics for player and creature. Avoid separate creature lock/cooldown maps as final architecture. |
| Creature brain package | `PARTIAL` | `internal/ai/creature_brain.go`, `skill_setup.go`, `tactics.go`, `memory.go`, and `region_brain_system.go` were rebuilt. `gameapi` now converts `WolfRuntimePolicy` and skill behavior bindings into `ai.Policy` and asks the regional brain system instead of owning lunge/maul/dodge selection inline. | Rich perception, stamina/resource budgets, repeat penalties, and creature action-runtime parity are still not complete. | Continue from this module, not from `gameapi` branches: add perception inputs, stamina/resource budget, setup memory, and tests before calling creature runtime fully restored. |
| Wolf behavior contracts | `PARTIAL` | DB seed `016_wolf_behavior_contract_seed.sql` includes lunge/maul/bite/dodge, behavior runtime contract, target opportunity, orbit, evasion, setup, bindings. Unreal placeholder has creature visual skill motion. | Server-side brain parity is missing/monolithic. Wolf floating/overlap issues suggest Unreal presentation and server ground/nav state still need proof. | Restore server creature brain first, then fix wolf ground plane/visual offset using authoritative locomotion data, not visual hacks. |
| Creature skill movement | `PARTIAL` | DB has movement action contracts for wolf skills and legacy compatibility for lunge. Unreal has predictive creature skill visual motion. | Current lunge/maul behavior may be using stale or simplified server state. Need prove landing inertia, airborne passthrough, hit timing, damage timing. | gRPC-check lunge/maul contracts, add server tests for movement envelope and damage timing, then Unreal visual validation. |
| World/map/session pipeline | `PARTIAL` | Unreal has world volumes/exporter and map docs; server docs describe map/runtime startup integration. | Multi-region/map future is not final; current focus is PlainTestMap vertical slice. | Do not expand world streaming until combat/movement recovery is stable. Keep contracts ready but avoid broad scope. |
| Automated rubberband validation | `PARTIAL` | Server movement tests exist; Unreal bridge has validation routines and docs mention scanner. | User-observed rubber can escape current automation. Client tests may not drive real continuous input/camera/skill sequences faithfully. | Build a smaller but faithful scenario suite: run + aggressive yaw, W+Shift+M1 chain, stationary M1/F/R, post-F/R sprint/strafe, leap hit/landing, dodge exit. Scanner must be strict, not softened. |
| Recovery fixtures/fallbacks | `PARTIAL` | `RecoveryFixtureRuntimeContracts()` is now explicitly named/commented as a dev/test reconstruction fixture. Latest ledger says it is not normal boot path. `gameapi` command execution no longer accepts dodge/leap/skill movement when their movement contracts are absent or invalid. | Unreal still has inert/default profile values that need explicit missing-profile visibility. | Keep fixture out of app boot. Next cleanup is Unreal/profile-side fallback visibility, not using fixture data as gameplay authority. |

## Immediate Priority Order

1. `P0`: Restore creature brain architecture in `server-apeiron/internal/ai`.
   - Status: `PARTIAL`.
   - Done: rebuilt local brain/tactic/setup/memory/region brain system module and wired wolf decision selection through it.
   - Remaining: stamina/resource budget, repeat penalties, richer perception, and creature action runtime parity.
2. `P0`: Re-audit player movement / skill movement authority.
   - Verify no duplicate position authority, no stale recovery handoff, no normal input fighting skill movement.
   - Protected baselines: leap, dodge, turn.
3. `P0`: Convert runtime fallback suspicion into hard proof.
   - Normal boot uses DB contracts only.
   - Recovery fixtures may exist only for tests/dev reconstruction and must be named as such.
4. `P1`: Complete temporal melee hitboxes across all current player and wolf skills.
   - M1-1, M1-2, M1-3, Shield Bash, Shield Rush, bite, lunge, maul.
5. `P1`: Make combat mode source of truth airtight.
   - Bulwark owns current M1/R/F.
   - Vanguard is empty until real Vanguard skills are implemented.
   - No local fallback slot injection.
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
- Movement command dedup/hash sync is either active and tested or removed as dead code.
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

#### C2. Damage Pipeline

Status: `PARTIAL`

Expected final architecture:

- One damage pipeline handles player and creature skills.
- Pipeline resolves: hit confirmation, dodge/iFrame, block, parry, posture, stamina, health, control/push, death.
- Damage and posture values come from skill/runtime contracts.
- Resistances are future-ready but not required to block current vertical slice.

Acceptance criteria:

- Creature can die.
- Blocked hits obey defense contract.
- Parry/block are not artificial side checks outside the pipeline.
- Damage events give Unreal enough data for feedback/VFX.

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
- Creature skill cooldowns are tracked by the runtime and passed to the brain as unavailable skills; the brain skips cooldown-blocked bindings instead of repeating them.
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

- Server readiness reports missing contract categories.
- Runtime logs show contract ids/hashes.
- No gameplay fallback path is reachable during normal boot.

#### E3. Legacy Compatibility Audit

Status: `SUSPICIOUS`

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
