# AAA Damage Types, Resistances & Weapon Kits Roadmap

Date: 2026-06-26

## Why This Doc Exists

The schema hints at an RPG damage model but the runtime delivers none of it:

- `skill.damage_type` exists (defaults to `'physical'`, basically everything is physical today),
  plus `skill.elemental_type` (mostly NULL) and `skill.armor_penetration` (0.0) — all unused.
- `combat_core_profile` has only `physical_defense` + `magic_defense` (two defenses, and "magic"
  does not fit the realistic alchemical theme) plus **control** resistances
  (`stun_resistance`/`root_resistance`/`knockback_resistance`). There is **no damage-type
  mitigation**.
- Damage resolution (`internal/combat/types.go` ~line 262) is flat:
  `final = skill.base_damage × source.damage_dealt_multiplier × target.damage_taken_multiplier`.
  It ignores `damage_type`, defenses, and armor penetration entirely.
- Only one weapon kit exists (`weaponkit_sword_shield`); the other five planned weapons do not
  exist as data.

So a heavy hammer hits exactly like a sword, fire hits exactly like a blade, and a player in
heavy armor takes the same damage from acid as from a slash. This doc defines the damage-type +
resistance model and registers the initial weapon kits (data only, no skills yet), applied to
every actor (players and creatures).

Scope: damage types, the 3-resistance model, the resolution formula, and weapon-kit identity.
**Out of scope (future docs):** attribute scaling (str/dex/int -> damage), XP/leveling, full skill
kits per weapon, status-effect DoTs (bleed/poison ticks) beyond their damage-type classification.

## Damage Type Taxonomy

Three families, each mitigated by one resistance. Keep the type list small and realistic.

| Family | Damage types | Notes |
| --- | --- | --- |
| **Physical** | `slashing`, `piercing`, `blunt` | swords/cuts; arrows/needles/points; hammers/shields |
| **Chemical** | `fire`, `corrosive` | oil/ember/reaction; caustic/acid-alkali; anti-armor |
| **Biological** | `poison`, `bleed`, `trauma` | toxins/fumes; bleeding; deep/precise trauma |

Rules:
- Every damage instance has exactly one **damage type** (`skill.damage_type`), which maps to
  exactly one **family**, which is mitigated by exactly one **resistance**.
- Damage type lives on the **skill**, not the weapon (see Weapon Kits below): the same weapon deals
  different types through different skills (a sword's slash = `slashing`, its shield bash = `blunt`).
- A second damage type per source (`skill.elemental_type`) is optional and resolves as a separate,
  smaller damage instance with its own family/resistance (e.g. a poison-arrow = `piercing` primary
  + `poison` secondary). Not required for the first slice.
- `trauma` is a biological-family type for needles/precise/deep strikes (it can tear internally);
  it is NOT a new fourth resistance — it lives under Biological. (Keeps balancing to 3.)

## The 3-Resistance Model

Replace `physical_defense` + `magic_defense` with three resistances on every actor's
`combat_core_profile`:

| Resistance | Reduces | High on | Low on |
| --- | --- | --- | --- |
| **Physical** | slashing, piercing, blunt | heavy armor | light cloth |
| **Chemical** | fire, corrosive | treated cloth, alchemist coats, special leather | heavy metal armor |
| **Biological** | poison, bleed, trauma | (per build/consumable) | — |

### Resistance math: RATING + diminishing-returns curve (the MMO gear-chase model)

Each resistance is a **rating** (a number that grows with gear/stats), converted to a % reduction
through a diminishing-returns curve whose constant `K` scales with the attacker's tier/level:

```text
effRating  = max(0, defenderResistanceRating - skill.armor_penetration_rating)
K          = mitigationK(attackerTier)        // grows per content tier; constant for now
reduction  = effRating / (effRating + K)       // 0..1, asymptotes below 1 (never immune)
reduction  = min(reduction, resistance_cap)    // safety cap, e.g. 0.85
mitigated  = baseDamage * (1 - reduction)
FinalDamage = mitigated
```

Why this model (chosen for "do it right once", drives gear chase):

- **A climbing number to chase.** Rating grows with every gear piece; seeing it rise feels good.
- **The gear treadmill, healthily.** Higher-tier enemies raise `K`, so your old rating yields less %
  against new content — a real reason to chase the new set, without flat power inflation.
- **Specialization sells sets.** Stacking one resistance rating vs a damage-type-heavy boss spikes %
  against it → "anti-chemical set", "anti-bleed set" become build goals.
- **No trivial immunity.** The curve asymptotes below 100%; nothing is immune by stat alone, so
  there is always more to chase. (`resistance_cap` is just a safety net.)
- **Still only three knobs.** Three resistance ratings + one `K` curve scale the whole game; the
  "easy to balance with 3 resistances" goal holds.

Rejected alternatives (documented so we do not relitigate):
- **Flat `damage - defense`:** negates weak hits to chip or does nothing vs big hits; does not scale
  across an MMO damage range; weak gear-chase.
- **Linear flat %:** caps out (no reason to chase past the cap) or allows stacking to immunity.

Other rules:
- `armor_penetration` is a **rating** subtracted from the defender's effective rating (same units),
  so penetration scales with the curve too.
- `K` starts as a tunable constant; it becomes a function of attacker level/tier when the
  progression doc lands (this is the seam for that).
- Only **three** resistances exist, by design. No "magic", no disease.

## Damage Resolution Formula

The single seam is `internal/combat/types.go` after `baseDamage` is computed. Insert the rating
curve from "Resistance math" above before `FinalDamage`:

```text
family       = familyOf(skill.damage_type)               // physical | chemical | biological
rating       = targetResistanceRating(targetCore, family)
effRating    = max(0, rating - skill.armor_penetration)
reduction    = effRating / (effRating + mitigationK(attackerTier))
reduction    = min(reduction, resistance_cap)
mitigated    = baseDamage * (1 - reduction)
FinalDamage  = mitigated
```

Then (optionally) the secondary `elemental_type` is resolved as its own mitigated instance and
added. Block/parry (already in `combat_core_profile`) apply after type mitigation, unchanged.

AAA rule: damage type and resistance are the ONLY new mitigation here. Do not fold attribute
scaling or DoT ticks into this slice.

## Database Changes (full spec)

Implementation checklist. Latest migration is `043`, so new files are `044+`. Schema lives in
`db-apeiron/migrations/`, seed data in `db-apeiron/bootstrap/`.

| # | File | What |
| --- | --- | --- |
| 044 | `migrations/044_combat_resistance_ratings.sql` | add 3 resistance ratings + cap to `combat_core_profile`; deprecate the 2 old defenses |
| 045 | `migrations/045_weapon_kit_role.sql` | add `role` to `weapon_kit` |
| — | `bootstrap/0xx_*` (new or edit existing combat-core/skill/weapon seeds) | seed resistance ratings (player + creatures), `damage_type` per skill, the 6 weapon kits |
| — | no schema change | `skill.damage_type` / `elemental_type` / `armor_penetration` already exist (migration 011) |
| — | config/env | `MITIGATION_K` (default 100) |

### Migration 044 — `combat_core_profile` resistance ratings

Per-actor table (one row per combat profile: player + each creature). Resistances are **ratings**
(grow with gear/stats; 0 = none, tens/hundreds at gear tiers).

| Column | Type | Default | Meaning |
| --- | --- | --- | --- |
| `physical_resistance_rating` | FLOAT NOT NULL | 0.0 | mitigation rating vs physical family (slashing/piercing/blunt) |
| `chemical_resistance_rating` | FLOAT NOT NULL | 0.0 | mitigation rating vs chemical family (fire/corrosive) |
| `biological_resistance_rating` | FLOAT NOT NULL | 0.0 | mitigation rating vs biological family (poison/bleed/trauma) |
| `resistance_cap` | FLOAT NOT NULL | 0.85 | max % reduction the curve may reach (safety net) |

```sql
ALTER TABLE apeiron.combat_core_profile
    ADD COLUMN physical_resistance_rating   FLOAT NOT NULL DEFAULT 0.0,
    ADD COLUMN chemical_resistance_rating   FLOAT NOT NULL DEFAULT 0.0,
    ADD COLUMN biological_resistance_rating FLOAT NOT NULL DEFAULT 0.0,
    ADD COLUMN resistance_cap               FLOAT NOT NULL DEFAULT 0.85;
```

Deprecate `physical_defense` + `magic_defense` (leave the columns, stop reading them; drop in a later
migration once nothing references them). Example feel at `K=100`: 100 rating = 50% reduction, 300 =
75%, never 100%.

### Migration 045 — `weapon_kit.role`

| Column | Type | Default | Meaning |
| --- | --- | --- | --- |
| `role` | TEXT NOT NULL | `'frontline'` | gameplay role: `frontline` / `ranged_dps` / `breaker` / `area_control` / `healer` / `anti_tank` |

```sql
ALTER TABLE apeiron.weapon_kit
    ADD COLUMN role TEXT NOT NULL DEFAULT 'frontline';
-- no damage_type columns on the weapon: damage type is per-skill (skill.damage_type).
```

### No migration — `skill` damage typing (already exists, migration 011)

| Column (existing) | Use |
| --- | --- |
| `damage_type TEXT` | primary type; allowed values: `slashing`, `piercing`, `blunt`, `fire`, `corrosive`, `poison`, `bleed`, `trauma` |
| `elemental_type TEXT` (nullable) | optional secondary type (same allowed values), e.g. poison arrow |
| `armor_penetration FLOAT` | rating subtracted from the defender's effective resistance rating |

The **type -> family** map lives in code (the only code-side table; keep it tiny), not in the DB:
`physical = {slashing, piercing, blunt}`, `chemical = {fire, corrosive}`,
`biological = {poison, bleed, trauma}`. Adding a type later = one line in this map + use it on a skill.

### Weapon kits (the 6 initial weapons, data only — no skills yet)

**Damage type does NOT live on the weapon. It lives on the weapon's SKILLS** (`skill.damage_type`,
already in the schema). A weapon can deal several types because each of its skills picks one — a
sword's slash skill is `slashing`, its shield-bash skill is `blunt`. So `weapon_kit` only needs a
gameplay **role** (and its existing name/description for theme). The damage-type columns are NOT
added to the weapon; the "primary/secondary damage" below is just **design intent for the future
skills**, not a weapon attribute.

The `role` column comes from migration 045. Register the six initial kits (combat modes / skill
slots stay empty for the new five for now). The "skills will deal" column is design intent for
those future skills, NOT stored on the kit:

| `weapon_kit.id` | `name` | `role` | `primary_weapon_type` | Its skills will deal (intent) |
| --- | --- | --- | --- | --- |
| `weaponkit_sword_shield` (exists) | Sword & Shield | `frontline` | `sword` | slashing (sword), blunt (shield bash) |
| `weaponkit_bow` | Bow | `ranged_dps` | `bow` | piercing; poison/fire on special ammo |
| `weaponkit_warhammer` | Warhammer | `breaker` | `warhammer` | blunt; heavy posture/poise break |
| `weaponkit_alchemical_censer` | Alchemical Censer | `area_control` | `censer` | fire, poison; smoke/debuff zones |
| `weaponkit_bone_bronze_needles` | Bone & Bronze Needles | `healer` | `needles` | light piercing; trauma + bio effects |
| `weaponkit_caustic_siphon` | Caustic Siphon | `anti_tank` | `siphon` | corrosive; chemical fire/pressure |

Theme guardrail in each kit's `description`/`metadata`: needles/censer/siphon are
**field-medic / alchemist** (needles, moxa, herbs, bandages, antidote, cautery;
tank-hose-bellows-bronze-nozzle), **never** mystic staves. "Caster" = technical alchemist, not a mage.

### Seed changes (data)

**Resistance ratings** (Slice 1) — starting values, tune in PIE. Update the seeded combat-core
profiles so every actor has ratings:

| Profile id | `physical` | `chemical` | `biological` | `resistance_cap` | Note |
| --- | --- | --- | --- | --- | --- |
| `combat_core_player_sword_shield_v1` | 80 | 25 | 30 | 0.85 | armored frontline: high physical |
| `combat_core_steppe_wolf` | 40 | 30 | 45 | 0.85 | beast: light armor, tougher vs bio |

**Skill damage types** (Slices 1-2) — set `damage_type` on the existing skill seeds, e.g. player
basic attacks = `slashing`, shield bash = `blunt`; wolf `bite` = `piercing`, `lunge` = `piercing`,
`maul` = `blunt`. (Exact mapping is a quick design pass during Slice 2.)

**Weapon kits** (Slice 3) — set `role` on `weaponkit_sword_shield` only. The five new kits are a
backlog (table above), each INSERTed on demand when that weapon is developed (with its skills) — not
bulk-seeded now, since a weapon without skills carries no damage type and the code does not need it.

**`MITIGATION_K`** — config/env, default `100`.

## Server Runtime Work

- Add a `damageFamilyOf(damageType)` map (physical/chemical/biological) in the combat package.
- In `types.go` damage resolution, after `baseDamage`, apply the rating curve above using the
  target's family resistance rating, minus `armor_penetration`, over `K`.
- Load the resistance ratings + `resistance_cap` into the runtime combat-core contracts (mirrors how
  stamina/defense already load), for both player and creature profiles.
- Add `mitigationK` (one tunable constant for now) read from config/seed; structure it as
  `mitigationK(attackerTier)` so per-tier scaling drops in later.
- Load the weapon-kit `role` into the runtime (the kit already loads via
  `GetWeaponCombatModeSlots`); damage type comes from each skill, not the kit.
- Snapshot: optionally publish the dealt damage type and mitigated amount so the client can show
  type-correct hit feedback (numbers/colors) later.

## Implementation Slices

### Slice 1 - Resistance ratings + curve resolution
Add the 3 resistance-rating columns + `resistance_cap` + the `K` constant, seed player + creature
profiles, derive damage family in code, apply the rating curve in `types.go`. Done when a `blunt`
hit on a high-physical-rating target is reduced per the curve and a `fire` hit on the same target is
not (its chemical rating, not physical, governs it).

### Slice 2 - Damage type taxonomy + armor penetration
Constrain/seed `skill.damage_type` to the taxonomy, wire `armor_penetration` into the formula. Done
when a high-penetration skill bypasses part of the matching resistance.

### Slice 3 - Weapon kit `role` column (registration on demand)
Add the `role` column (migration 045) and set it on the existing `weaponkit_sword_shield`. **Do NOT
bulk-register the five new weapons now** — the weapon carries no damage type (that is per-skill), so
each new weapon is registered as its own row only when that weapon is actually being developed
(together with its skills + their damage types). The code does not require an unused weapon to exist.
Done when the kit loads its role and the existing sword+shield is unchanged. The five new kits are a
documented backlog (the table above), created one at a time later.

### Slice 4 - Secondary damage instance (optional)
Resolve `elemental_type` as a separate smaller mitigated instance (poison arrow, fire ammo). Done
when a poison-tipped piercing hit deals physical-mitigated + biological-mitigated portions.

### Slice 5 - Presentation hook
Publish dealt damage type + mitigated amount in the snapshot for type-correct hit feedback. Done
when the client can tell physical vs chemical vs biological damage.

## Authority Matrix

| Domain | Owner | Must Not Own |
| --- | --- | --- |
| Damage type of a hit | `skill.damage_type` (+ `elemental_type`) | runtime literals |
| Type -> family mapping | combat code (`damageFamilyOf`) | DB per-row |
| Per-actor resistance values | `combat_core_profile` (player + creature) | hardcoded Go |
| Mitigation formula | combat damage resolution (`types.go`) | Unreal |
| Weapon role/theme | `weapon_kit` | weapon damage type (that is the skill's) |

## Non-Negotiable Rules

- Exactly **three** resistances: Physical, Chemical, Biological. No magic, no disease/plague.
- Damage type and resistance are data-driven; the type->family map is the only code-side table and
  must stay tiny.
- No actor is fully immune by stat alone (`resistance_cap` < 1).
- Realistic/alchemical theme: no mystic staves; healer is a field medic.
- This slice does not add attribute scaling, XP/leveling, or DoT ticks — only typed damage +
  mitigation + weapon identity. Those are separate docs.
- Apply resistances to **every** actor (players and all creatures), not just the player.

## Done Criteria

- Every damage instance has a type that maps to one of three families and is mitigated by the
  matching per-actor resistance, with armor penetration reducing it.
- Physical/Chemical/Biological resistance ratings exist on player and creature profiles, seeded and
  loaded, mitigating via the diminishing-returns curve (rating/(rating+K), capped).
- The six initial weapon kits exist as data with correct role/theme (damage types come from each
  kit's skills, not the kit; skills optional for the new five).
- Flat damage is gone: a hammer (blunt) vs a sword (slashing) vs a siphon (corrosive) resolve
  differently against the same target based on its resistances.
- No fourth resistance and no "magic" anywhere; theme guardrails respected.

## Boundary With Other Roadmaps

- **Future Attribute & Progression doc**: str/dex/int -> derived stats (incl. these resistances and
  weapon damage scaling) and XP/level. This doc provides the typed-damage + resistance substrate it
  will scale.
- **Temporal hit volume / impact**: unchanged; this only changes how the resolved damage number is
  mitigated, not when/where contact happens.
- **Combat mode / weapon kit**: this adds weapon identity; full per-weapon skill kits (12 skills,
  combat modes) come later per the weapon-kit roadmap.
