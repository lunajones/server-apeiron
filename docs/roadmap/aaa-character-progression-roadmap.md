# AAA Character Progression Roadmap (XP, Level, Attributes)

Date: 2026-06-27

## Why This Doc Exists

The player schema already promises an RPG character, but the runtime delivers a flat one:

- `player` table has `level` (1), `experience` (0), `attribute_points` (0), `strength` / `dexterity`
  / `intelligence` (1.0) — and **the game server never loads or uses any of them**. The player
  entity is created fresh on attach (`ensurePlayerLocked`) with hardcoded stats; it never reads the
  player's DB row.
- Creatures do not really die: there is **no death/kill event, no despawn, no XP award** when a
  creature's health hits 0.
- Damage/resistance (just built) are static profile values — nothing makes a higher-level or
  stronger character hit harder or tank more.

So a level-50 character is identical to a level-1 one. This doc adds the progression loop that
makes the combat foundation **grow with the character** — the core MMO retention loop: kill →
XP → level → attribute points → stronger combat → harder content.

Scope: load/persist player progression, XP-on-kill, leveling, and attributes scaling the combat
stats (damage, health, resistance ratings — a "sum" addend, per the damage doc). Out of scope:
skill trees, talents, gear stats, prestige.

## Design (concrete proposals — forks flagged)

### Attributes -> derived stats (the connection to combat)

Three attributes, mapped to the three weapon/damage families so builds matter:

| Attribute | Fantasy | Scales (proposed) |
| --- | --- | --- |
| **Strength** | bruiser / frontline (sword, hammer, shield) | physical damage, max health, physical resistance rating |
| **Dexterity** | precise / agile (bow, needles) | attack/crit, stamina, armor penetration |
| **Intelligence** | alchemist (censer, siphon) | chemical + biological damage, chemical + biological resistance |

This makes each attribute a real build axis tied to the weapons, and keeps the realistic theme
(intelligence = alchemy/medicine, not magic). **[FORK 1: confirm this attribute->effect mapping.]**

Scaling is a simple linear bonus on top of the base profile (so progression is an addend, never a
rewrite): e.g. `effectivePhysicalDamage = base * (1 + strength * k_str_dmg)`,
`resistanceRating = base + gear + strength*k_str_res`. Constants are tuning.

### XP and leveling

- **XP source:** killing a creature awards XP from a creature `xp_value` (default derived from the
  creature's level/tier). **[FORK 2: flat per-kill XP, or scaled by creature level vs player level
  (anti-farm)?]**
- **XP curve:** `xpToNext(level) = base * level^exponent` (proposed `base=100`, `exponent=1.5`),
  one tunable formula. **[FORK 3: confirm curve shape / numbers.]**
- **Level up:** crossing the threshold raises `level`, grants `+N attribute_points`
  (proposed `N=3`). **[FORK 4: points per level; manual spend vs auto-distribute by class?]**

### Persistence

The server loads the player's progression row on attach and writes it back on level-up and on
disconnect (and periodically). Today nothing is persisted, so this is new plumbing.

## Current State To Build On

- DB columns exist (`player.level/experience/attribute_points/strength/dexterity/intelligence`) —
  no migration needed for the base fields. An `xp_value` on `creature_template` is the one likely
  add.
- Player data service exists (`GetPlayer`-style) but the game server does not call it for these
  fields — Slice 1 wires that.
- Damage/resistance resolution (`internal/combat`) is the seam attributes feed (the damage doc
  already says resistance is `base + gear + buffs`; attributes are another addend).

## Implementation Slices

### Slice 1 - Load + persist player progression
Server loads `level/experience/attribute_points/strength/dexterity/intelligence` from the player DB
row on attach onto the player entity; writes back on disconnect. Done when a player's level/XP
survive a reconnect (no longer reset to defaults).

### Slice 2 - XP on kill
Detect creature death (health reaches 0 from a player hit), award XP to the killer from the
creature's `xp_value`, publish an XP-gain event. Done when killing a wolf raises the player's XP.

### Slice 3 - Leveling
Apply the XP curve: crossing the threshold raises level and grants attribute points; publish a
level-up event. Done when enough kills level the player up and grant points.

### Slice 4 - Attributes scale combat
Wire attributes into derived stats (damage, max health, resistance ratings) per the mapping above,
as additive bonuses over the base profile. Done when a high-strength character visibly hits harder
and tanks physical better than a fresh one.

### Slice 5 - Persistence + presentation
Persist on level-up + periodically; publish level/XP/attributes in the snapshot for the HUD. Done
when the client can show level, XP bar, and attributes.

## Non-Negotiable Rules

- Attribute scaling is an **additive bonus over the base profile**, never a rewrite of the combat
  resolution (resistance/damage stay `base + addends`).
- XP/level/attributes are DB-authoritative and persisted; the server must not invent or lose them.
- Only three attributes (Strength / Dexterity / Intelligence), tied to the three damage families.
- Curve/points/scaling constants are data/tunable, not buried Go literals.
- No skill trees / gear stats here (separate future docs); this is the XP+attribute spine only.

## Done Criteria

- Player level/XP/attributes load from DB, survive reconnect, and persist.
- Killing creatures grants XP; crossing the curve levels up and grants attribute points.
- Spending attribute points visibly changes combat (damage/health/resistance), as additive bonuses.
- All curve/scaling values are tunable data; the three attributes map to the three damage families.

## Boundary With Other Roadmaps

- **Damage Types & Resistances** (done): provides the combat stats attributes scale; resistance is
  already designed as `base + gear + buffs`, so attributes slot in as another addend with no
  refactor.
- **Future gear / skill-tree docs**: equipment stats and talents are further addends on the same
  derived-stat sum; this doc only adds the XP + attribute spine.
