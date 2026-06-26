# AAA Pack Coordination Runtime Roadmap

Date: 2026-06-25

## Why This Doc Exists

The wolf is already declared as a pack animal everywhere in the data:

- capability `wolf_pack_harasser`
- runtime contract `contract_wolf_pack_harasser_v1`
- behavior family `beast_harasser`
- combat role `duelist`

But there is **no pack**. Every wolf runs its own solo brain (`internal/ai/creature_brain.go`)
against the same target, with zero awareness of the other wolves. The single-wolf combat is now
AAA (orientation, lunge envelope, attack-yaw latch, action transition), which makes the missing
layer obvious: the moment two or three wolves engage one player, they will all orbit the same
ring, all pick the same commit window, and all lunge at once — clumped on top of each other,
unreadable, and trivially face-tankable. That is not a pack harasser. That is N copies of one
wolf fighting the same fight.

This roadmap defines the **pack coordination layer**: a group-level authority that sits above the
individual creature brains and turns N solo wolves into one readable, relentless, fair pack. It is
the next dimension of everything already built — it does not replace the per-wolf systems, it
assigns intent into them.

## The AAA Pack Fantasy (target behavior)

What a good pack does (Shadow of Mordor caragors, Monster Hunter packs, Sekiro, Horizon):

- **Surround, not clump.** Members distribute around the target on a ring at distinct angles,
  cutting off escape, instead of stacking on one orbit arc.
- **One threat at a time, always a threat.** Only one (or a small budget of) members commit a
  lunge/maul at once. The rest harass, feint, reposition. The player always has someone to read,
  never five simultaneous unreadable commits.
- **Take turns.** Commit rotates between members so pressure is continuous but legible. While the
  player answers the front wolf, a flanker is already circling to the back for the next turn.
- **Flank and punish.** A member attacks the exposed back/side while the player faces another.
- **Cover and rotate on failure.** If the committing wolf is staggered/parried/whiffs, another is
  positioned to take the next window instead of the pack going limp.
- **Pressure scales, readability holds.** More wolves = more relentless cadence, but never more
  than the commit budget attacking at the same instant.
- **Regroup as a unit.** When losing members or morale, the pack retreats/regroups together
  instead of feeding the player one suicidal solo charge after another.

## Current State

What exists and must be reused (not duplicated):

- Per-wolf brain and tactics: `internal/ai/creature_brain.go`, `internal/ai/tactics.go`,
  `internal/ai/memory.go`, `internal/ai/region_brain_system.go`.
- Per-wolf DB policy: `creature_behavior_runtime_contract`, `creature_orbit_policy`,
  `creature_target_opportunity_policy`, `creature_skill_behavior_binding`,
  `creature_skill_setup_policy`, `creature_evasion_policy`.
- Per-wolf action authority: `action_orientation_policy`, `action_envelope_policy`, the
  attack-yaw latch (`creatureActionOrientationLatch`), and the action transition runtime
  (`creatureActionTransitionState`).
- Spawning: `spawn_zone`, `spawn_profile`, `creature_instance`.

What is missing entirely:

- Any notion of a pack as a runtime entity.
- Any shared role/slot/commit/threat/morale state across members.
- Any reason for wolf A to pick a different angle, hold a commit, or wait its turn because of
  wolf B.

## Design: The Pack As An Authority Above The Brain

A pack is a runtime coordinator that owns **group intent**, while each member's brain still owns
its own execution. The flow is:

```text
PackRuntime (group authority)
  -> assigns each member: role + ring slot + commit permission + focus target
member CreatureBrain (per-wolf)
  -> executes that intent using existing orientation/envelope/latch/transition
```

The pack never moves a wolf directly. It only sets the *constraints/intent* the wolf brain reads.
This keeps a single movement owner per creature (consistent with the action-transition roadmap's
authority rules) — the pack is a decision input, not a second movement owner.

### Roles (data-driven, rotating)

| Role | Job |
| --- | --- |
| `engager` | Holds a commit token; sets up and commits the next lunge/maul. |
| `flanker` | Circles to the target's back/blind side for the next turn. |
| `harasser` | Stays at mid range, feints/bites, keeps pressure without committing. |
| `recoverer` | Backed off, regenerating stamina/cooldown, out of the kill ring. |
| `blocker` | Holds an escape lane / cuts off the player's retreat direction. |

Roles are assigned by the pack and rotate over time and on events (commit consumed, member
staggered, member died, player repositioned).

### The Five Coordination Levers

1. **Surround slotting.** The pack assigns each in-ring member a target angle on the engagement
   ring (`spacing_deg` apart) so members spread around the target. Each wolf still uses
   `creature_orbit_policy` to move; the pack only chooses *which* angle is that wolf's slot.
2. **Commit budget / tokens.** At most `max_committed_members` (typically 1, scaling up with pack
   size) may be in a committed action at once. A member must hold a commit token to enter the
   lunge/maul `pre_commit` phase. No token -> it stays in setup/harass. This is the single most
   important readability lever.
3. **Commit rotation.** When a commit token frees (action completed/transitioned/interrupted), the
   pack hands it to the next eligible member by `role_rotation_policy` (round-robin, threat-weighted,
   or flank-priority), respecting per-member cooldown and the repeat-skill penalty already in
   `creature_target_opportunity_policy`.
4. **Shared focus / threat.** The pack maintains one threat view of the target(s). In MMO group
   fights (multiple players) it decides focus-fire vs spread, so the pack does not all chase one
   fleeing player while another revives behind them.
5. **Morale / regroup.** Pack-level morale falls with member deaths / heavy losses and triggers a
   coordinated retreat+regroup instead of solo suicide charges; rises with successful hits.

## Proposed DB Contracts

Reuse existing per-creature policy. Add only the group-level schema that has no home today.

### `pack_coordination_profile`

```sql
CREATE TABLE apeiron.pack_coordination_profile (
    id TEXT PRIMARY KEY,
    owner_kind TEXT NOT NULL DEFAULT 'creature',
    description TEXT NOT NULL DEFAULT '',
    max_members INT NOT NULL DEFAULT 6,
    max_committed_members INT NOT NULL DEFAULT 1,
    commit_token_cooldown_ms INT NOT NULL DEFAULT 1200,
    role_rotation_policy TEXT NOT NULL DEFAULT 'threat_weighted_round_robin',
    surround_spacing_deg DOUBLE PRECISION NOT NULL DEFAULT 60.0,
    surround_policy TEXT NOT NULL DEFAULT 'even_ring_then_flank_back',
    pressure_curve JSONB NOT NULL DEFAULT '[]',     -- aggression vs member count
    focus_policy TEXT NOT NULL DEFAULT 'shared_threat_soft_focus',
    morale_policy TEXT NOT NULL DEFAULT 'retreat_regroup_on_loss',
    regroup_distance_cm DOUBLE PRECISION NOT NULL DEFAULT 900.0,
    join_radius_cm DOUBLE PRECISION NOT NULL DEFAULT 1600.0,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
```

### `pack_role_policy`

```sql
CREATE TABLE apeiron.pack_role_policy (
    id TEXT PRIMARY KEY,
    pack_profile_id TEXT NOT NULL REFERENCES apeiron.pack_coordination_profile(id),
    role TEXT NOT NULL,                              -- engager | flanker | harasser | recoverer | blocker
    min_count INT NOT NULL DEFAULT 0,
    max_count INT NOT NULL DEFAULT 99,
    preferred_range_cm DOUBLE PRECISION NOT NULL DEFAULT 300.0,
    allowed_skill_ids TEXT[] NOT NULL DEFAULT '{}',  -- which skills this role may use
    movement_tactic TEXT NOT NULL DEFAULT 'orbit',
    holds_commit_token BOOLEAN NOT NULL DEFAULT FALSE,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
```

### Binding

Bind a pack profile to the spawning unit, not to a hardcoded wolf branch:

```sql
ALTER TABLE apeiron.spawn_zone     ADD COLUMN pack_coordination_profile_id TEXT NULL
    REFERENCES apeiron.pack_coordination_profile(id);
ALTER TABLE apeiron.creature_template ADD COLUMN default_pack_coordination_profile_id TEXT NULL
    REFERENCES apeiron.pack_coordination_profile(id);
```

Example wolf pack: `pack_steppe_wolf_harasser_v1` with `max_committed_members=1` at 2 wolves,
scaling to 2 at 5+ wolves via `pressure_curve`, `surround_spacing_deg=60`, roles
engager(1)/flanker(1-2)/harasser(n)/recoverer(0-2).

## Server Runtime Work

### Pack runtime state

```go
type packRuntime struct {
    PackID        string
    ProfileID     string
    MemberIDs     []uint64
    Roles         map[uint64]string   // memberID -> role
    RingSlots     map[uint64]float64  // memberID -> assigned ring angle (deg)
    CommitTokens  int                 // remaining commit budget this moment
    CommitHolder  map[uint64]time.Time// memberID -> token acquired at
    Threat        map[uint64]float64  // targetID -> shared threat
    FocusTargetID uint64
    Morale        float64
    LastRotatedAt time.Time
}
```

Lives keyed by pack id in the region brain system (`internal/ai/region_brain_system.go`), next to
where creatures are already ticked. It is movement *intent*, not movement authority — it never
writes creature positions.

### Lifecycle

1. **Form** a pack when creatures sharing a `pack_coordination_profile` are within `join_radius_cm`
   and aggroed on overlapping targets (spawn-zone grouping is the first source).
2. **Tick** the pack before the member brains each frame: refresh shared threat, reassign roles,
   slot members around the ring, free/grant commit tokens by `role_rotation_policy`.
3. Each member brain decision reads its pack assignment: role, ring slot angle, focus target, and
   `mayCommit` (does it hold a token). It then runs the EXISTING decision/skill/setup pipeline
   constrained by that intent.
4. **Dissolve / split** the pack when members scatter beyond range, all die, or targets diverge.

### Integration points (must stay clean)

- The pack only *gates and biases* the existing brain decision. A member that does not hold a
  commit token must not enter a skill's `pre_commit` (it stays in setup/harass) — this hooks the
  attack-yaw latch / envelope from the orientation roadmap, which already gates commit by phase.
- Commit tokens free on action completion/transition. This hooks the action-transition roadmap:
  release the token in/after `completeCreatureActionTransitionLocked`, not at raw completion, so
  a wolf still owns its landing inertia before the next member commits.
- Ring slotting feeds the orbit policy's target angle; it does not bypass `creature_orbit_policy`
  movement.

## Unreal / Presentation

Server stays authoritative; Unreal presents and debugs the pack:

- Publish per-member pack fields in the creature snapshot: `pack_id`, `pack_role`,
  `ring_slot_deg`, `holds_commit_token`, `pack_morale`.
- Debug placeholder should label each wolf with its role (`engager`/`flanker`/`harasser`/...) and
  draw the assigned ring slot + commit-token holder, so the coordination is diagnosable in PIE
  even with sphere placeholders.

## Implementation Slices

### Slice 1 - Pack formation + membership

Form/dissolve packs from spawn grouping + proximity + shared aggro. No behavior change yet; just
the pack exists and lists members. Done when two aggroed wolves report the same `pack_id`.

### Slice 2 - Surround slotting

Assign even ring slots and feed them into orbit target angles. Done when wolves spread around the
player instead of stacking on one arc.

### Slice 3 - Commit budget + tokens

Gate skill commit (`pre_commit` entry) on holding a token; cap concurrent commits at
`max_committed_members`. Done when only the budgeted number of wolves lunge at once.

### Slice 4 - Commit rotation + roles

Rotate the token by `role_rotation_policy`; assign engager/flanker/harasser/recoverer roles. Done
when wolves take turns committing and a flanker is pre-positioning for the next turn.

### Slice 5 - Shared threat + morale/regroup

Shared threat (incl. multi-player MMO focus) and coordinated retreat/regroup on loss. Done when a
losing pack regroups instead of feeding solo charges, and multi-player aggro is sane.

### Slice 6 - Presentation + PIE tuning

Publish pack fields, debug labels, and tune pressure/spacing/budget curves in PIE.

## Authority Matrix

| Domain | Owner | Must Not Own |
| --- | --- | --- |
| Pack membership/role/slot/commit budget | `packRuntime` (region brain) | Individual creature brain |
| Per-wolf skill choice within its role | `internal/ai` brain + DB bindings | Pack runtime |
| Committed action root / orientation / transition | existing per-wolf runtime | Pack runtime |
| Creature position | movement resolver / action runtime | Pack runtime (intent only) |
| Pack display/debug | Unreal snapshot consumer | Server-side art logic |

## Non-Negotiable Rules

- The pack assigns intent only. It must never set a creature's position directly.
- One movement owner per creature at all times (consistent with the action-transition roadmap).
- No `if numWolves > 1` hardcoded combat branches — pack behavior comes from
  `pack_coordination_profile` / `pack_role_policy`, not Go literals.
- Commit budget is the readability contract: never let more than `max_committed_members` be in a
  committed action at the same instant to "look aggressive".
- Reuse the per-wolf orientation/envelope/latch/transition; do not fork a separate pack-wolf
  movement path.
- Do not solve clumping by inflating creature collision radius; solve it with slotting.
- Pack difficulty comes from readable coordination, not from raw stat/damage inflation.

## Done Criteria

This roadmap is complete when:

- 3-5 wolves engage one player and **surround** rather than clump.
- At any instant only the budgeted number of wolves are committing; the rest harass/flank/recover.
- Commit **rotates** between members, with a flanker pre-positioned for the next turn.
- Hitting/staggering the committing wolf hands the next turn to another member instead of the pack
  going passive.
- Pressure scales with pack size while staying readable and fair to dodge/parry.
- The pack regroups as a unit on heavy loss instead of feeding solo charges.
- In MMO multi-player fights, pack focus/threat is coherent (no all-chase-one-flee).
- All of the above is driven by `pack_coordination_profile`/`pack_role_policy`, reusing the
  existing single-wolf AAA systems with no wolf-count hardcoding.

## Boundary With Existing Roadmaps

- **Action Orientation / Lunge Envelope** (done): the per-wolf execution layer. The pack chooses
  *who* commits and *where they stand*; orientation/latch decides *how that wolf faces and strikes*.
- **Creature Action Transition** (server-implemented, PIE pending): commit tokens free through the
  transition completion, so a wolf finishes its landing inertia before the next member's turn.
- This roadmap sits one level above both: it is the first **group** authority in the game and the
  natural template for future multi-creature encounters (other beast packs, humanoid squads,
  formation enemies).
