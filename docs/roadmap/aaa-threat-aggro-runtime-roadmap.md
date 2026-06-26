# AAA Threat / Aggro Runtime Roadmap

Date: 2026-06-25

## Why This Doc Exists

Today a creature's idea of "who do I fight" is trivial: the wolf runtime targets *the* player
(`ensureWolfLocked(player)`), and aggression is a single scalar (`entityState.aggression`,
`aggroState`). That is fine for one wolf versus one player. It collapses the instant the game is
actually an MMO:

- One player pulls a pack: which wolf picks which target when there are also other players nearby?
- Three players fight one pack: do the wolves dogpile the closest player, focus the one doing
  damage, or split? Without a model they will all chase whoever moved last.
- A player flees: does the creature chase forever, or leash back home (souls-like reset)?
- A healer/support keeps a tank alive: nothing makes the creatures notice the threat the healer
  represents.

Threat/aggro is the model that answers "**who** does each creature want to fight, **how much**,
and **for how long**." It was carved out of the Pack Coordination roadmap because it is a large,
independent subsystem: the pack consumes a shared threat view, but threat exists even for a single
creature and is reused by every future encounter type.

## Scope

This is an **action-combat** threat model, not classic tank-and-spank. Proximity and line of sight
matter as much as raw damage, and target switching is soft and hysteretic (no per-tick flip-flop).
It must serve:

- single creature vs single player (degenerate case must equal today's behavior);
- single creature vs multiple players (target selection);
- pack vs single player (trivial — one target);
- pack vs multiple players (shared focus vs spread);
- future: creature vs creature (faction/charm), taunt/peel abilities, support threat.

## Current State

- `entityState.aggression`, `entityState.aggroState` — scalar mood, no per-target memory.
- `creature_target_opportunity_policy` — commit-angle/range/bite/lunge windows; *how* to attack a
  chosen target, not *which* target.
- Damage pipeline: `enqueueCreatureSkillImpactLocked` / impact resolution / `SnapshotEvent` damage
  events — the natural place to emit threat, currently emits none.
- `internal/ai/memory.go` — carries tactic/orbit/skill memory; the natural home for a threat table.

There is no threat table, no target selection from threat, no leash/reset, no support/taunt threat.

## Design

### Threat table

Each creature (or pack, see Pack Coordination roadmap) keeps a decaying threat table:

```text
threat[targetID] -> float
```

Updated by events, decayed over time, read for target selection. Lives in AI memory because it is
behavior, not movement authority.

### Threat sources (data-driven weights)

Split into **core** (ship the system with these) and **future** (only meaningful once the
matching player kit exists — do not build dead inputs now). The schema keeps the future weights so
no migration is needed later; they just stay zero until support/tank kits exist.

| Source | Tier | Generates threat because |
| --- | --- | --- |
| Damage dealt to the creature | core | classic; the thing hurting me |
| Posture/stagger damage | core | aggressive pressure even without HP loss |
| Proximity + time-in-range | core | action games: the thing in my face is a threat |
| First-aggro / puller bonus | core | the one who woke me starts with a lead (cheap, one-shot) |
| Heal/support done to my current target | future | needs healing to exist; the enabler behind the damage |
| Taunt / peel ability | future | needs explicit tank threat tools to exist |

Build Slices 1-4 on the core tier only. Heal/taunt are wired in Slice 6 **if and when** those
kits exist; until then their weights are 0 and they cost nothing.

### Threat decay and loss

- Per-second decay so stale targets fade.
- Faster decay when line of sight is broken or the target is out of engage range.
- Hard loss when the target dies, stealths, or leaves leash.

### Target selection (hysteretic)

Pick the highest-threat target that is within engage range and has LOS, but only switch off the
current target when a challenger exceeds it by `switch_threshold_ratio` and the
`switch_cooldown_ms` has elapsed. This stickiness is the same anti-flip-flop idea as the orbit
side-switch cooldown already in `creature_orbit_policy`.

### Leash / reset (souls-like)

If the creature is pulled beyond `leash_distance_cm` from its home/anchor (or threat fully decays
with no valid target), it **disengages**: wipe threat, return home, regen. This prevents
train-pulling abuse and matches souls-like reset expectations.

### Group focus (consumed by the pack)

The pack aggregates member threat into a pack threat view and chooses a focus policy:

- `hard_focus` — all members prefer the pack's top-threat target;
- `soft_focus` — engagers focus top threat, flankers may peel to a strong secondary;
- `spread` — each member picks its own nearest/highest.

The threat model **owns** the per-target values; the pack roadmap **owns** how members distribute
across them.

## Proposed DB Contract

```sql
CREATE TABLE apeiron.threat_profile (
    id TEXT PRIMARY KEY,
    owner_kind TEXT NOT NULL DEFAULT 'creature',
    description TEXT NOT NULL DEFAULT '',
    damage_threat_per_point       DOUBLE PRECISION NOT NULL DEFAULT 1.0,
    posture_threat_per_point      DOUBLE PRECISION NOT NULL DEFAULT 0.8,
    heal_threat_factor            DOUBLE PRECISION NOT NULL DEFAULT 0.5,
    proximity_threat_per_sec      DOUBLE PRECISION NOT NULL DEFAULT 2.0,
    proximity_range_cm            DOUBLE PRECISION NOT NULL DEFAULT 400.0,
    taunt_multiplier              DOUBLE PRECISION NOT NULL DEFAULT 4.0,
    first_aggro_bonus             DOUBLE PRECISION NOT NULL DEFAULT 25.0,
    decay_per_sec                 DOUBLE PRECISION NOT NULL DEFAULT 6.0,
    los_break_decay_multiplier    DOUBLE PRECISION NOT NULL DEFAULT 3.0,
    out_of_range_decay_multiplier DOUBLE PRECISION NOT NULL DEFAULT 2.0,
    switch_threshold_ratio        DOUBLE PRECISION NOT NULL DEFAULT 1.25,
    switch_cooldown_ms            INT NOT NULL DEFAULT 1500,
    leash_distance_cm             DOUBLE PRECISION NOT NULL DEFAULT 3500.0,
    leash_reset_policy            TEXT NOT NULL DEFAULT 'wipe_and_return_home',
    focus_policy                  TEXT NOT NULL DEFAULT 'soft_focus',
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb
);
```

Bind via `creature_template.threat_profile_id` (and optionally override per
`pack_coordination_profile`). Do not bake these values into Go.

## Server Runtime Work

### State

```go
type threatTable struct {
    ProfileID     string
    Entries       map[uint64]float64 // targetID -> threat
    CurrentTarget uint64
    LastSwitchAt  time.Time
    AnchorPos     vector             // home/leash anchor
}
```

On `entityState` (per creature) and aggregated read-only on `packRuntime`.

### Exact integration mechanism

Two precise hooks, both grounded in code that already exists. Keep them surgical.

**(1) Emit threat where damage is APPLIED to the creature, not where the attack is scheduled.**
Threat is "who is hurting me", so it is credited on the victim. The single place to hook is the
impact *application* path — the same code that reduces a victim's `health`/`posture` and produces
the damage `SnapshotEvent` (the player-skill-vs-creature resolution, e.g. `resolveRuntimeSkillImpact`
applying to the wolf, and the impact/control application in `impact.go`). At that point:

```text
on apply damage/posture D from attacker A to creature C:
    C.threat.Entries[A] += D * weight   // damage_threat_per_point / posture_threat_per_point
    propagate to C.pack threat view (if any)
```

Do NOT emit from `enqueueCreatureSkillImpactLocked` (that is creature->player, the wrong
direction). Threat is fed by the victim taking the hit.

**(2) Target selection replaces the hardcoded single-player target at one call site.** Today the
wolf loop is driven with the one player (`updateWolfPolicyLocked(wolf, player)` /
`ensureWolfLocked(player)`). Introduce one resolver:

```text
target := resolveCreatureTargetLocked(wolf, now)   // reads threatTable.CurrentTarget
updateWolfPolicyLocked(wolf, target)
```

`resolveCreatureTargetLocked` returns the threat-selected entity (with hysteresis), or the lone
player when that is the only candidate. **This is the single-player no-regression guarantee:** with
one attacker in range, `CurrentTarget` resolves to exactly that player, so the existing pipeline is
byte-for-byte unchanged. Multi-target behavior only emerges when there is more than one candidate.

**Proximity/leash run on the same creature tick** that already updates the wolf, so no new loop:
proximity adds slow threat for in-range candidates; the leash check tests `AnchorPos` distance and
disengages on breach.

## Snapshot / Presentation

Publish for debug: `current_target_id`, `threat_on_me` (top threat), `aggro_state`
(`engaged`/`leashing`/`returning`), so PIE can show who each creature is locked onto and why.

## Edge Cases and Failure Modes

- **Current target dies or leaves the region mid-fight.** Drop its entry, then re-select from the
  table next tick (respecting hysteresis against the *remaining* candidates). Never keep a dead
  `CurrentTarget`; never address a despawned id.
- **Target goes invalid** (disconnect, phase change, became untargetable): treat as death for the
  table — remove and re-select. Selection must skip non-targetable candidates, not just dead ones.
- **Stale entries grow unbounded.** Entries decay to ~0 and are then pruned, so the map cannot grow
  across a long fight against many transient targets. Prune on decay tick.
- **AoE / multi-hit from one attacker.** Threat is per applied damage event, so a multi-hit or AoE
  naturally credits the attacker once per hit. Do not special-case AoE; it is just more events.
- **Zero-damage / whiffed hits.** No damage applied => no threat from that source (proximity still
  ticks). A blocked/parried hit credits posture threat if posture damage applied, nothing if fully
  negated — which is correct (a fully blocked attacker is less threatening).
- **Two creatures share a victim's attacker.** Each creature owns its own table; the same player
  can be top-threat for several creatures independently. The pack view aggregates, but per-creature
  tables stay independent so a non-pack creature is unaffected.
- **Summons / pets / second creature as attacker (future).** The table is keyed by any entity id,
  so a future pet that damages a creature generates threat like a player. No schema change needed.
- **Leash thrash at the boundary.** Add a small hysteresis band on `leash_distance_cm` (or require
  N ticks beyond it) so a creature kited right at the leash edge does not engage/disengage every
  tick.
- **Pack vs solo ownership.** Per-creature tables are authoritative; the pack view is a read-only
  aggregate. If a creature leaves a pack, it keeps its own table and continues — no reset.

## Tuning Philosophy and Starting Values

The feel goal is **fair stickiness**: a creature commits to whoever is actually pressuring it and
does not comically flip targets, but a coordinated group can peel it off with enough pressure. It
should read like the creature has intent, not like a damage spreadsheet.

Starting values for the steppe wolf (tune in PIE):

- `damage_threat_per_point` 1.0, `posture_threat_per_point` ~0.8 — posture pressure matters but a
  little less than HP.
- `proximity_threat_per_sec` low (~2) vs `proximity_range_cm` ~400 — proximity should *engage an
  idle loiterer*, never out-rank someone actively hitting the creature.
- `decay_per_sec` ~6 — a target you stop pressuring fades over a few seconds, not instantly.
- `switch_threshold_ratio` ~1.25 + `switch_cooldown_ms` ~1500 — a challenger needs to clearly
  out-threaten the current target AND wait out the cooldown to steal aggro. This is the single
  anti-flip-flop lever; tune it first.
- `leash_distance_cm` generous (~3500) — leash is an anti-abuse backstop, not a combat leash; it
  should almost never trigger in a real fight, only on deliberate train-pulling.

Anti-goals: do not make proximity out-weigh damage (turns it into a "nearest target" toy), do not
set decay so fast that brief repositioning drops aggro, do not shrink the switch cooldown to make
fights "dynamic" — that reads as the creature being confused.

## Implementation Slices

### Slice 1 - Threat table + decay
Add the table and per-second decay; emit threat from the damage *application* path. Prune entries
that decay to zero. Done when hitting a creature raises your threat entry and it decays when you
stop. Single creature vs single player must be unchanged.

### Slice 2 - Target selection with hysteresis
Replace the hardcoded single-player target with highest-threat selection + switch threshold/cooldown.
Done when a creature among multiple players locks onto the one pressuring it and does not flip-flop.

### Slice 3 - Proximity + first-aggro + LOS decay
Add proximity threat, puller bonus, and faster decay on LOS-break/out-of-range. Done when a
creature engages a loiterer and loses a fled/hidden target naturally.

### Slice 4 - Leash / reset
Disengage + wipe + return home beyond leash. Done when train-pulling resets instead of chaining.

### Slice 5 - Group focus hook
Aggregate threat to the pack and expose focus policy for the Pack Coordination roadmap to consume.
Done when a pack focuses coherently in a multi-player fight.

### Slice 6 - Support/taunt threat + presentation + PIE tuning
Heal/support threat, taunt override, debug fields, and tuning of weights/decay/leash in PIE.

## Authority Matrix

| Domain | Owner | Must Not Own |
| --- | --- | --- |
| Per-target threat values | `threatTable` (AI memory) | Pack runtime, movement |
| How threat is generated | damage/impact pipeline + `threat_profile` | Brain literals |
| Which target a creature picks | threat selection in brain | Movement resolver |
| How a pack distributes across targets | Pack Coordination roadmap | Threat model |
| Creature position/movement | movement/action runtime | Threat model |

## Non-Negotiable Rules

- Threat is behavior memory, never movement authority.
- Single-creature vs single-player must equal today's targeting (no regression).
- No per-tick target flip-flop — switching requires threshold + cooldown.
- Threat generation values come from `threat_profile`, not Go literals.
- Leash/reset must be real (no infinite train pulls), and must wipe threat, not just slow it.
- The threat model exposes values; the pack decides distribution. Do not duplicate focus logic here.

## Done Criteria

- A creature targets whoever is actually pressuring it, chosen from a decaying threat table.
- Multiple players produce sane, sticky target selection (no flip-flop, supports peel/taunt).
- Fleeing/LOS-break/leash cause natural disengage and souls-like reset.
- Proximity makes idle-in-face targets get engaged.
- A pack reads one coherent threat view for focus vs spread.
- All driven by `threat_profile`, reusable by every creature type, with no single-player hardcoding.

## Boundary With Other Roadmaps

- **Pack Coordination** consumes the aggregated pack threat view for focus policy; this doc owns
  the per-target values and selection.
- **Action Orientation / Transition** are unaffected: threat only changes *which* target the
  existing per-creature systems aim/commit at.
- Future **PvP / faction** systems reuse the same table with player-vs-player threat sources.
