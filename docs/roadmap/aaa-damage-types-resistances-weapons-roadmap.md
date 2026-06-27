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

### Resistance math: fraction (recommended) vs flat "souls-style"

Two ways to mitigate. Pick one; the doc assumes the fraction model.

- **Fraction / absorption % (recommended):** resistance is `0..1`; `final = damage × (1 - resist)`.
  A 0.3 physical resist always takes 70% of physical damage. Predictable, scales cleanly across a
  wide damage range, trivial to balance with three knobs — which is the goal.
- **Flat "souls-style":** the defender has a flat defense number subtracted (and/or a defense->
  absorption curve like Dark Souls). `final = damage - defense` makes armor strong vs many small
  hits but it either negates weak hits to chip damage or does nothing vs big hits, and needs a
  tuned curve per actor. More authentic to Souls, but harder to balance in an MMO with a huge
  damage spread. **Not recommended for the first pass.**

Other rules:
- `resistance_cap` (e.g. 0.85) keeps nothing fully immune by stat alone.
- `skill.armor_penetration` (0..1) subtracts from the defender's effective resistance for that hit.
- Only **three** resistances exist, by design, for balance simplicity. No "magic", no disease.

## Damage Resolution Formula

The single seam is `internal/combat/types.go` after `baseDamage` is computed. Insert resistance
mitigation before `FinalDamage`:

```text
family        = familyOf(skill.damage_type)              // physical | chemical | biological
resist        = targetResistance(targetCore, family)     // 0..1
effectiveR    = clamp(resist - skill.armor_penetration, 0, resistance_cap)
mitigated     = baseDamage * (1 - effectiveR)
FinalDamage   = mitigated
```

Then (optionally) the secondary `elemental_type` is resolved as its own mitigated instance and
added. Block/parry (already in `combat_core_profile`) apply after type mitigation, unchanged.

AAA rule: damage type and resistance are the ONLY new mitigation here. Do not fold attribute
scaling or DoT ticks into this slice.

## Proposed DB Contract Changes

### Resistances on `combat_core_profile`

```sql
ALTER TABLE apeiron.combat_core_profile
    ADD COLUMN physical_resistance   FLOAT NOT NULL DEFAULT 0.0,
    ADD COLUMN chemical_resistance   FLOAT NOT NULL DEFAULT 0.0,
    ADD COLUMN biological_resistance FLOAT NOT NULL DEFAULT 0.0,
    ADD COLUMN resistance_cap        FLOAT NOT NULL DEFAULT 0.85;
```

`physical_defense`/`magic_defense` are deprecated (keep the columns until callers are migrated,
then drop). Seed real values for the player profile and every creature profile (steppe wolf, etc.)
so resistance is data-driven for ALL actors — not just the player.

### Damage type taxonomy on `skill`

`skill.damage_type` is reused (constrain values to the taxonomy above). `family` is derived in
code (a small map), not stored, so adding a type is a one-line change. Keep `elemental_type` for
the optional secondary instance and `armor_penetration` for the formula.

### Weapon kits (the 6 initial weapons, data only — no skills yet)

**Damage type does NOT live on the weapon. It lives on the weapon's SKILLS** (`skill.damage_type`,
already in the schema). A weapon can deal several types because each of its skills picks one — a
sword's slash skill is `slashing`, its shield-bash skill is `blunt`. So `weapon_kit` only needs a
gameplay **role** (and its existing name/description for theme). The damage-type columns are NOT
added to the weapon; the "primary/secondary damage" below is just **design intent for the future
skills**, not a weapon attribute.

```sql
ALTER TABLE apeiron.weapon_kit
    ADD COLUMN role TEXT NOT NULL DEFAULT 'frontline';
-- no damage_type columns on the weapon: damage type is per-skill (skill.damage_type).
```

Register the six initial kits (combat modes / skill slots stay empty for the new five for now). The
"skills will deal" column is documentation for designing those skills later, NOT stored on the kit:

| Weapon kit | Role | Its skills will deal | Strong against (emergent) |
| --- | --- | --- | --- |
| `weaponkit_sword_shield` (exists) | tank / bruiser / frontline | slashing (sword), blunt (shield) | physical-armored, pressure |
| `weaponkit_bow` | ranged DPS | piercing; poison/fire on special ammo | light targets, biological via ammo |
| `weaponkit_warhammer` | heavy DPS / breaker | blunt; heavy posture/poise break | physical armor, shields, poise |
| `weaponkit_alchemical_censer` | technical caster / area control | fire, poison; smoke/debuff zones | grouped enemies, chemical/biological |
| `weaponkit_bone_bronze_needles` | healer / debuffer (field medic, NOT mystic) | light piercing; trauma + bio effects | precision; biological over time |
| `weaponkit_caustic_siphon` | anti-tank / offensive alchemist | corrosive; chemical fire/pressure | heavy armor, shields (chemical) |

Theme guardrail in `weapon_kit` description/metadata: needles/censer/siphon are
**field-medic / alchemist** (needles, moxa, herbs, bandages, antidote, cautery;
tank-hose-bellows-bronze-nozzle), **never** mystic staves. Same for "caster" — technical alchemist,
not a mage.

## Server Runtime Work

- Add a `damageFamilyOf(damageType)` map (physical/chemical/biological) in the combat package.
- In `types.go` damage resolution, after `baseDamage`, apply the resistance formula above using the
  target's `combat_core_profile` resistance for the family, minus `armor_penetration`.
- Load the new resistance fields into the runtime combat-core contracts (mirrors how stamina/defense
  already load), for both player and creature profiles.
- Load the weapon-kit `role` into the runtime (the kit already loads via
  `GetWeaponCombatModeSlots`); damage type comes from each skill, not the kit.
- Snapshot: optionally publish the dealt damage type and mitigated amount so the client can show
  type-correct hit feedback (numbers/colors) later.

## Implementation Slices

### Slice 1 - Resistances + resolution
Add the 3 resistance columns + `resistance_cap`, seed player + creature profiles, derive damage
family in code, apply mitigation in `types.go`. Done when a `blunt` hit on a high-physical-resist
target is reduced and a `fire` hit on the same target is not (chemical resist governs it).

### Slice 2 - Damage type taxonomy + armor penetration
Constrain/seed `skill.damage_type` to the taxonomy, wire `armor_penetration` into the formula. Done
when a high-penetration skill bypasses part of the matching resistance.

### Slice 3 - Register the 6 weapon kits (data only)
Add the `role` column + seed all six kits with id/name/role/theme (no damage-type columns on the
kit; no skills for the new five yet). Done when all six kits load with correct role/identity and the
existing sword+shield is unchanged.

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
- Physical/Chemical/Biological resistances exist on player and creature profiles, seeded and loaded.
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
