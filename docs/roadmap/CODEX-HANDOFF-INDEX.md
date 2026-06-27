# Codex Handoff Index — Apeiron Systems Map

Date: 2026-06-27 · Owner split: **Claude builds foundations → Codex expands each module's specific items.**

This is the single entry point. Each system below points to its detailed roadmap doc, states what is
**already built (the seam)**, and lists the **open items Codex picks up** when we expand that module.
Read this first, then open the linked doc for the module you're working.

> Convention reminders (non-negotiable): all code/data/ids/values/comments in **English**;
> migrations are **CREATE-only** (no ALTER — add columns to the CREATE TABLE); seeds live in
> `bootstrap/` and re-run on db-api boot; chat with the human stays Portuguese.

---

## 1. Combat Foundation — Damage Types, Resistances, Weapons
Doc: [aaa-damage-types-resistances-weapons-roadmap.md](aaa-damage-types-resistances-weapons-roadmap.md)

**Built (live):**
- 3 resistance families (Physical / Chemical / Biological) as **rating + diminishing-returns curve**
  (`combat_core_profile`), per-actor cap, K tunable via `MITIGATION_K`.
- **Armor penetration** (rating units) on skills, bypasses part of resistance.
- **Typed damage events** in the snapshot (real damage_type/family, not hardcoded).
- **5 weapon kits registered as data** (bow, warhammer, alchemical censer, bone/bronze needles,
  caustic siphon) — id/name/themed description/role+theme in metadata. **No combat modes, no skills.**

**Codex expands (per weapon, on demand):**
- For each new weapon: its `weapon_combat_mode`(s), skill slots, and skills.
- Assign **specific damage_types** to skills (slashing/piercing/blunt/fire/corrosive/poison/bleed/trauma).
  ⚠️ Today **every skill falls back to `physical`** — Chemical/Biological resistances are **built but
  dormant** until a skill carries a non-physical damage_type. First alchemist skill wakes them.
- Secondary/elemental damage instance (e.g. poison arrow) — needs a non-physical weapon first.

---

## 2. Character Progression — Combat-Mode Mastery + Character Level
Doc: [aaa-character-progression-roadmap.md](aaa-character-progression-roadmap.md)
**On Codex's worklist starting Monday 2026-06-29.**

**Built:** nothing yet — but **design is LOCKED and AAA-complete** (structure + v1 numbers; node content
is the fill-in work). DB columns `player.level/experience/attribute_points/strength/dexterity/intelligence`
exist but the **server never loads them**; creatures have **no death/kill/XP event** yet.

**Design (locked):** classless, two spines. Spine A = one level tree per combat mode per weapon (mode
owns the tree; level 1 = free basic attack; +1 pt/level; node types skill/passive/modifier/crossover;
cap 50 pts). Spine B = character XP → level (v1 cap 10, game 50) → +3 attr/level + universal milestone
picks (1 of 3). Two XP pools: level XP = damage-on-kill only; weapon XP = damage/heal/support for the
mode, capped per participant at the creature's weapon-XP value. Respec free <10, then copper→silver→gold.

**Codex expands:** author node pools per combat mode (`combat_mode_tree_node` + `passive_definition` +
`skill_modifier`), the universal milestone sets, tune the levers, wire crossover nodes that wake the
dormant chemical/biological resistances. Full DB spec + balance levers + telemetry targets in the doc.

---

## 3. Creature AI — Pack Coordination
Doc: [aaa-pack-coordination-runtime-roadmap.md](aaa-pack-coordination-runtime-roadmap.md)

**Built (live, but buggy):** wolves form one pack (`CREATURE_PACK_SIZE`), ring-slot surround,
focus policy seeded in behavior contract metadata.

**Codex expands / fixes:**
- **Bug:** surround-slotting spreads wolves beyond `join_radius` → pack splits → each sub-pack gets its
  own attack budget → multiple wolves attack simultaneously.
- **Bug:** skill direction wrong on some pack members (unconfirmed without PIE).
- Tune coordination (stagger attacks, role rotation, retreat/flank cadence).

---

## 4. Creature AI — Threat / Aggro
Doc: [aaa-threat-aggro-runtime-roadmap.md](aaa-threat-aggro-runtime-roadmap.md)

**Built (partial):** threat crediting on damage/posture (`creditThreatLocked`), threat profile read from
behavior contract metadata, leash fields on entity state.

**Codex expands:** full aggro table arbitration (highest-threat target switching, threat decay, taunt/
de-aggro, off-tank rules), integrate with pack focus.

---

## 5. Movement & Action (Codex's existing track)
Docs:
- [aaa-action-orientation-and-lunge-envelope-roadmap.md](aaa-action-orientation-and-lunge-envelope-roadmap.md)
  — Claude landed the **attack_yaw latch** (rules 3-5). Open for Codex: turn-rate caps, `commit_align_ms`,
  airborne root re-aim, **generalize orientation to players** (not just creatures).
- [aaa-creature-action-transition-runtime-roadmap.md](aaa-creature-action-transition-runtime-roadmap.md)
- [aaa-skill-movement-contract-roadmap.md](aaa-skill-movement-contract-roadmap.md)
- [aaa-temporal-melee-hit-volume-roadmap.md](aaa-temporal-melee-hit-volume-roadmap.md)
- [aaa-movement-rubberband-regression-roadmap.md](aaa-movement-rubberband-regression-roadmap.md)
- [server-apeiron-aaa-movement-action-contract-roadmap.md](server-apeiron-aaa-movement-action-contract-roadmap.md)
- [temp-reconciliation-contract-migration-roadmap.md](temp-reconciliation-contract-migration-roadmap.md)

These are primarily Codex-authored; Claude only touched the orientation/lunge latch. Codex owns the rest.

---

## Prior Handoff
[CODEX-HANDOFF-2026-06-25.md](CODEX-HANDOFF-2026-06-25.md) — superseded by this index for system status;
keep for the dated movement/orientation context.

---

## How To Use This (workflow)
1. Claude builds a system's **foundation + seam** and commits per slice (no force-push).
2. The specific, per-instance expansion (each weapon, each passive, each tuning pass) becomes a Codex
   item under the matching section above.
3. When picking up a module, open its linked doc; this index only tracks **status + ownership**.
