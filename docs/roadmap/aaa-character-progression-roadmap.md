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

### Earning XP — two pools, different rules
XP is **credited only on the creature's death, only if you were in combat with it**, and always
**relative to the creature's XP values**. There are two separate pools:

**Character (level) XP — Spine B:**
- Earned **only from damage** dealt to the creature (kills). **Healing and buffs grant ZERO level
  XP.** This keeps leveling tied to fighting.
- The creature's level-XP value is **split across damage contributors by damage share** (sums to the
  creature's value — a group does not multiply it).

**Weapon (mode) XP — Spine A:**
- Earned **per combat mode**, only from **using that mode** in the fight, by **contribution**:
  damage **or** healing **or** support (buff/debuff/control). So healer/support modes (needles,
  censer) level without dealing damage.
- **Hard cap per participant = the creature's weapon-XP value.** Overhealing/over-buffing cannot
  farm beyond it: if a wolf grants 200 weapon XP, you can heal 100,000 and still earn at most 200.
  Each participant's mode credit is `min(yourContribution → xp, creatureWeaponXpValue)`.

Seam: the runtime already credits damage/posture at impact (`creditThreatLocked`,
internal/gameapi/impact.go). XP crediting extends the same hook; healing/support add parallel credit
calls when those effects resolve, accumulated per-creature and settled on death.

---

## Spine B — Character Level + Attributes

- **Character XP:** earned from **damage on kills only** (not healing/buff) → character level. Capped
  by character level (v1 cap 10; full-game cap 50).
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

## Numbers & Pacing (v1 starting values — tunable data)

**Design intent:** progression is **slow and deliberate**. Combat is laborious and creatures are
**sparse** — the player should weigh each fight and how they leave the city, not grind mindlessly.
Nothing in the game grants much XP. Reaching the v1 cap (level 10) should take **at least ~5 real
days** and be **unreachable in a single 24h day**. Spawn sparsity + these numbers together enforce
the pace.

### Caps
- **Character level:** full game = **50**; **v1 (first map) = 10**. Config-gated per map.
- **Combat-mode points:** **50 per mode** (1 point per mode level). A node may cost **>1 point**
  (e.g. a strong skill costs 3) — a second balancing lever beyond level-gating.

### Character level XP (v1, anchored on the wolf = 100 level XP)
| Level | XP to next | Cumulative | Wolves this level | Cumulative wolves |
| --- | --- | --- | --- | --- |
| 1→2 | 1,200 | 1,200 | 12 | 12 |
| 2→3 | 1,600 | 2,800 | 16 | 28 |
| 3→4 | 2,100 | 4,900 | 21 | 49 |
| 4→5 | 2,700 | 7,600 | 27 | 76 |
| 5→6 | 3,400 | 11,000 | 34 | 110 |
| 6→7 | 4,200 | 15,200 | 42 | 152 |
| 7→8 | 5,100 | 20,300 | 51 | 203 |
| 8→9 | 6,100 | 26,400 | 61 | 264 |
| 9→10 | 7,300 | 33,700 | 73 | **337** |

~337 wolves to cap 10. First level is 12 wolves (each wolf a real contribution, but never "3 and
ding"). Attribute points: **+3 per level**. Milestone passive picks at levels **10/15/20/30** (and
every +10 thereafter to 50) — pick 1 of 3, universal.

### Weapon-mode XP (long-tail, anchored on the wolf = 200 weapon XP)
Curve: `modeXpToNext(M) = round(400 · M^1.3)` for mode level M = 1..49. Anchors:

| Mode level | XP to next | ≈ wolves |
| --- | --- | --- |
| 1→2 | 400 | 2 |
| 5→6 | 3,250 | 16 |
| 10→11 | 7,960 | 40 |
| 20→21 | 19,600 | 98 |
| 30→31 | 33,200 | 166 |
| 40→41 | 48,200 | 241 |
| 49→50 | 62,900 | 314 |

Weapon mastery is the **deep chase**: by the time a player hits character cap 10 (~337 wolves,
~67k weapon XP into one mode) their main mode is only ~level 13/50. Modes 13→50 are the endgame.

### Respec
- **Free below level 10.** From level 10 it **costs gold**, scaling with level.
- Currency (Tang-era China, ~Wu Zetian): **copper → silver → gold**, `100 copper = 1 silver,
  100 silver = 1 gold`. (The "bronze cash" of the period is the copper coin — treated as copper.)
- Cost: `respecCost(L) = round(1.259^(L-10))` copper → clean milestones:
  **1 copper at lv 10 · 1 silver at lv 30 · 1 gold at lv 50** (×10 every 10 levels).

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
| `prerequisite_node_id` | TEXT NULL FK → combat_mode_tree_node | **modifier** nodes require a prior node (the skill they modify / a prior modifier) |
| `required_points_prior_tier` | INT NULL | **passive** nodes require ≥ this many points spent in the previous level/tier |
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

**`player`** — EDIT-CREATE — base columns already exist: `level`, `experience`, `attribute_points`,
`strength`, `dexterity`, `intelligence`. **Add for respec wallet:** `copper INT NOT NULL DEFAULT 0`,
`silver INT NOT NULL DEFAULT 0`, `gold INT NOT NULL DEFAULT 0` (added to the table's CREATE migration,
not via ALTER).

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
  attack. +1 point per mode level; one pick per point; never the whole shelf. A node may cost >1 point.
- XP is credited **only on death, only in combat**, always relative to the creature's XP values.
  **Level XP: damage only** (no heal/buff), split by damage share. **Weapon XP: damage/heal/support
  for the mode used, capped per participant at the creature's weapon-XP value.**
- Node gating: **modifier** nodes require a `prerequisite_node_id`; **passive** nodes require
  `required_points_prior_tier`; **crossover** nodes require an attribute threshold.
- Caps: character level 50 (v1 = 10); combat-mode points 50. Pacing target: cap 10 ≈ 5+ real days.
- Respec **free below level 10**, then **gold cost scaling 1 copper (lv10) → 1 silver (lv30) →
  1 gold (lv50)**. Currency: copper/silver/gold, 100:100.
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
