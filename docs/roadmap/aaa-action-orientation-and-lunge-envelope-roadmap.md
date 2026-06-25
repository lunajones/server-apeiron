# AAA Action Orientation And Lunge Envelope Roadmap

Date: 2026-06-24

## Objective

Make Apeiron able to express natural action movement with separate tactical facing, body/root facing, head/focus facing, attack direction, pre-commit movement, airborne movement and landing inertia.

Immediate target: wolf lunge.

General target: the same model must support player and creature actions without hardcoded one-off branches.

## Current Answer

Today the project is only partially ready.

The current base can already express:

- movement action duration;
- airborne duration;
- recovery/inertia duration;
- horizontal distance;
- speed curve;
- vertical curve;
- setup policy before the committed action;
- target-facing hitbox direction.

The current base does not yet cleanly express all of this as first-class runtime authority:

- body/root yaw separate from head/focus yaw;
- attack yaw separate from current body yaw;
- flank/circle body following the ring while head/focus tracks the player;
- a named post-flank `pre_lunge_commit` phase;
- a 100ms curved alignment/run phase before takeoff;
- lunge takeoff direction snapped from the commit phase, not recalculated later;
- per-action orientation policy shared by server and Unreal;
- player and creature parity for action orientation.

So the lunge contract is close, but not complete enough to be called AAA for the behavior requested. The correct fix is not another lunge height tweak. The correct fix is to add an action orientation/envelope model and then tune the lunge through that model.

## Desired Wolf Lunge Shape

The wolf lunge should be a full action envelope:

```text
Tactical flank/circle
  -> pre_lunge_commit alignment/run
  -> low airborne lunge
  -> post-landing inertia
  -> tactical reentry
```

Requested tuning target:

- `pre_lunge_commit_ms`: at least `100ms`.
- `airborne_duration_ms`: `520ms`.
- `landing_inertia_ms`: `200ms`.
- max height: low, raking, not a high parabola.
- setup: windup flank/circle still happens before commit.
- commit: after flank, wolf curves/alines body toward target, runs briefly in a straight/curved line, then jumps.
- landing: inertia continues after touch-down, but does not dominate the lunge distance.

Important distinction:

- `flank/circle` is tactical setup.
- `pre_lunge_commit` is action-owned preparation.
- `airborne` is lunge-owned root movement.
- `landing_inertia` is action transition, not tactical movement.

## Orientation Model

AAA behavior needs four orientation concepts.

| Concept | Meaning | Server Authority? | Unreal Presentation? |
| --- | --- | --- | --- |
| `movement_direction` | Direction the root/capsule is moving | yes | yes |
| `body_yaw` / `root_yaw` | Direction the body/capsule is committed toward | yes | yes |
| `focus_yaw` | Direction attention/head/chest wants to look | yes, as intent | yes |
| `attack_yaw` | Direction the action/hitbox commits to | yes | yes |

For the wolf:

- during flank/circle, `movement_direction` follows the ring;
- during flank/circle, `focus_yaw` points to the player;
- during flank/circle, `body_yaw` can be partly side-on and follow motion;
- during `pre_lunge_commit`, `body_yaw` blends toward `attack_yaw`;
- at takeoff, `attack_yaw` is latched from the target/commit snapshot;
- during airborne lunge, `attack_yaw` should not jitter with target movement unless the contract explicitly allows steering;
- after landing inertia, `body_yaw` should turn naturally into the next tactic instead of instantly rotating 180 degrees.

The server does not need skeletal head bone transforms. It does need to publish the authoritative intent:

- body/root yaw;
- focus yaw;
- attack yaw;
- orientation phase;
- turn-rate limits;
- whether attack direction is latched or tracking.

Unreal should use that intent for visual body rotation, future head/neck IK, debug labels and hitbox presentation. Unreal must not invent gameplay-facing attack direction.

## Proposed DB Contract Additions

Do not hardcode this in Go or Unreal. Add reusable policy data.

### `action_orientation_policy`

Purpose: define how an action separates movement direction, body yaw, focus yaw and attack yaw.

Candidate schema:

```sql
CREATE TABLE apeiron.action_orientation_policy (
    id TEXT PRIMARY KEY,
    owner_kind TEXT NOT NULL DEFAULT 'shared',
    body_yaw_source TEXT NOT NULL,
    focus_yaw_source TEXT NOT NULL,
    attack_yaw_source TEXT NOT NULL,
    body_turn_rate_deg_s DOUBLE PRECISION NOT NULL,
    focus_turn_rate_deg_s DOUBLE PRECISION NOT NULL,
    attack_turn_rate_deg_s DOUBLE PRECISION NOT NULL,
    commit_align_ms INT NOT NULL DEFAULT 0,
    attack_yaw_latch_policy TEXT NOT NULL,
    allow_head_look_while_strafing BOOLEAN NOT NULL DEFAULT TRUE,
    allow_body_side_on_movement BOOLEAN NOT NULL DEFAULT TRUE,
    metadata JSONB NOT NULL DEFAULT '{}'
);
```

Example lunge policy:

```text
id: orientation_lunge_flank_commit_v1
body_yaw_source: movement_direction_until_commit
focus_yaw_source: target
attack_yaw_source: commit_target_snapshot
body_turn_rate_deg_s: 420
focus_turn_rate_deg_s: 900
attack_turn_rate_deg_s: 720
commit_align_ms: 100
attack_yaw_latch_policy: latch_at_takeoff
allow_head_look_while_strafing: true
allow_body_side_on_movement: true
```

### `action_envelope_policy`

Purpose: define the phases around a movement action without mixing them into tactical AI or raw recovery.

Candidate schema:

```sql
CREATE TABLE apeiron.action_envelope_policy (
    id TEXT PRIMARY KEY,
    pre_commit_ms INT NOT NULL DEFAULT 0,
    airborne_ms INT NOT NULL DEFAULT 0,
    landing_inertia_ms INT NOT NULL DEFAULT 0,
    pre_commit_direction_policy TEXT NOT NULL DEFAULT 'none',
    airborne_direction_policy TEXT NOT NULL DEFAULT 'action_attack_yaw',
    inertia_direction_policy TEXT NOT NULL DEFAULT 'exit_direction',
    tactical_reentry_policy TEXT NOT NULL DEFAULT 'after_inertia',
    speed_curve JSONB NOT NULL DEFAULT '[]',
    vertical_curve JSONB NOT NULL DEFAULT '[]',
    metadata JSONB NOT NULL DEFAULT '{}'
);
```

Example lunge envelope:

```text
id: envelope_lunge_low_raking_100_520_200_v1
pre_commit_ms: 100
airborne_ms: 520
landing_inertia_ms: 200
pre_commit_direction_policy: curve_from_setup_to_attack_yaw
airborne_direction_policy: latched_attack_yaw
inertia_direction_policy: preserve_landing_exit
tactical_reentry_policy: blend_body_yaw_to_next_tactic
```

### Binding

Bind these policies to skills/actions, not to hardcoded wolf branches:

```text
lunge -> low_fast_lunge_v1
lunge -> orientation_lunge_flank_commit_v1
lunge -> envelope_lunge_low_raking_100_520_200_v1
```

Use the existing movement action contract for physical distance/speed/vertical samples, but do not overload it with every behavior/AI/orientation concern if the schema becomes unclear.

## Contract Evolution Catalog

This section is the broader AAA contract map. The goal is that future gameplay tuning should usually be:

```text
adjust DB contract/policy -> restart DB/server -> validate in PIE
```

not:

```text
add Go/C++ branch -> rebuild -> risk breaking another movement/action path
```

The list below is intentionally broader than wolf lunge. It covers the contract families Apeiron will need for combat, movement, creature behavior, player skills, PvP readability and future equipment/build systems.

### 1. Movement Action Contract

Purpose: physical root motion shape for an action.

Already exists in some form and should remain canonical.

Needs to be complete enough for:

- dodge;
- leap;
- shield rush;
- shield bash;
- basic attack movement;
- wolf lunge;
- wolf dodge;
- wolf maul;
- future heavy attacks;
- future fatality/execution skills.

Required fields:

- action type;
- duration;
- startup/pre-commit;
- active/airborne;
- recovery/inertia;
- distance;
- base speed;
- speed curve;
- vertical curve or ballistic model;
- root owner;
- contact policy;
- prediction/reconciliation profile id;
- exit/landing handoff policy id.

AAA rule:

- movement action contract owns physical movement shape;
- it must not own AI tactical decisions, hitbox geometry, resource cost or animation-only VFX.

### 2. Action Envelope Policy

Purpose: define high-level phase envelope around an action.

This is needed when `windup/active/recovery` is not enough.

Examples:

- wolf lunge:
  - tactical flank;
  - pre-lunge commit run;
  - airborne;
  - landing inertia;
  - tactical reentry.
- player dodge:
  - burst;
  - exit carry;
  - grounded reentry.
- player leap:
  - takeoff;
  - airborne;
  - landing transition;
  - grounded reentry.
- heavy attack:
  - windup tracking;
  - attack yaw latch;
  - active frames;
  - recovery with limited turn.

Required fields:

- `pre_commit_ms`;
- `airborne_ms`;
- `landing_inertia_ms`;
- phase names;
- phase ownership;
- phase direction policy;
- phase input policy;
- phase cancel policy;
- phase transition curve ids.

AAA rule:

- action envelope owns phase sequencing;
- movement action contract owns root path inside each phase.

### 3. Action Orientation Policy

Purpose: separate body/root direction, focus/head direction, attack direction and movement direction.

Required for:

- wolf circling side-on while looking at player;
- lunge pre-commit alignment;
- Souls-like player attacks with limited turning;
- Shield Rush direction lock;
- heavy attacks with commitment;
- future head/neck aim/IK.

Required fields:

- `body_yaw_source`;
- `focus_yaw_source`;
- `attack_yaw_source`;
- turn rates per yaw type;
- commit alignment window;
- attack yaw latch policy;
- tracking policy during windup;
- tracking policy during active frames;
- reentry/body realignment policy.

AAA rule:

- server owns gameplay direction intent;
- Unreal presents head/body/IK from that intent;
- hitboxes use `attack_yaw`, not accidental body/camera yaw.

### 4. Action Exit Transition Profile

Purpose: prevent snap/rubber after dodge, leap, lunge, rush, maul, bash or any action with movement.

This is the formal version of the lesson learned from dodge/leap.

Required fields:

- transition kind;
- duration;
- endpoint source;
- exit direction source;
- exit speed;
- speed decay curve;
- sprint/normal movement reentry policy;
- tactical reentry policy for creatures;
- replay/held-input policy for players;
- correction/reconciliation profile id.

Examples:

- `player_dodge_exit_transition_v1`;
- `player_leap_landing_transition_v1`;
- `creature_lunge_landing_transition_v1`;
- `creature_maul_contact_release_transition_v1`;
- `shield_rush_contact_release_transition_v1`.

AAA rule:

- action exit transition is a movement owner;
- normal movement/tactical movement resumes only after explicit handoff.

### 5. Reconciliation Profile

Purpose: define how client prediction and server authority resolve differences per movement family.

Needed families:

- grounded movement;
- sprint;
- strafe/lateral run;
- backward movement;
- turn/yaw;
- dodge exit;
- leap airborne;
- leap landing;
- grounded skill movement;
- contact rush/push;
- creature action transition;
- creature tactical movement.

Required fields:

- base correction tolerance;
- max correction per tick;
- smooth correction duration;
- hard snap threshold;
- pending command defer distance;
- action handoff tolerance;
- timeline buffer policy;
- replay ownership policy.

AAA rule:

- reconciliation profile should describe network correction, not gameplay feel;
- never use bigger deadzones to hide a broken movement owner.

### 6. Locomotion Profile

Purpose: normal movement feel by actor/combat state/equipment.

Needed for:

- player walk/run/strafe/backward;
- lateral sprint at 75%;
- backward run/walk at 50%;
- combat mode movement;
- future armor weight classes;
- creature walk/run/orbit/retreat.

Required fields:

- walk speed;
- run speed;
- strafe speed multiplier;
- backward speed multiplier;
- acceleration;
- deceleration;
- turn rate;
- braking friction/friction analog;
- stamina drain profile id;
- movement mode restrictions.

AAA rule:

- normal locomotion profile owns base locomotion;
- skill/action movement must not modify this profile to solve skill bugs.

### 7. Resource Cost And Regeneration Contract

Purpose: stamina/posture/mana-like resource cost and regen rules.

Needed for:

- sprint stamina drain;
- dodge stamina cost;
- block hold stamina drain;
- block hit stamina damage;
- exhausted stamina lockout;
- creature stamina/evasion budget;
- future armor/equipment modifiers.

Required fields:

- cost type;
- flat cost;
- continuous cost per second;
- regen delay;
- exhausted regen penalty;
- exhausted unlock threshold;
- allowed actions while exhausted;
- cost modifiers by equipment/build.

AAA rule:

- resources are gameplay contracts, not scattered checks;
- no action should silently ignore cost because a fallback path missed the resource gate.

### 8. Defense Window Contract

Purpose: express iframe, block, parry, hyperarmor, interruptibility and stagger windows.

Needed for:

- dodge iframe full duration;
- wolf dodge iframe;
- shield block active hold;
- parry timing;
- Shield Rush interrupt;
- Shield Bash stun;
- Maul blockable grab;
- lunge pass-through or interruption behavior.

Required fields:

- window start/end;
- defense type;
- facing requirement;
- block angle;
- posture/stamina damage;
- interruptibility;
- hyperarmor or poise;
- damage reduction;
- control immunity flags.

AAA rule:

- defense windows must align with action phases and hitbox active frames;
- damage pipeline resolves defense; movement code must not special-case damage immunity.

### 9. Temporal Hit Volume Contract

Purpose: represent melee damage as time-progressing volumes, not instant static areas.

Already started and should become universal.

Needed for:

- basic attack 1/2/3;
- Shield Bash;
- Shield Rush;
- bite;
- lunge;
- maul;
- future heavy attack;
- future fatality/execution setup.

Required fields:

- motion profile id;
- damage group id;
- sample timeline;
- shape per sample;
- orientation source;
- attachment/source socket or root;
- multi-hit policy;
- target dedupe policy;
- block/parry contact point policy;
- debug visualization policy.

AAA rule:

- hit volume follows the attack in time;
- same target takes damage according to damage group policy, not every tick by accident;
- contact should match perceived physical reach.

### 10. Contact And Impact Response Profile

Purpose: what happens when bodies/actions touch.

Needed for:

- Shield Rush pushing all targets in front;
- basic attack 3 shield punch;
- wolf lunge pass-through;
- maul grab/drag/release;
- stagger;
- knockback;
- interruption;
- collision blocking between player and creature.

Required fields:

- contact shape;
- contact owner;
- push direction;
- push distance/speed;
- carry target policy;
- source stop/pass-through policy;
- collision bypass policy;
- interrupt policy;
- release transition profile id.

AAA rule:

- contact/carry is not a hitbox hack;
- contact is a physics/combat response contract evaluated by server.

### 11. Skill Setup Policy

Purpose: what the actor does before committing to a skill.

Needed for:

- wolf flank-before-lunge;
- wolf chase-lunge when player flees;
- wolf maul counter under pressure;
- future creature bait/feint;
- player charged/heavy attack prep if needed.

Required fields:

- setup tactic;
- min/preferred/max range;
- commit distance;
- setup duration;
- movement ability during setup;
- allowed orbit side switching;
- target memory/latch policy;
- cancel/interruption policy;
- body/focus behavior during setup.

AAA rule:

- setup is not skill root movement;
- setup prepares the action, then committed action owns root.

### 12. Skill Selection And Behavior Policy

Purpose: choose skills intelligently without hardcoded wolf spam.

Needed for:

- wolf choosing bite/lunge/maul/dodge;
- reducing lunge repetition;
- pressure-based maul;
- defensive/evasive response;
- future creature archetypes;
- future player auto/basic/heavy routing.

Required fields:

- skill weights;
- cooldown pressure;
- repeat penalty;
- range weights;
- angle weights;
- player pressure/threat response;
- defensive state response;
- aggression curve;
- patience/pause windows;
- evasion budget interaction.

AAA rule:

- behavior difficulty comes from policy and readable decisions;
- no creature should be hardcoded to spam one skill.

### 13. Action Cancel And Queue Policy

Purpose: define what can interrupt, buffer or cancel what.

Needed for:

- basic attack combo queue;
- dodge cancel windows;
- block after attack;
- skill lock rules;
- recovery-to-next-skill timing;
- creature action commitment;
- PvP fairness.

Required fields:

- cancel source action;
- cancel target action;
- allowed phase;
- queue window;
- input buffer duration;
- priority;
- resource/cooldown handling;
- server rejection reason.

AAA rule:

- action lock must be explainable by contract;
- no hidden "locked" state outside action/channel policy.

### 14. Combo Chain Contract

Purpose: manage multi-step basic attacks and future weapon combos.

Needed for:

- M1 combo timing;
- combo reset timeout;
- third hit movement/impact;
- heavy attack on hold;
- future weapon kits.

Required fields:

- chain id;
- step order;
- input type;
- max delay between steps;
- branch rules;
- hit/movement/action contract per step;
- recovery/cancel per step.

AAA rule:

- basic attack is not just three unrelated skills;
- combo state should be contract-driven and visible to server/client.

### 15. Combat Mode And Weapon Kit Contract

Purpose: bind weapon/combat mode/selected skills/hotbar.

Needed for:

- Bulwark current M1/R/F;
- Vanguard empty until real skills;
- future 12 skills per weapon combination;
- two selected combat modes;
- heavy attack/fatality slots;
- skill tree modifiers.

Required fields:

- weapon kit id;
- combat mode id;
- slot bindings;
- selected build binding;
- allowed skill ids;
- mode switch timing;
- mode switch restrictions;
- UI display/icon ids.

AAA rule:

- hotbar must reflect server-authoritative selected mode;
- no local fallback skill injection.

### 16. Animation Presentation Contract

Purpose: link gameplay phases to animation/VFX without making animation authoritative.

Needed for:

- windup bars;
- wolf lunge telegraph;
- body/head orientation;
- skill icons/VFX;
- hitbox debug visualization;
- future real animations.

Required fields:

- animation state tags;
- montage/section ids;
- VFX cue ids;
- debug cue ids;
- phase-to-animation mapping;
- root motion authority flag;
- visual smoothing policy.

AAA rule:

- gameplay root remains server/contract-owned;
- animation presents contract state, not the other way around.

### 17. Body Shape And Collision Contract

Purpose: define gameplay capsule/body contact by actor type and state.

Needed for:

- player cylinder;
- wolf body/head length;
- shield-front contact;
- lunge pass-through;
- maul drag;
- creature not entering player unless action allows pass-through.

Required fields:

- body radius/height/length;
- contact radius;
- head/front offset;
- side/back zones;
- pass-through permission by action;
- collision response by action phase;
- ground offset.

AAA rule:

- damage hitbox, collision body and visual mesh are related but not the same;
- server should know gameplay body/contact shape, not skeletal mesh detail.

### 18. Equipment Weight Modifier Contract

Purpose: prepare future light/medium/heavy sets without rewriting movement.

Needed for:

- different leap heights/distance;
- different dodge distance/iframe/stamina cost;
- run/strafe speed modifiers;
- block stamina efficiency;
- posture/poise.

Required fields:

- weight class;
- movement profile override;
- dodge profile override;
- leap profile override;
- stamina modifier;
- poise/posture modifier;
- allowed skill modifier rules.

AAA rule:

- equipment weight swaps contracts/modifiers;
- it must not create separate hardcoded movement branches.

### 19. Skill Modifier Contract

Purpose: future skill tree/passives modifying skills without duplicating whole skills.

Needed for:

- wider Shield Rush;
- shorter Shield Bash recovery;
- lunge bleed variant;
- maul stronger control;
- dodge iframe/passive modifiers;
- PvP/PvE balance knobs.

Required fields:

- source skill id;
- modifier id;
- affected contract fields;
- additive/multiplicative/set operation;
- compatibility constraints;
- build requirement;
- PvP/PvE scaling.

AAA rule:

- modifiers patch contract fields through validated operations;
- do not clone entire skills for every passive unless the behavior truly becomes a different skill.

### 20. Runtime Contract Manifest And Coverage Gate

Purpose: fail loudly when a required contract is missing.

Needed because missing data previously turned into fallback/rubber/bug behavior.

Required fields/process:

- required contract ids by runtime mode;
- required skill ids by weapon kit;
- required profile ids by actor template;
- startup validation;
- server/client manifest hash;
- debug report for missing or mismatched contracts.

AAA rule:

- if the contract is required, boot/runtime should fail clearly;
- no C++/Go fallback should invent gameplay values.

## Suggested Priority

Do not build all tables blindly in one giant pass. Build the contract surface in the order that removes current gameplay risk.

### P0 - Movement/Combat Stability

1. `action_exit_transition_profile`
2. `action_orientation_policy`
3. `action_envelope_policy`
4. reconciliation profile completeness
5. locomotion profile completeness
6. runtime contract manifest gate

Why:

- these prevent rubber, snap, bad handoff, wrong direction and missing-contract bugs.

### P1 - Physical Combat Readability

1. temporal hit volume contract completion
2. contact/impact response profile
3. defense window contract
4. body shape/collision contract
5. skill setup policy completion

Why:

- these make hits, blocks, push, lunge, maul and shield skills feel physical and fair.

### P2 - Combat Depth

1. skill selection/behavior policy
2. action cancel/queue policy
3. combo chain contract
4. stamina/resource contract expansion
5. weapon kit/combat mode contract

Why:

- these make player/wolf combat deeper and less repetitive without changing movement authority.

### P3 - Build/Progression Future

1. equipment weight modifier contract
2. skill modifier contract
3. animation presentation contract
4. profession/non-combat skill slot contract

Why:

- these prepare the MMO progression layer after the combat foundation is stable.

## Contract Boundary Rules

Use this boundary when deciding where a new rule belongs.

| Question | Contract Owner |
| --- | --- |
| How far/fast does the action move? | `movement_action_contract` |
| What phases does the action have? | `action_envelope_policy` |
| Which way does body/head/attack face? | `action_orientation_policy` |
| How does action return to normal movement? | `action_exit_transition_profile` |
| How does prediction reconcile? | `runtime_movement_reconciliation_profile` |
| How fast does normal movement feel? | `locomotion_profile` |
| What does stamina/posture cost? | `resource_cost_regeneration_contract` |
| When is iframe/block/parry active? | `defense_window_contract` |
| Where/when does damage apply? | `temporal_hit_volume_contract` |
| What happens on physical contact? | `contact_impact_response_profile` |
| What does AI do before using skill? | `skill_setup_policy` |
| Which skill does AI choose? | `skill_selection_behavior_policy` |
| Can action cancel/buffer another? | `action_cancel_queue_policy` |
| How does M1 chain advance? | `combo_chain_contract` |
| Which slots exist in current mode? | `weapon_kit_combat_mode_contract` |
| Which visual/animation cue plays? | `animation_presentation_contract` |
| What shape does the body occupy? | `body_shape_collision_contract` |
| How does gear alter movement/combat? | `equipment_weight_modifier_contract` |
| How does passive alter a skill? | `skill_modifier_contract` |

## Server Runtime Work

### Required Runtime State

Add explicit action orientation state:

```go
type ActionOrientationState struct {
    BodyYawDeg float64
    FocusYawDeg float64
    AttackYawDeg float64
    MovementYawDeg float64
    Phase string // tactical_setup | pre_commit | airborne | landing_inertia | reentry
    AttackYawLatched bool
    PolicyID string
}
```

Add explicit envelope state:

```go
type ActionEnvelopeState struct {
    PolicyID string
    PreCommitStartedAt int64
    AirborneStartedAt int64
    LandingStartedAt int64
    Phase string
    PreCommitMs int32
    AirborneMs int32
    LandingInertiaMs int32
}
```

### Runtime Rules

1. Tactical flank/circle may choose movement intent, but it must not own the lunge root after the action enters `pre_commit`.
2. `pre_commit` owns body alignment and short forward/curved run.
3. `attack_yaw` is latched at the configured latch point.
4. Airborne root movement uses latched `attack_yaw`.
5. Hitbox direction uses `attack_yaw`, not whatever the body/focus happens to do later.
6. Landing inertia uses action transition ownership.
7. Tactical movement resumes only after envelope completion.
8. Body yaw reentry is rate-limited; no instant 180-degree turn after landing.

## Unreal Work

Unreal should consume orientation/envelope state.

Required presentation:

- body/root follows authoritative `body_yaw`;
- future head/neck/face IK follows `focus_yaw`;
- debug can draw:
  - movement direction;
  - body yaw;
  - focus yaw;
  - attack yaw;
  - current envelope phase;
- placeholder should show tactic/action labels again:
  - `circle`
  - `flank`
  - `pre_lunge_commit`
  - `lunge_airborne`
  - `landing_inertia`
  - `reentry`

Do not make Unreal decide target-facing hitboxes or action direction. It can only present what the server publishes.

## Player Support

This is not wolf-only. The same model should support player actions later.

Player examples:

- Shield Rush:
  - body yaw and attack yaw mostly match camera/aim at commit;
  - movement root owns rush;
  - hitbox follows attack yaw/front shield face;
  - exit transition hands off to grounded movement.
- Shield Bash:
  - short pre-commit/body align;
  - compact temporal hitbox;
  - short recovery.
- Heavy attack:
  - longer windup;
  - body can turn at limited rate;
  - attack yaw latches before active frames.

Player differences:

- player has local prediction and input replay;
- player orientation must be predicted from the same contract;
- creature orientation is server simulation + presentation.

The shared schema still applies.

## Immediate Lunge Retune After Architecture Exists

After the policy/runtime exists, retune lunge through DB:

- `pre_commit_ms = 100`.
- `airborne_duration_ms = 520`.
- `landing_inertia_ms = 200`.
- lower vertical curve again if PIE still reads too high.
- keep most horizontal distance in airborne phase, not landing inertia.
- keep landing inertia visible but shorter than the airborne travel.
- preserve flank setup before pre-commit.
- ensure lunge damage applies on temporal contact/crossing, not from a distant invisible wall.

## Implementation Slices

### Slice 1 - Audit Current Readiness

Map current fields:

- movement action contract timing;
- setup policy timing;
- transition state;
- creature yaw publication;
- hitbox orientation source;
- Unreal placeholder yaw use;
- proto fields available for creature orientation/debug.

Done when:

- we know exactly whether proto needs new fields or metadata can carry this temporarily.

### Slice 2 - DB Policies

Add or reuse typed DB policies for orientation and envelope.

Done when:

- no critical orientation/phase value lives as Go/C++ literal;
- `lunge` has explicit orientation and envelope policy binding.

### Slice 3 - Server State Machine

Implement:

```text
tactical_setup
  -> pre_commit
  -> airborne
  -> landing_inertia
  -> tactical_reentry
```

Done when:

- action root, body yaw, focus yaw and attack yaw are distinct where needed.

### Slice 4 - Snapshot/Proto/Unreal Presentation

Expose and consume:

- orientation phase;
- body yaw;
- focus yaw;
- attack yaw;
- envelope phase;
- debug labels.

Done when:

- the wolf can circle side-on while looking at player;
- before lunge, it curves/alines;
- during lunge, attack direction is stable.

### Slice 5 - Retune Wolf Lunge

Apply requested values through DB:

- 100ms pre-commit run/alignment;
- 520ms airborne;
- 200ms inertia;
- low long arc;
- no high parabola;
- no post-landing distance dominating the action.

Done when:

- PIE shows the diagram behavior: pre-run, low long jump, controlled landing inertia.

### Slice 6 - Apply Same Model To Player Actions

Do this after wolf lunge proves the model:

- Shield Rush;
- Shield Bash;
- basic attack 3;
- future heavy attack;
- future fatality.

Done when:

- player and creature use the same action language, with different prediction layers.

## Non-Negotiable Rules

- No `if wolf && lunge` movement branch for values that belong in DB.
- No fixing lunge by changing player leap/dodge/walk.
- No changing tactical orbit speed to fake lunge setup.
- No using head/IK visuals as gameplay authority.
- No distant hitbox damage before physical/temporal contact.
- No tactical movement while the lunge envelope owns root.
- No instant body yaw flip after landing.
- No hiding missing contract values with fallback numbers.

## Done Criteria

This roadmap is complete when:

- wolf can circle/flank side-on while focus/head points at player;
- lunge has explicit post-flank pre-commit alignment/run;
- lunge has low 520ms airborne travel;
- lunge has 200ms landing inertia;
- body yaw, focus yaw and attack yaw are separate in runtime/presentation;
- lunge hitbox/damage follows latched attack direction and temporal contact;
- tactical movement resumes after explicit reentry;
- same model is available for player actions without wolf-specific code.

## Implementation Progress - 2026-06-25 (attack_yaw latch)

This pass implemented the attack-yaw latch, which is what gives the already-plumbed
orientation/envelope data real gameplay authority. It covers roadmap orientation rules 3-5.

### Done

- New persistent runtime state `creatureActionOrientationLatch` on `entityState`
  (`internal/gameapi/runtime.go`), keyed by the owning action `InstanceID`.
- `updateCreatureActionOrientationLatchLocked` (`internal/gameapi/action_orientation.go`)
  runs each tick inside `applyCreatureActionRuntimeLocked`, **before** the hitbox is
  enqueued, and latches attack yaw at the policy-defined point:
  - `latch_at_takeoff` -> at airborne start (after the pre-commit alignment window);
  - `latch_at_active_start` -> when the action leaves windup;
  - `none`/empty -> never latches (target tracking preserved, e.g. bite).
- Committed attack yaw prefers the physical owned-root direction (`actionMotion.Direction`)
  so the hitbox sweep matches actual travel; falls back to the target snapshot.
- Hitbox: `creatureSkillImpactScheduleLocked` now sweeps along the latched line instead of
  re-aiming at the moving target every tick (was the rule-5 violation).
- Presentation: `resolveCreatureActionOrientation` freezes `attack_yaw` to the latch while
  `focus_yaw` keeps tracking the target — the two are now genuinely separate. Latch survives
  into the landing-inertia transition so attack direction stays stable through the tail.
- Latch is reset on new instance and cleared on complete/interrupt/transition-complete.
- New proto field `CreatureAIState.attack_yaw_latched` (game.proto field 61, regenerated)
  so Unreal can show latch state in the debug label.
- Tests: `internal/gameapi/attack_yaw_latch_test.go` (latch-at-takeoff + hitbox follows
  latched line + focus/attack separation + per-instance reset). Both pass.

### Pre-existing test debt found (NOT caused by this pass - for Codex to fix)

`go build` was green but `go test ./internal/gameapi/` did not even compile at HEAD `e0b931a`:
the `d2b0390` commit added 3 methods to `ProfileContractSource` and changed
`loadSkillRuntimeContract`'s signature without updating the test fakes. Fixed the compile
(added the 3 fake methods + the call site) so the suite runs again. After that, 5 tests fail
and were confirmed pre-existing by reverting this pass's logic and re-running:

- `TestStrictRuntimeCoverageAllowsNonDamagingMovementSkillWithoutHitbox` - fake wolf policy missing turn rate.
- `TestLoadRuntimeContractsFromDBUsesRequiredSkillBindings` - fake combat-core profiles missing stamina fields.
- `TestDBRuntimeSourcePromotesOnlyAfterStrictCompleteLoad` - same stamina/turn-rate fake gaps.
- `TestWolfLungeActivePhaseUsesSkillRootMotionOwner` - stale: expects active lunge z unchanged, but the vertical motion model now arcs the lunge (z 98 -> 115).
- `TestWolfMaulContactStopsBeforeOverlappingTargetUsingContractGeometry` - maul contact flags now passthrough:false stop:false.

These need either fixture-data updates (turn rate + stamina) or test expectation updates
(lunge vertical, maul contact). Left untouched to avoid masking a possible real regression.

### Deferred (next AAA steps)

- **Airborne root re-aim:** physical root direction is committed at root-start; the latch
  captures that same direction so hitbox/presentation are consistent, but the 100ms
  pre-commit alignment does not yet re-resolve the physical airborne path to the latched
  yaw. Doing so risks reintroducing rubberband, so it needs care.
- **Apply policy turn rates:** `commit_align_ms`, `focus_turn_rate_deg_s`,
  `attack_turn_rate_deg_s` are loaded but only `body_turn_rate` is used in runtime.
- **Player generalization (Slice 6):** the latch/orientation runtime only runs on the wolf
  path today; players still don't consume the policies. Same model, plus prediction layer.
