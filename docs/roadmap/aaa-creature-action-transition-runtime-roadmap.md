# AAA Creature Action Transition Runtime Roadmap

Date: 2026-06-24

## Objective

Bring creature movement actions, starting with the wolf, to the same AAA transition standard now being restored for the player: action-owned movement must hand off into tactical creature movement through explicit transition states, not through a sudden switch back to orbit/flank/pursuit.

This is not a request to copy player code. Player and creature share the same combat language, but their runtime pressures differ:

- Player action transitions must handle local prediction, held input, sprint intent, command replay, ACKs and snapshots.
- Creature action transitions must handle brain/tactic handoff, setup policy, action runtime commitment, contact carry, interruption and snapshot presentation.

The shared concept is `ActionExitTransition`. The implementations must be profile/contract-driven and reusable.

## Current Decision

The wolf does need equivalent transition architecture, but not a player-input clone.

Required creature states:

- `CreatureDodgeActive -> CreatureDodgeExitTransition -> CreatureTacticalMove`
- `CreatureSkillActive -> CreatureSkillExitTransition -> CreatureTacticalMove`
- Future: `CreatureLeapAirborne -> CreatureLeapLandingTransition -> CreatureTacticalMove`

The wolf does not yet need a fully separate leap implementation unless the lunge becomes an actual airborne/leap contract. Current lunge/maul/rush-like creature actions still need action exit/carry transitions because they move the creature physically.

## Audit Pass 2026-06-24

This roadmap is no longer a blank design. Current recovered code and DB already contain several canonical pieces that must be consumed before adding new schema.

### Existing Canonical DB Pieces

Already present and should be treated as source-of-truth unless runtime audit proves otherwise:

- `movement_action_contract`
  - `wolf_dodge_lateral_leap_v1`
  - `low_fast_lunge_v1`
  - `wolf_maul_lateral_counter_v1`
  - `wolf_bite_melee_commit_v1`
- `skill_movement_action_binding`
  - `wolf_dodge -> wolf_dodge_lateral_leap_v1`
  - `lunge -> low_fast_lunge_v1`
  - `maul -> wolf_maul_lateral_counter_v1`
  - `bite -> wolf_bite_melee_commit_v1`
- `creature_behavior_runtime_contract`
  - `contract_wolf_pack_harasser_v1`
- `creature_skill_setup_policy`
  - `wolf_lunge_flank_windup_v1`
  - `wolf_lunge_chase_windup_v1`
  - `wolf_maul_pressure_counter_v1`
- `creature_skill_behavior_binding`
  - lunge/circle setup
  - lunge/chase setup
  - maul/pressure counter
  - wolf dodge pressure/evasion
- `skill_impact_control_effect`
  - `impact_wolf_maul_lateral_grab`
- temporal hitboxes
  - `motion_wolf_lunge_cross_v1`
  - `motion_wolf_maul_lateral_counter_v1`

Therefore the first implementation pass must not invent replacement rows. It must wire the existing canonical rows through runtime action transition ownership.

### Existing Server Evidence

Files already showing creature action runtime concepts:

- `internal/gameapi/action_runtime_lifecycle.go`
  - `completeCreatureActionRuntimeLocked` currently clears:
    - `actionInstance`
    - `actionMotion`
    - `creatureActiveSetupPolicyID`
    - `skillState`
    - `combatState`
  - This is the most suspicious snap point: completion appears to end action ownership immediately instead of entering a transition.
- `internal/gameapi/creature_action_runtime.go`
  - creature action execution/completion should be audited as the action owner.
- `internal/gameapi/runtime.go`
  - publishes locomotion/action fields and movement projection.
- `internal/gameapi/creature_policy_publication.go`
  - publishes brain movement/tactic decisions.
- `internal/ai/creature_brain.go`
  - active skill continuation currently drives setup/committed decision.
  - non-active skill path can return orbit/flank/chase immediately.
- `internal/ai/memory.go`
  - carries tactic/orbit/skill memory.

### Current Likely Weak Point

The most likely creature version of the player snap bug is:

```text
Creature committed action completes
  -> completeCreatureActionRuntimeLocked clears actionMotion/actionInstance
  -> brain immediately publishes orbit/flank/pursuit
  -> tactical movement becomes owner in the same or next tick
  -> client sees creature snap/float/rotate weirdly because action endpoint/carry was not released
```

The AAA fix is not to slow orbit or tune lunge blindly. The fix is to insert a creature action transition owner between action completion and tactical movement.

## Authority Matrix

| Domain | Owner | Must Not Own |
| --- | --- | --- |
| Creature skill selection | `internal/ai` brain + DB behavior/skill bindings | Movement resolver |
| Skill setup movement | `creature_skill_setup_policy` + brain decision | Normal orbit code |
| Committed action root | `movement_action_contract` + action runtime + movement resolver | Brain tactic movement |
| Action exit/carry | New/consolidated creature action transition runtime | Raw orbit/flank/pursuit |
| Contact carry/drag | impact/control profile + resolver/contact policy | Direct position set |
| Tactical reentry | Brain after transition complete | Action runtime after completion |
| Unreal creature display | Snapshot consumer/presentation | Physics authority |

## Conflict Matrix

| Conflict | Bad Outcome | Correct Resolution |
| --- | --- | --- |
| Brain tactic vs active action | Wolf starts orbiting during lunge/maul/dodge | Action runtime owns root until action phase and transition complete |
| Setup policy vs committed action | Lunge curves/windup fights jump/commit vector | Setup ends at commit; committed action owns root |
| Action completion vs tactical reentry | Snap at end of dodge/lunge/maul | Enter `CreatureActionExitTransition` before tactical move |
| Contact carry vs creature movement | Maul drags wrong or target desyncs | Contact carry has explicit owner and release policy |
| Lunge as leap vs grounded skill | Wrong gravity/landing behavior | Contract action type decides: `leap` uses landing transition; `grounded_skill` uses skill exit transition |
| DB fixture fallback vs seed | Behavior works in tests but not live | Live runtime must load DB canonical rows; fixture only for tests/recovery |
| Unreal smoothing vs server authority | Looks smooth but server snaps later | Unreal can smooth mesh only; root comes from authoritative snapshot |

## Contract Model: Reuse First, Extend Only If Needed

### Use Existing Fields First

The following existing fields may already be enough for first implementation:

- From `movement_action_contract`:
  - `action_type`
  - `duration_ms`
  - `active_ms`
  - `recovery_ms`
  - `distance_cm`
  - `base_speed_cm_s`
  - `phase_window_policy`
  - `prediction_error_policy`
  - `reconciliation_contract_id`
  - `allow_recovery_locomotion`
  - `root_motion_owner`
  - `contact_policy`
  - `speed_curve`
  - `vertical_curve`
  - `metadata`
- From `skill_movement_action_binding`:
  - `handoff_policy`
  - `normal_input_policy`
  - `target_policy`
  - `contact_policy`
- From `runtime_movement_reconciliation_profile`:
  - carry/handoff timing fields already used by player.

### Add New Schema Only If These Questions Fail

Add a generic transition profile only if existing contracts cannot answer:

- How long should creature action exit own root after action completion?
- Should brain tactic reentry be immediate, delayed, blended or blocked until carry release?
- Which curve controls exit speed decay?
- Can setup memory survive through the transition?
- Should contact carry extend the transition?
- What should happen if the creature is interrupted during transition?

If needed, prefer:

```sql
CREATE TABLE apeiron.action_transition_profile (
    id TEXT PRIMARY KEY,
    owner_kind TEXT NOT NULL, -- player | creature | shared
    action_type TEXT NOT NULL, -- dodge | leap | grounded_skill | impact_carry
    transition_kind TEXT NOT NULL, -- exit | landing | contact_release
    duration_ms INT NOT NULL,
    speed_curve JSONB NOT NULL DEFAULT '[]',
    direction_policy TEXT NOT NULL,
    next_owner_policy TEXT NOT NULL,
    tactical_reentry_policy TEXT NOT NULL,
    contact_release_policy TEXT NOT NULL DEFAULT 'none',
    interruption_policy TEXT NOT NULL DEFAULT 'preserve_until_control',
    metadata JSONB NOT NULL DEFAULT '{}'
);
```

Then bind it without duplicating movement contracts:

```sql
ALTER TABLE apeiron.skill_movement_action_binding
    ADD COLUMN action_transition_profile_id TEXT NULL REFERENCES apeiron.action_transition_profile(id);
```

But do not add this table if `movement_action_contract.metadata` + `handoff_policy` cleanly covers the first slice.

## Runtime State Model

### Required Server Runtime Struct

The implementation should converge on a reusable server-side state like:

```go
type CreatureActionTransition struct {
    CreatureID string
    SkillID string
    ActionContractID string
    Kind string // dodge_exit | skill_exit | leap_landing | contact_release
    StartedAt time.Time
    EndsAt time.Time
    Endpoint domainmath.Position
    ExitDirection domainmath.Vec3
    ExitSpeedCMPerSec float64
    CarryDurationMS int32
    MovementOwner string // creature_action_transition
    PreviousTactic string
    NextTacticPolicy string
    SetupPolicyID string
    ContactCarryActive bool
    Interrupted bool
}
```

This can be implemented as a field on `entityState` or a dedicated runtime map. It must not be stored only in AI memory because it is movement authority, not just behavior memory.

### Required Locomotion Publication

During transition, creature locomotion should publish:

- `Action`
- `AbilityKey`
- `ActionContractId`
- `MovementType`
- `MovementMode`
- `Phase`
- `ReconciliationMode`
- `TargetSpeed`
- `EffectiveSpeed`
- `PhaseSpeedScale`
- `ActionStartPosition`
- `ActionProjectedPosition`
- `ActionDistanceTraveled`
- `LandingHandoffActive` or equivalent transition active flag
- `LandingExitDirection` or equivalent exit direction
- `LandingExitSpeed` or equivalent exit speed
- debug metadata:
  - `movement_owner=creature_action_transition`
  - `previous_tactic`
  - `next_tactic_policy`

If the existing proto only has player-flavored names like landing handoff, use them only if semantically correct. Otherwise add creature-neutral fields rather than overloading misleading names.

## Implementation Invariants

1. `completeCreatureActionRuntimeLocked` must not directly drop a moving action into `ready` if the action moved root or has contact carry.
2. Any contract with `distance_cm > 0`, `action_type in ('dodge','leap','grounded_skill')`, or `contact_policy != 'none'` must evaluate whether it needs transition.
3. Brain may decide the next tactic during transition, but may not move the creature root until transition completion.
4. Setup policy can run before commit, not after committed action starts.
5. Maul contact carry must keep both actor and target under one movement/contact owner until release.
6. Lunge with `action_type='leap'` must use landing transition; if later converted to grounded lunge, use skill exit transition.
7. Bite can skip heavy transition only if `distance_cm=0` and no contact carry/drag/control movement applies.
8. Runtime must fail loudly if a moving creature skill has no movement action binding.
9. DB fixtures must not override live DB rows in normal runtime.
10. Player transition code must not be changed while implementing creature transition unless the shared abstraction is intentionally extracted.

## Non-Negotiable Rules

- Do not create wolf-only hardcoded branches when a generic creature action transition profile can express the behavior.
- Do not let AI orbit/flank/pursuit retake movement while a committed movement action is still releasing.
- Do not solve creature snap by globally slowing tactical movement.
- Do not copy player local prediction code into creatures.
- Do not add unit tests in this slice. Runtime behavior first; tests come after the user confirms the feel.
- Do not increase reconciliation deadzones to hide snap.
- Do not create DB fallback values in Go or Unreal.
- Do not leave two active movement bindings/policies selectable for the same wolf skill.

## Source Of Truth

The source of truth should be:

1. DB movement/action contracts for physical movement shape.
2. DB transition profiles for exit/carry/landing handoff behavior.
3. Creature behavior/setup/evasion policies for tactical decisions.
4. Server runtime as authority for creature position/action state.
5. Unreal as presentation/snapshot consumer only.

## Target Architecture

### Shared Runtime Concept

Create or consolidate a generic runtime concept:

```text
ActionMotion
  -> ActionExitTransition
      -> NextMovementOwner
```

`ActionExitTransition` should carry:

- `action_endpoint_position`
- `exit_direction`
- `exit_speed`
- `transition_duration_ms`
- `carry_curve_id` or speed curve samples
- `direction_blend_curve_id`
- `next_owner_policy`
- `interruption_policy`
- `contact_carry_policy`
- `tactical_reentry_policy`

### Player-Specific Layer

Player uses the shared transition but adds:

- local prediction
- held input intent
- sprint reentry
- pending command replay
- ACK/snapshot reconciliation

### Creature-Specific Layer

Creature uses the shared transition but adds:

- brain action lock/commitment state
- tactic reentry delay
- setup policy cleanup
- selected skill memory
- evasion budget/cooldown memory
- contact carry against player or other creatures
- interruption/control response

## DB Contract Work

Audit existing DB before adding schema. Reuse canonical fields if already present.

Expected DB areas:

- `movement_action_contract`
- `skill_movement_effect`
- `skill_movement_action_binding`
- `creature_behavior_runtime_contract`
- `creature_skill_setup_policy`
- `creature_evasion_policy`
- `combat_defense_contract`
- `impact_response_profile`
- `runtime_movement_reconciliation_profile`

If missing after audit, add a generic transition profile table or extend the correct existing table. Do not create player-only fields for creature behavior.

Candidate profiles:

- `creature_dodge_exit_transition_v1`
- `creature_grounded_skill_exit_transition_v1`
- `creature_airborne_landing_transition_v1`
- `creature_contact_carry_transition_v1`

Wolf bindings should reference these profiles through skill/action bindings, not code branches.

First preferred path:

- Use existing `skill_movement_action_binding.handoff_policy`.
- Use existing `movement_action_contract.contact_policy`.
- Use existing `movement_action_contract.metadata` only for non-schema notes that do not affect critical runtime.
- Add schema only when a runtime decision needs a typed value.

Do not bury critical values like transition duration or tactical reentry policy inside freeform JSON if the server must branch on them in normal runtime.

## Server Work

### AI/Brain

Files to inspect:

- `internal/ai/creature_brain.go`
- `internal/ai/skill_setup.go`
- `internal/ai/tactics.go`
- `internal/ai/memory.go`
- `internal/ai/region_brain_system.go`
- `internal/app/creature_attack_profile_provider.go`

Required behavior:

- Brain cannot switch to orbit/flank/pursuit until action exit transition is complete.
- Setup movement must end when committed action starts.
- Tactical movement reentry must be explicit.
- Orbit side and tactic memory should survive the action unless the transition policy says reset.

### Combat/Action Runtime

Files to inspect:

- `internal/combat`
- `internal/movement/resolver.go`
- `internal/gameapi/runtime.go`
- creature action instance/runtime files if present.

Required behavior:

- Creature movement action uses the authoritative resolver path.
- No direct position assignment for dodge/lunge/maul except spawn/teleport.
- `ActionProjectedPosition`, `ActionDistanceTraveled`, `ActionStartPosition` and endpoint fields remain coherent.
- Transition publishes locomotion state until carry/release finishes.
- Impact/collision can interrupt or modify transition only through control/impact policy.

Specific required change:

- Replace direct completion in `completeCreatureActionRuntimeLocked` with:
  1. determine whether the completed action needs transition;
  2. if no transition, complete as today;
  3. if transition needed, clear setup ownership but keep movement ownership in `CreatureActionTransition`;
  4. publish transition locomotion;
  5. only after transition complete, clear action runtime and return combat state to `ready`.

This does not mean the action can deal damage after completion. Damage schedule and movement transition are separate. A skill can stop dealing damage while still carrying movement inertia/reentry.

### Snapshot/Presentation

Snapshots should expose enough state for Unreal to present without guessing:

- creature action
- phase
- movement mode
- transition active
- transition duration
- exit direction
- exit speed
- projected endpoint
- tactical state label for debug placeholder

## Unreal Work

Unreal should not decide creature physics.

It should:

- consume creature snapshot action/transition state;
- draw placeholder/debug state (`circle`, `flank`, `pursuit`, `setup`, `lunge`, `maul`, `dodge_exit`, etc.);
- smooth visual presentation without moving server authority;
- avoid hiding creature snap with camera or mesh hacks.

No player dodge/leap code should be reused directly for creature prediction.

## Wolf Skill Requirements

### Wolf Dodge

Needs:

- `CreatureDodgeActive`
- `CreatureDodgeExitTransition`
- invulnerability window if configured by evasion/defense policy
- stamina/evasion budget consumption if enabled
- tactical reentry after transition

Goal:

Wolf dodge should feel like a fast low hop/backstep/side-step, then naturally resume behavior. No instant snap back to orbit.

Expected contract:

- `skill_id=wolf_dodge`
- `movement_action_contract_id=wolf_dodge_lateral_leap_v1`
- `action_type=dodge`
- `contact_policy=iframe`
- transition kind: `creature_dodge_exit`
- next owner: `creature_tactical_move`
- tactical reentry: preserve previous orbit side unless policy says reset.

### Wolf Lunge

Current note: full wolf leap is not implemented yet.

Needed now:

- If lunge is still grounded/skill movement: use `CreatureSkillExitTransition`.
- If lunge becomes airborne: add `CreatureLeapAirborne` and `CreatureLeapLandingTransition`.
- Setup policy must own windup/run/orbit setup before commit.
- Tactical orbit/pursuit must not fight committed lunge.

Goal:

Wolf runs/circles during setup, commits to lunge, crosses/lands naturally, then exits through transition before brain resumes.

Expected contract:

- `skill_id=lunge`
- `movement_action_contract_id=low_fast_lunge_v1`
- current `action_type=leap`
- `contact_policy=airborne_passthrough`
- setup policies:
  - `wolf_lunge_flank_windup_v1`
  - `wolf_lunge_chase_windup_v1`
- transition kind if kept as leap: `creature_leap_landing`
- transition kind if later made grounded: `creature_skill_exit`

Important:

- Since DB currently declares `low_fast_lunge_v1` as `action_type='leap'`, the implementation must either honor it as airborne/leap or deliberately change DB to grounded. Do not keep DB saying leap while runtime treats it as generic grounded movement.

### Wolf Maul

Needs:

- `CreatureSkillExitTransition`
- contact carry policy for dragging/staggering player
- side selection policy
- interruption policy
- block/defense interaction

Goal:

Maul movement must be a committed action with contact carry, not tactical orbit disguised as an attack.

Expected contract:

- `skill_id=maul`
- `movement_action_contract_id=wolf_maul_lateral_counter_v1`
- `action_type=grounded_skill`
- `contact_policy=lateral_counter_contact`
- impact/control:
  - `impact_wolf_maul_lateral_grab`
- setup policy:
  - `wolf_maul_pressure_counter_v1`
- transition kind: `creature_skill_exit` plus `contact_release` when target was carried.

Important:

- Contact carry release must be explicit. If target is dragged/staggered, the release frame must not be a raw return to normal movement.

### Bite

Likely does not need heavy movement transition unless it contains displacement.

Needed:

- short recovery transition only if bite movement or hit reaction causes snap;
- otherwise tactical reentry can be direct after recovery.

## Implementation Slices

### Slice 1 - Audit And Ownership Map

Status: ready.

Tasks:

- Map current creature action movement path from brain to combat to movement resolver to snapshot.
- Identify every place where creature position can change.
- Identify whether creature normal movement can run during active skill movement.
- Identify whether current wolf dodge/lunge/maul use DB contracts or code fallback.
- Identify whether Unreal only presents creature movement or also corrects it.

Output:

- Update this doc with actual files and authority owners.

Focused grep targets:

- `completeCreatureActionRuntimeLocked`
- `creature.actionMotion`
- `creature.actionInstance`
- `creatureActiveSetupPolicyID`
- `MovementTactic`
- `ActionProjectedPosition`
- `ActionDistanceTraveled`
- `publishCreature`
- `skill_movement_action_binding`
- `impact_wolf_maul_lateral_grab`

Slice 1 must produce a small table:

| Skill | Contract | Action Type | Setup Policy | Contact Policy | Current Completion | Needs Transition |
| --- | --- | --- | --- | --- | --- | --- |
| wolf_dodge | wolf_dodge_lateral_leap_v1 | dodge | none/evasion | iframe | audit | yes |
| lunge | low_fast_lunge_v1 | leap | flank/chase windup | airborne_passthrough | audit | yes |
| maul | wolf_maul_lateral_counter_v1 | grounded_skill | pressure counter | lateral_counter_contact | audit | yes |
| bite | wolf_bite_melee_commit_v1 | grounded_skill | none | melee_contact | audit | maybe |

### Slice 2 - Generic Creature Action Transition Runtime

Status: pending.

Tasks:

- Add server runtime concept for creature action transition.
- Make transition state explicit and independent from player input transition.
- Keep action endpoint, exit direction, exit speed, carry duration and next movement owner.
- Prevent brain/tactic movement from overwriting transition root until completion.

Done when:

- A creature action can finish into transition and only then return to tactical movement.

Implementation shape:

```text
startCreatureActionRuntime
  -> action owns root
  -> action phase complete
  -> beginCreatureActionTransitionLocked
  -> transition owns root/carry/release
  -> completeCreatureActionTransitionLocked
  -> brain/tactical movement resumes
```

Do not mark creature combat state `ready` until transition completion unless the action had no movement and no carry.

### Slice 3 - DB/Profile Binding

Status: implemented for current wolf skills; transition-profile schema deferred until runtime proves necessary.

Tasks:

- Add/reuse DB profile fields for creature transition durations and carry curves.
- Bind wolf dodge/lunge/maul to transition profiles.
- Remove/disable stale duplicate movement effects or setup bindings.
- Ensure lunge range/setup/movement contract envelope is coherent.

Done when:

- Runtime does not invent creature transition values in Go or Unreal.

Implementation note 2026-06-24:

- Normal server runtime already consumes `skill_movement_action_binding + movement_action_contract`.
- Compatibility API `GetSkillMovementEffect` now has canonical rows for:
  - `lunge -> low_fast_lunge_effect_v1`
  - `wolf_dodge -> wolf_dodge_lateral_leap_effect_v1`
  - `maul -> wolf_maul_lateral_counter_effect_v1`
- These compatibility rows are not runtime authority; they point back to the canonical movement action contracts in metadata.
- No `action_transition_profile` table was added because the current contracts provide typed duration/recovery/contact policy for this slice.

Expected first binding approach:

- `wolf_dodge`: use `explicit_recovery_handoff` and action type `dodge`.
- `lunge`: use `grounded_handoff` because contract is currently `leap`.
- `maul`: use `explicit_recovery_handoff` plus contact policy `lateral_counter_contact`.

If these policies are too vague for runtime, promote them to typed transition profiles in DB rather than adding string switches in Go.

### Slice 4 - Wolf Dodge

Status: server runtime implemented; PIE validation pending.

Tasks:

- Apply `CreatureDodgeExitTransition`.
- Preserve invulnerability/iframe rules through configured dodge duration.
- Ensure evasion budget/cooldown/stamina rules remain data-driven.
- Return to previous tactic through explicit reentry.

Done when:

- Wolf dodge no longer snaps or resumes orbit abruptly.

Implementation note 2026-06-24:

- `skill_root` action motions with action type `dodge` now report combat pipeline state `dodge`, so creature dodge receives the same iframe/readability semantics as the player during the configured movement window.
- Dodge exit now flows through `CreatureActionTransition` before tactical reentry.

### Slice 5 - Wolf Lunge

Status: server runtime implemented; PIE validation pending.

Tasks:

- Keep current lunge as grounded skill movement or convert to airborne deliberately, not accidentally.
- If grounded: use skill exit transition.
- If airborne later: add leap landing transition.
- Preserve setup flank/run behavior from skill setup policy.
- Ensure hit/contact happens when crossing the player, not from stale target position.

Done when:

- Lunge movement, hit timing, landing/reentry and tactical continuation are readable and natural.

Implementation note 2026-06-24:

- `low_fast_lunge_v1` remains DB-authoritative as `action_type='leap'`.
- Creature `skill_root` now enables vertical root when the movement action contract declares leap/airborne/vertical curve data.
- Lunge exits through `creature_leap_landing_transition`, then tactical movement resumes.

### Slice 6 - Wolf Maul

Status: server runtime implemented for source action/contact classification; PIE validation pending.

Tasks:

- Restore/implement maul contact carry.
- Add action exit transition after carry release.
- Apply block/interruption policy.
- Prevent brain movement from fighting maul movement.

Done when:

- Maul moves, carries/releases naturally, and does not snap into orbit.

Implementation note 2026-06-24:

- `lateral_counter_contact` is now classified as target-carry contact, not source-stop contact.
- This prevents the wolf maul root action from stopping at the first contact point; the target control effect remains DB-driven through `impact_wolf_maul_lateral_grab`.
- Maul exits through `CreatureActionTransition` before orbit/flank/pursuit can retake root.

### Slice 7 - Unreal Creature Debug/Presentation

Status: server-side state published; Unreal code already consumes `CreatureAIState` debug labels; PIE validation pending.

Tasks:

- Restore debug placeholder labels for tactic/action/transition.
- Show `circle`, `flank`, `pursuit`, `setup`, `dodge_exit`, `lunge`, `maul`, `transition`.
- Ensure visual smoothing does not move authority.

Done when:

- Runtime behavior can be visually diagnosed in PIE without reading server internals.

Implementation note 2026-06-24:

- Server now publishes transition as `MovementTactic` and `PathState=creature_action_transition`.
- Existing `ApeironCreaturePlaceholder` debug label already prints `MovementTactic`, `CombatTactic`, `Commitment`, `SelectedSkillId`, range and path state.
- No Unreal C++ change was needed in this pass.

## Runtime Validation First

Do not implement unit tests in this slice.

Validation order:

1. Build server and DB if touched.
2. Restart DB/server.
3. Run PIE/manual validation.
4. Observe logs for action/transition/tactic ownership.
5. Only after user confirms runtime feel, add focused unit/integration tests.

Runtime scenarios:

- wolf idle/circle/flank without skill;
- wolf dodge while pressured;
- wolf dodge then reenters circle/flank;
- wolf lunge setup then commit;
- wolf lunge exit/reentry;
- wolf maul movement/contact/release;
- player shield rush hits wolf during creature action;
- creature does not float, snap, teleport or instantly retake orbit.

No unit tests in the implementation slice unless explicitly requested later. Existing broken unit tests must not redirect this work away from runtime behavior.

## Hypothesis Matrix For First Implementation

Before coding, evaluate these hypotheses together:

| Hypothesis | Evidence To Check | Fix If True | Conflicts |
| --- | --- | --- | --- |
| Action completion clears motion too early | `completeCreatureActionRuntimeLocked` clears `actionMotion` at phase complete | Insert `CreatureActionTransition` before clear | Must not keep damage active |
| Brain resumes tactic too early | decision publishes orbit/flank same tick as action complete | Gate tactical movement while transition active | Brain can still decide next tactic as intent |
| DB contract exists but runtime ignores it | live logs show fixture/fallback or empty action contract | Load/require DB movement binding | May expose missing seed rows |
| Lunge DB says leap but runtime treats as grounded | locomotion action type mismatches contract | Honor leap or change DB deliberately | Do not half-convert |
| Maul contact carry lacks release transition | player/wolf snaps after carry | Add contact release owner | Must separate damage end from carry release |
| Unreal lacks creature transition display | server publishes transition but placeholder jumps/labels wrong | Add presentation/debug only | Unreal still must not own physics |

Common-denominator fix:

The server must publish one movement owner at a time for creatures:

```text
setup_tactic -> committed_action -> action_transition -> tactical_move
```

No two adjacent stages can write creature root in the same tick.

## Logs

Add feature flags when implementing:

- `APEIRON_CREATURE_ACTION_TRANSITION_DEBUG`
- `APEIRON_CREATURE_DODGE_DEBUG`
- `APEIRON_CREATURE_LUNGE_DEBUG`
- `APEIRON_CREATURE_MAUL_DEBUG`

Logs should include:

- `transition_begin`
- `transition_tick`
- `transition_complete`
- `creature_id`
- `skill_id`
- `brain_tactic_before`
- `brain_tactic_after`
- `movement_owner`
- `endpoint`
- `exit_dir`
- `exit_speed`
- `carry_duration`
- `contact_carry_active`
- `interrupted`
- `next_tactic`

## Done Criteria

This roadmap is done when:

- Wolf dodge, lunge and maul do not snap at action end.
- Creature tactical movement never competes with active movement action or transition.
- Creature transition values are DB/profile-driven.
- No wolf-only hardcoded movement branch remains where a generic profile can express the behavior.
- Unreal presentation/debug clearly shows action/tactic/transition state.
- Player movement/dodge/leap remains untouched and unchanged by creature work.
- Runtime validation confirms the wolf feels physical, readable and hard in a learnable way.

## Implementation Pass 2026-06-24

Status: server runtime transition owner implemented and extended through wolf dodge/lunge/maul semantics; PIE validation still required.

Implemented:

- Added `CreatureActionTransition` server state in `internal/gameapi/creature_action_transition.go`.
- `skill_root` creature motion now enters an explicit action transition before tactical movement can own root again.
- Transition duration is sourced from existing canonical contracts first:
  - `movement_action_contract.recovery_ms` for current creature skill movement.
  - existing runtime movement handoff profile only as a profile-driven fallback when the contract has no recovery.
- Transition kind is generic by action type:
  - `creature_dodge_exit_transition`
  - `creature_leap_landing_transition`
  - `creature_skill_exit_transition`
- During transition, locomotion publishes:
  - skill action/ability;
  - `grounded_handoff`;
  - `exit_handoff` or `landing_handoff`;
  - action contract id/hash through the existing locomotion contract payload;
  - transition endpoint, projected position, carried direction and carried speed.
- Brain/tactic movement is gated while transition is active:
  - `applyCreatureDecisionMotion` will not write creature root during transition.
  - `publishWolfLocomotionLocked` preserves transition locomotion during transition.
  - `applyCreatureActionRuntimeLocked` treats transition as the active action owner instead of clearing runtime.
- `completeCreatureActionRuntimeLocked` no longer drops a moving creature action directly to `ready` while a transition is alive.
- Wolf AI debug/presentation now exposes transition state as `movement_tactic`/`path_state=creature_action_transition`.
- Added optional server debug flag:
  - `APEIRON_CREATURE_ACTION_TRANSITION_DEBUG=1`
- Wolf dodge iframe now comes from the same combat-pipeline dodge state detection used for player dodge, generalized to `skill_root`.
- Wolf lunge now honors DB `action_type='leap'` by enabling vertical root when the movement action contract carries airborne/vertical data.
- Wolf maul `lateral_counter_contact` no longer stops the source action at contact; it is classified as carry contact so the DB impact control effect can move the target while the wolf action continues.

Architecture decision:

- No DB schema was added in this pass. The current DB already has canonical action contracts and recovery windows for wolf dodge/lunge/maul/bite. A new `action_transition_profile` table should only be added if runtime validation proves that recovery/handoff policy cannot express the needed exit behavior per action.

Validation:

- `go build ./cmd/game-server` passed.

Manual runtime validation still required:

- wolf dodge under pressure exits into tactic without snap;
- lunge commit/landing does not instantly retake orbit;
- maul release does not snap into orbit;
- wolf does not float or climb Z across repeated lunge/transition cycles;
- placeholder/debug shows transition state before returning to circle/flank/pursuit.

## Implementation Pass 2026-06-24 - Lunge Contract Retune After Vertical Root Restore

Status: DB/policy tuning implemented; PIE validation required.

Implemented:

- Kept `low_fast_lunge_v1` as an authoritative DB `leap` contract.
- Retuned its vertical profile from a tall leap into a low raking arc:
  - peak `44cm` instead of `120cm`;
  - apex earlier in the motion;
  - lower jump velocity metadata;
  - shorter contract recovery.
- Reduced post-landing inertia metadata from `1.1/1.2` to `0.7` and mirrored it in the compatibility
  `skill_movement_effect` surface.
- Raised lunge cooldown to `9000ms`.
- Reduced lunge skill-slot and binding weights so the wolf does not answer every opening with lunge.
- Aligned dev/reconstruction fixtures with the same low arc so recovery paths do not preserve the old
  exaggerated vertical motion.

Expected runtime result:

- Windup still circles/curves by setup policy.
- Commit becomes fast and low, not a high player-sized hop.
- Landing inertia remains readable but shorter.
- Wolf should mix bite/maul/dodge more often instead of lunge chaining.

Validation still needed in PIE:

- Repeated wolf lunge should not climb Z.
- Lunge should cross/pressure the player at a low height.
- Bite/maul should appear in rotation after cooldown/weight change.
- Creature debug label should still expose tactic/setup/action/transition state.

## Implementation Pass 2026-06-24 - Long Low Lunge And Grounded Transition Clamp

Status: implemented; PIE validation required.

Implemented:

- Retuned lunge into a longer, much lower raking arc:
  - `distance_cm=1652`;
  - `vertical_curve` peak `18cm`;
  - `base_speed_cm_s=1920`;
  - metadata apex/vertical velocity lowered.
- Aligned skill max range, setup policy ranges, behavior bindings, opportunity policy, compatibility movement effect, and dev fixtures with that longer envelope.
- Added generic entity ground-root tracking and creature leap transition clamping so airborne endpoint Z cannot leak into post-lunge tactical movement.

Validation target:

- Wolf should not remain a finger above the ground after lunge/contact/player dodge.
- Lunge should read as a low long pounce, not a high hop.
- If it still appears high, inspect Unreal creature mesh/capsule visual offset separately from server root Z.
