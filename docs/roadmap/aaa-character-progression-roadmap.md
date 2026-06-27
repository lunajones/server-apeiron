# AAA Progression Roadmap — Combat-Mode Mastery + Character Level

Date: 2026-06-27 · Status: design (approved structure, nodes TBD) · Owner: Claude builds, Codex fills nodes/tuning.

## The Pitch (no classes)

Apeiron has **no classes**. Identity comes from **what you wield and how you fight**, on two parallel
progression spines:

- **Spine A — Combat-Mode Mastery (weapon side):** every weapon has 2+ **combat modes (stances)**;
  **each combat mode owns its own level tree.** You earn a mode's XP by *using that mode*, level it,
  and spend points in its tree on skills / passives / skill-modifiers.
- **Spine B — Character Level (attribute side):** kills grant character XP → levels → **attribute
  points** spent freely on Strength / Dexterity / Intelligence, plus **milestone passive picks**
  (choose 1 of 3 at level 10/15/20/30…), a **universal** pool shared by everyone.

Build diversity = `weapon × combat mode × mode-tree picks × stat allocation × milestone passive`.
The attribute side stays universal (easy to manage); the weapon/mode side creates the real spread.

---

## Spine A — Combat-Mode Mastery

### One tree per combat mode, per weapon
- The tree owner is the **combat mode**, not a node. Sword & shield keeps its two modes
  (`vanguard`, `bulwark`) → two trees. Initially **2 modes per weapon**, can grow.
- Each mode tree is organized by **mode level**. **Level 1 = the free basic attack** (not a node).
  Every level after grants **+1 point**.
- Each level exposes a **shelf** of options (mix of skills / passives / modifiers; sometimes a
  **crossover** node). You spend the point on **one** option from a shelf you've reached. You never
  buy the whole shelf → choice = build. Strong/late skills sit at deeper levels (must keep leveling).
- Node types: `skill` (new ability), `passive` (always-on), `modifier` (changes an existing skill),
  `crossover` (requires an attribute threshold too — bridges Spine B into the weapon, e.g. needs
  high INT to deal chemical damage with this mode → wakes the dormant chemical/biological resists).

### Earning mode XP — contribution credit (damage AND support)
- XP is **per combat mode** and only accrues from **using that mode**.
- Credit is **proportional to contribution** to the encounter, not a flat per-kill:
  - **Damage:** share of the creature's health you removed *with that mode*.
  - **Healing:** health restored to allies *with that mode*.
  - **Support:** buff/debuff/control value applied *with that mode* (so healer/support modes — needles,
    censer — level without needing damage).
- On a creature's death, its `experience_value` is split across contributors by their normalized
  contribution; each contributor's share lands on the **specific mode** they used.
- Seam: the runtime already credits damage/posture at impact (`creditThreatLocked`,
  internal/gameapi/impact.go). Contribution-XP crediting extends the same hook; healing/support add
  parallel credit calls when those effects resolve.

---

## Spine B — Character Level + Attributes

- **Character XP:** the same contribution that feeds mode XP also feeds character XP (you always
  progress your character by playing, regardless of which mode). Character XP → character level.
- **Attribute points:** each level grants points spent freely on the three attributes:

| Attribute | Theme | Scales (additive over base profile) |
| --- | --- | --- |
| **Strength** | bruiser (sword, hammer, shield) | physical damage, max health, physical resistance rating |
| **Dexterity** | precise (bow, needles) | crit, stamina, armor penetration |
| **Intelligence** | alchemist (censer, siphon) | chemical + biological damage and resistance |

- **Milestone passives:** at milestone levels (`10, 15, 20, 30, …` — tunable) the player picks
  **1 of 3** beneficial passives. The pool is **universal** (same options per milestone for everyone,
  no class gating) — easy to author/balance; build spread comes from weapon/mode + stat allocation +
  which of the 3 you pick.
- Attribute scaling is an **additive bonus over the base profile** — slots into the damage doc's
  `resistance/damage = base + gear + buffs` sum with no combat refactor.

---

## Database Design

Legend: **NEW** = new table (CREATE). **EDIT-CREATE** = add column(s) to that table's existing
CREATE TABLE migration (migrations are **CREATE-only — never ALTER**). All ids/names English.

### Definition tables (authored data, seeded in bootstrap/)

**`combat_mode_tree_node`** — NEW — a node in a combat mode's tree.
| column | type | notes |
| --- | --- | --- |
| `id` | TEXT PK | e.g. `node_bulwark_l3_shieldwall` |
| `combat_mode_id` | TEXT FK → weapon_combat_mode | owning stance |
| `unlock_level` | INT | mode level where this node's shelf appears |
| `node_type` | TEXT | `skill` \| `passive` \| `modifier` \| `crossover` |
| `point_cost` | INT DEFAULT 1 | usually 1 |
| `skill_id` | TEXT NULL FK → skill | when type is skill, or the skill a modifier targets |
| `passive_id` | TEXT NULL FK → passive_definition | when type is passive |
| `modifier_id` | TEXT NULL FK → skill_modifier | when type is modifier |
| `required_attribute` | TEXT NULL | crossover gate: `strength`\|`dexterity`\|`intelligence` |
| `required_attribute_value` | INT NULL | crossover threshold |
| `name`, `description` | TEXT | |
| `is_enabled` | BOOL DEFAULT TRUE | |
| `metadata` | JSONB | |

**`passive_definition`** — NEW — a reusable passive effect (used by weapon trees AND milestones).
| column | type | notes |
| `id` | TEXT PK | |
| `name`, `description` | TEXT | |
| `effect` | JSONB | structured effect (stat add, conditional dmg, on-dodge cleanse, …) |
| `category` | TEXT | `offense`\|`defense`\|`utility`\|`support` |
| `is_enabled` | BOOL | |
| `metadata` | JSONB | |

**`skill_modifier`** — NEW — a modifier that changes a skill.
| column | type | notes |
| `id` | TEXT PK | |
| `target_skill_id` | TEXT FK → skill | |
| `name`, `description` | TEXT | |
| `effect` | JSONB | what it changes (add bleed, +range, swap damage_type, …) |
| `is_enabled` | BOOL | |
| `metadata` | JSONB | |

**`attribute_milestone_passive`** — NEW — universal milestone choices (3 rows per milestone level).
| column | type | notes |
| `id` | TEXT PK | |
| `milestone_level` | INT | 10, 15, 20, 30… |
| `choice_index` | INT | 0..2 (the 3 options) |
| `passive_id` | TEXT FK → passive_definition | the granted passive |
| `is_enabled` | BOOL | |
| `metadata` | JSONB | |

### Player-state tables (runtime-persisted)

**`player`** — EDIT-CREATE — columns already exist: `level`, `experience`, `attribute_points`,
`strength`, `dexterity`, `intelligence`. No change needed for the base fields.

**`player_combat_mode_progress`** — NEW — per player per mode mastery.
| column | type | notes |
| `player_id` | TEXT FK → player | |
| `combat_mode_id` | TEXT FK → weapon_combat_mode | |
| `mode_level` | INT DEFAULT 1 | |
| `mode_experience` | BIGINT DEFAULT 0 | |
| `unspent_points` | INT DEFAULT 0 | |
| PK | (player_id, combat_mode_id) | |

**`player_combat_mode_node`** — NEW — nodes a player unlocked.
| column | type | notes |
| `player_id` | TEXT | |
| `node_id` | TEXT FK → combat_mode_tree_node | |
| `unlocked_at` | TIMESTAMP | |
| PK | (player_id, node_id) | |

**`player_attribute_milestone_choice`** — NEW — milestone pick per player.
| column | type | notes |
| `player_id` | TEXT | |
| `milestone_level` | INT | |
| `chosen_passive_id` | TEXT FK → passive_definition | |
| PK | (player_id, milestone_level) | |

### Creature side

**`creature_template`** — EDIT-CREATE — add `experience_value INT NOT NULL DEFAULT 0` (XP pool split
across contributors on death). Tier-derived default acceptable.

---

## Implementation Slices

1. **Persistence spine.** Server loads `player` level/xp/attributes + `player_combat_mode_progress`
   on attach; writes back on disconnect. (Today the server loads none of it.)
2. **Creature death + contribution XP.** Detect death, split `experience_value` by contribution
   (damage now; healing/support hooks stubbed), credit character XP + the used mode's XP.
3. **Mode leveling + tree unlock.** Mode XP curve → mode level → `unspent_points`; spend a point to
   insert a `player_combat_mode_node` (validate `unlock_level`, type, crossover attribute gate).
4. **Character leveling + attributes + milestones.** Character XP curve → level → attribute_points;
   apply milestone `pick 1 of 3`; persist choices.
5. **Attributes + nodes scale combat.** Wire attributes and unlocked passives/modifiers into derived
   stats as additive bonuses over the base profile (damage, health, resistance, crossover damage
   families). Healing/support contribution credit goes live here.
6. **Presentation.** Publish mode levels, character level/xp, attributes, and chosen passives in the
   snapshot for the HUD.

## Non-Negotiable Rules

- One tree **per combat mode per weapon**; the mode owns the tree (not a node). Level 1 = free basic
  attack. +1 point per mode level; one pick per point; never the whole shelf.
- Mode XP only from **using that mode**, credited by **contribution** (damage + healing + support).
- Attribute milestone passives are **universal** (same per milestone for all) — no class gating.
- All scaling/curves/costs are **tunable data**, never buried Go literals.
- Attribute/passive effects are **additive over the base profile** — no rewrite of combat resolution.
- Migrations are **CREATE-only**; new tables are CREATE, column adds go in the table's CREATE migration.
- English-only for all code/data/ids/values/comments.

## Codex Handoff (after Claude builds the spine)

- Author the actual node pools per combat mode (`combat_mode_tree_node` + `passive_definition` +
  `skill_modifier`) — per weapon, on demand.
- Author the universal milestone passive sets (`attribute_milestone_passive`).
- Tune curves (mode XP, character XP), point costs, milestone cadence, attribute scale constants.
- Wire crossover nodes that grant cross-family damage (wakes chemical/biological resistances).
