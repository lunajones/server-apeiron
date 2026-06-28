# AAA Progression Roadmap — Combat-Mode Mastery + Character Level

Date: 2026-06-27 · Status: **design locked (structure + v1 numbers), node content TBD** ·
Ownership: Claude builds the spine, Codex authors node pools + tuning.

## Contents
1. Vision
2. Glossary
3. Spine A — Combat-Mode Mastery
4. Spine B — Character Level + Attributes
5. XP Crediting — two pools, exact rules
6. Numbers & Pacing (v1)
7. Worked Examples
8. Edge Cases
9. Balance Levers
10. Telemetry & Validation
11. Database Design
12. Implementation Slices (with acceptance criteria)
13. Non-Negotiable Rules
14. Decisions Log
15. Codex Handoff

---

## 1. Vision

Apeiron has **no classes**. Who you are is **what you wield and how you fight**. Progression is
**slow and deliberate**: creatures are sparse, every fight is laborious-but-satisfying, and *nothing*
grants much XP. The player should think about how they leave the city and pick their battles — combat
is the spine, not a treadmill. Two parallel ladders carry that fantasy:

- **Spine A — Combat-Mode Mastery:** each weapon has 2+ **combat modes (stances)**; **each mode owns
  its own deep level tree** (skills, passives, skill-modifiers). The long chase.
- **Spine B — Character Level:** kills raise your level → **attribute points** (free build) + **milestone
  passive picks** (universal). The shorter, capped ladder.

Build identity emerges from `weapon × combat mode × mode-tree picks × stat allocation × milestone
passive`. Spine B stays universal (cheap to balance); Spine A creates the real spread.

---

## 2. Glossary

| Term | Meaning |
| --- | --- |
| **Combat mode (stance)** | A way to fight with a weapon (e.g. sword&shield → `vanguard`, `bulwark`). Owns one tree. |
| **Mode level / mode point** | A combat mode's mastery level; each level grants 1 spendable point. Cap 50. |
| **Node** | A buyable entry in a mode tree: `skill`, `passive`, `modifier`, or `crossover`. Costs ≥1 point. |
| **Shelf** | The set of nodes that becomes available at a given mode level. You buy from shelves you've reached. |
| **Milestone** | A character level (10/15/20/30…) where you pick 1 of 3 universal passives. |
| **Contribution** | Your share of an encounter via damage, healing, or support — the basis for XP. |
| **Level XP** | Character-ladder XP. Damage-on-kill only. |
| **Weapon XP** | Mode-ladder XP. Damage/heal/support with that mode, capped per creature. |

---

## 3. Spine A — Combat-Mode Mastery

**One tree per combat mode, per weapon.** The mode *owns* the tree; it is not a node inside one. Sword
& shield keeps `vanguard` + `bulwark` → two trees. Initially **2 modes per weapon**, can grow.

- Organized by **mode level**. **Level 1 = the free basic attack** (not a node). Every level after
  grants **+1 point**.
- Each level exposes a **shelf** of options (skills / passives / modifiers, sometimes a crossover).
  Spend the point on **one** option from a shelf you've reached. You never buy the whole shelf, and
  strong/late skills sit deeper → must keep leveling. **Nodes may cost >1 point** (a second balancing
  lever beyond level-gating).
- **Node types & gating:**
  - `skill` — a new ability. Gated by `unlock_level`.
  - `passive` — always-on bonus. Gated by `required_points_prior_tier` (≥N points spent in the prior level).
  - `modifier` — changes an existing skill. Requires a `prerequisite_node_id` (the skill it modifies / a prior modifier).
  - `crossover` — requires an **attribute threshold** too; can grant cross-family effects (e.g. high INT
    → this mode deals chemical damage), which **wakes the dormant chemical/biological resistances**.

---

## 4. Spine B — Character Level + Attributes

> ⚠️ **The attribute model is REDESIGNED — §17 is authoritative.** The 5 attributes are **Muscles**
> (physical dmg), **Nerves** (chemical dmg), **Cruelty** (biological/DoT — poison/bleed/trauma),
> **Kindness** (healing), **Resilience** (base resistances + vitality). The table below is the old/
> as-built v1 (Strength/Dexterity/Intelligence) — kept only because the code/DB still use those names.

- **Character XP** (damage-on-kill only) → level. Cap: full game **50**, v1 **10**.
- **Attribute points:** **+3 per level**, spent freely. New model (see §17):

| Attribute | Governs |
| --- | --- |
| **Muscles** | physical damage |
| **Nerves** | chemical/alchemical damage |
| **Cruelty** | biological / damage-over-time (poison, bleed, trauma) |
| **Kindness** | healing power |
| **Resilience** | base resistances (3 families) + vitality (hp/stamina/posture) |

- **Milestone passives — per attribute:** investing in an attribute unlocks its milestone passive choices
  (pick 1 of N); effects (incl. crit / armor penetration) can be shared across different attributes'
  pools. Crit/pen are passive/weapon stats, not a dedicated attribute. Content authored later.
- All attribute/passive effects are **additive over the base profile** — they slot into the damage doc's
  `resistance/damage = base + gear + buffs` sum with **no** rewrite of combat resolution.

---

## 5. XP Crediting — two pools, exact rules

XP is credited **only when the creature dies**, **only if you were in combat with it**, always
**relative to the creature's XP values**. Two separate pools:

**Level XP (Spine B)**
- From **damage only** (healing/buff grant zero level XP — leveling stays tied to fighting).
- The creature's `experience_value` is **split across damage contributors by damage share** (a group
  does not multiply it; sums to the creature's value).

**Weapon XP (Spine A)**
- Per **combat mode**, only from **using that mode**, by contribution: damage **or** healing **or**
  support. Healer/support modes (needles, censer) level without dealing damage.
- **Per-participant hard cap = the creature's `weapon_experience_value`.** Credit =
  `min(yourContribution → xp, weapon_experience_value)`. Overhealing can't farm: heal 100,000 on a wolf
  worth 200 → you still get at most 200. Multiple modes used → credit splits across them by what each did.

**Seam:** impact already credits damage/posture (`creditThreatLocked`, internal/gameapi/impact.go).
XP crediting extends the same hook; healing/support add parallel credit calls, accumulated per-creature
and settled on death.

---

## 6. Numbers & Pacing (v1 starting values — tunable data)

**Intent:** cap 10 should take **≥ ~5 real days** and be **unreachable in one 24h day**. Spawn sparsity
+ these numbers together enforce the pace.

### Caps
- **Character level:** game 50; **v1 = 10** (config-gated per map).
- **Combat-mode points:** **50 per mode**; a node may cost >1 point.

### Character level XP — wolf = 100 level XP
| Level | XP to next | Cumulative | Wolves/level | Cum. wolves |
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

~337 wolves to cap 10; first level 12 wolves (a real contribution each, never "3 and ding").

### Weapon-mode XP — wolf = 200 weapon XP · `modeXpToNext(M) = round(400 · M^1.3)`
| Mode level | XP to next | ≈ wolves |
| --- | --- | --- |
| 1→2 | 400 | 2 |
| 5→6 | 3,250 | 16 |
| 10→11 | 7,960 | 40 |
| 20→21 | 19,600 | 98 |
| 30→31 | 33,200 | 166 |
| 40→41 | 48,200 | 241 |
| 49→50 | 62,900 | 314 |

Weapon mastery is the **deep chase**: at character cap 10 (~337 wolves, ~67k weapon XP into one mode)
your main mode is only ~level **13/50**. Modes 13→50 are the endgame.

### Respec
- **Free below level 10.** From level 10, costs gold scaling with level.
- Currency: **copper → silver → gold**, `100 copper = 1 silver, 100 silver = 1 gold`. Stored as total
  copper in the existing `player.coin`; silver/gold are display conversions (no extra columns).
- `respecCost(L) = round(1.259^(L-10))` copper → **1 copper @ lv10 · 1 silver @ lv30 · 1 gold @ lv50**
  (×10 every 10 levels).
- **A respec refunds** all attribute points and all mode-tree points (re-spend freely) and reopens
  milestone picks. Crossover nodes whose attribute requirement is no longer met **deactivate** until
  re-met (not refunded automatically).

---

## 7. Worked Examples

**A. Solo bruiser, first sessions.** Picks sword&shield `vanguard`. Each wolf ≈ a careful fight; ~12
wolves → level 2 → +3 STR (more physical dmg/hp/resist) and `vanguard` mode ~level 6 (a couple early
nodes bought). By ~337 wolves: character 10, `vanguard` ~13/50, a handful of skills + passives chosen.

**B. Duo healer (needles).** Dealing no damage, heals their partner through wolf fights. On each wolf
death they earn **0 level XP** (no damage) but up to **200 weapon XP** for the `needles` support mode
(capped — overhealing doesn't help). They level the *weapon* steadily while contributing pure support;
their character level lags unless they also land hits. Support is a viable mastery path.

**C. Crossover build (the unique hook).** Bow main, dumps attribute points into **INT to 40**. That
unlocks a `crossover` node in the bow mode: *alchemical arrows* — the (physical) bow now deals
**chemical** damage. Suddenly it bypasses heavily physical-resistant foes and **wakes the
chemical resistance** that was dormant. A build no class system could express.

---

## 8. Edge Cases

| Case | Resolution |
| --- | --- |
| **Last hit / kill-steal** | Irrelevant — credit is contribution-based, never last-hit. |
| **Mode switch mid-fight** | Weapon XP splits across the modes by what each contributed; level XP unaffected (damage is mode-agnostic). |
| **Disconnect / death before the kill** | XP settles on death; must be alive + in-combat with it at death to be credited (tunable engagement window). |
| **Pure support in a group** | 0 level XP (no damage), up-to-cap weapon XP for the support mode. |
| **Overkill / overheal** | Capped — never exceeds the creature's values. |
| **Reaching a cap** | XP past the cap is discarded (no banking in v1). |
| **Respec below a crossover's attribute gate** | The crossover node deactivates until the requirement is met again. |
| **Group damage split** | Level XP divides by damage share among contributors; weapon XP is per-participant (each capped), not divided. |

---

## 9. Balance Levers

Every knob is **tunable data**, not a Go literal:
- Creature `experience_value` (level) and `weapon_experience_value` (weapon).
- Character per-level XP table; mode XP coefficient/exponent (`400`, `1.3`).
- Node `point_cost` (per node) and `unlock_level`.
- Attribute scale constants (`k_str_dmg`, `k_str_res`, …).
- Milestone cadence + the 3-option pools.
- Respec growth base (`1.259`).
- (External) spawn density — the real throttle alongside XP.

---

## 10. Telemetry & Validation

To prove the pacing holds, log and watch:
- **Median real-time to each level**; alert if time-to-10 drops below ~5 days or balloons past a ceiling.
- **Wolves-per-level** distribution vs the table above.
- **Main-mode level at character cap** (target ~13/50).
- **Respec frequency** post-10 (is the cost meaningful but not punishing?).
- **Support-only weapon XP/hour** (is healer leveling viable but not a farm?).

---

## 11. Database Design

Legend: **NEW** = new table (CREATE). **EDIT-CREATE** = add column(s) to that table's existing CREATE
migration (migrations are **CREATE-only — never ALTER**). All ids/names English.

### Definition tables (authored, seeded in bootstrap/)

**`combat_mode_tree_node`** — NEW
| column | type | notes |
| --- | --- | --- |
| `id` | TEXT PK | e.g. `node_bulwark_l3_shieldwall` |
| `combat_mode_id` | TEXT FK → weapon_combat_mode | owning stance |
| `unlock_level` | INT | mode level whose shelf this node sits on |
| `node_type` | TEXT | `skill` \| `passive` \| `modifier` \| `crossover` |
| `point_cost` | INT DEFAULT 1 | may be >1 |
| `skill_id` | TEXT NULL FK → skill | for skill nodes / the skill a modifier targets |
| `passive_id` | TEXT NULL FK → passive_definition | for passive nodes |
| `modifier_id` | TEXT NULL FK → skill_modifier | for modifier nodes |
| `required_attribute` | TEXT NULL | crossover gate: `strength`\|`dexterity`\|`intelligence` |
| `required_attribute_value` | INT NULL | crossover threshold |
| `prerequisite_node_id` | TEXT NULL FK → combat_mode_tree_node | modifier prereq |
| `required_points_prior_tier` | INT NULL | passive prereq (points in prior level) |
| `name`, `description` | TEXT | |
| `is_enabled` | BOOL DEFAULT TRUE | |
| `metadata` | JSONB | |

**`passive_definition`** — NEW — reusable passive (weapon trees AND milestones).
`id` PK · `name`/`description` · `effect` JSONB (stat add / conditional dmg / on-dodge cleanse …) ·
`category` (`offense`\|`defense`\|`utility`\|`support`) · `is_enabled` · `metadata`.

**`skill_modifier`** — NEW — changes a skill.
`id` PK · `target_skill_id` FK → skill · `name`/`description` · `effect` JSONB (add bleed / +range /
swap damage_type …) · `is_enabled` · `metadata`.

**`attribute_milestone_passive`** — NEW — universal milestone choices (3 rows per milestone).
`id` PK · `milestone_level` INT · `choice_index` INT (0..2) · `passive_id` FK → passive_definition ·
`is_enabled` · `metadata`.

### Player-state tables (runtime-persisted)

**`player`** — **no change needed.** Base columns already exist: `level`, `experience`,
`attribute_points`, `strength`, `dexterity`, `intelligence`, plus `endurance` (a 4th attribute column —
**reserved/unused by this design**, kept at default), and **`coin BIGINT`** which serves as the wallet.
Wallet = `player.coin` stored as **total copper**; silver/gold are display conversions (`100:100`). No
new currency columns. All already exposed by `PlayerDataService.GetPlayer`.

**`player_combat_mode_progress`** — NEW — PK (`player_id`, `combat_mode_id`); `mode_level` INT DEFAULT 1,
`mode_experience` BIGINT DEFAULT 0, `unspent_points` INT DEFAULT 0.

**`player_combat_mode_node`** — NEW — PK (`player_id`, `node_id`); `node_id` FK → combat_mode_tree_node,
`unlocked_at` TIMESTAMP.

**`player_attribute_milestone_choice`** — NEW — PK (`player_id`, `milestone_level`);
`chosen_passive_id` FK → passive_definition.

### Creature side

**`creature_template`** — EDIT-CREATE — add **two** XP pools:
`experience_value INT NOT NULL DEFAULT 0` (level XP, split by damage share) and
`weapon_experience_value INT NOT NULL DEFAULT 0` (weapon XP, per-participant cap). Wolf v1: `100` / `200`.

---

## 12. Implementation Slices (with acceptance criteria)

**Build status (2026-06-27):** ✅ Slice 1 (load + persist, dev player verified live) · ✅ Slice 2
(level XP on kill) · ✅ Slice 4 (level-up + attribute points) · ✅ Slice 5 (Strength scales
hp/damage/resistance; Dex/Int bind when their families exist) · ✅ Slice 6 (snapshot publishes
progression; Unreal HUD rendering is client-side) · ✅ attribute-point **spend command**
(`COMMAND_TYPE_ALLOCATE_ATTRIBUTE`). ⏳ Slice 3 (mode tree + weapon XP) and milestone passive picks are
**Codex-authoring scope** (see §16). The level + attribute spine is complete and live.

1. **Persistence spine.** Load `player` level/xp/attributes/wallet + `player_combat_mode_progress` on
   attach; write back on disconnect. ✅ *when a player's level/XP/points survive a reconnect.*
   **Status: 1a (load) + 1b (write-back) DONE** — runtime reads progression via
   `PlayerDataService.GetPlayer` on attach and persists dirty players every 10s (+ shutdown flush) via
   the new `PlayerDataService.UpdatePlayer` RPC. Both unit-tested.
   ✅ **End-to-end demonstrable:** a persistent dev player (`local_player`, name "Wanderer") is seeded
   (`bootstrap/022_dev_player_seed.sql`); `player.creature_instance_id` was made nullable so the record
   needs no permanent live body. Verified live against the running stack: GetPlayer reads it, UpdatePlayer
   writes level/xp/attributes and they persist on re-read. ⚠️ db-api resets the schema on every boot, so
   the dev row returns to level 1 on db-api restart; progression persists across client reconnects /
   game-server restarts while db-api stays up. A real character-creation flow replaces the dev seed later.
2. **Creature death + contribution XP.** Detect death; split `experience_value` by damage share (level
   XP); credit the used mode's weapon XP capped at `weapon_experience_value`. ✅ *when killing a wolf
   raises level XP (by damage) and weapon XP (capped).*
3. **Mode leveling + tree unlock.** Mode XP curve → `mode_level` → `unspent_points`; spending inserts a
   `player_combat_mode_node` after validating `unlock_level`, `point_cost`, prereqs, crossover gate.
   ✅ *when points buy nodes and gating is enforced.*
4. **Character leveling + attributes + milestones.** Character XP curve → level (+3 points); apply
   milestone pick-1-of-3; persist. ✅ *when ~337 wolves reach level 10 with milestone picks at 10.*
5. **Scaling into combat.** Wire attributes + unlocked passives/modifiers into derived stats as additive
   bonuses (damage, health, resistance, crossover families). Healing/support credit goes live.
   ✅ *when a high-STR character visibly out-damages/out-tanks a fresh one and a crossover deals its
   cross-family damage.*
6. **Presentation.** Publish mode levels, character level/xp, attributes, chosen passives, wallet in the
   snapshot. ✅ *when the HUD can show all of it.*

---

## 13. Non-Negotiable Rules

- One tree **per combat mode per weapon**; the mode owns it. Level 1 = free basic attack. +1 point per
  mode level; one pick per point; never the whole shelf; nodes may cost >1.
- XP credited **only on death, only in combat**, relative to creature values. **Level XP: damage only**,
  split by damage share. **Weapon XP: damage/heal/support for the mode used, capped per participant at
  `weapon_experience_value`.**
- Node gating: `modifier` → `prerequisite_node_id`; `passive` → `required_points_prior_tier`;
  `crossover` → attribute threshold.
- Caps: character 50 (v1 10); mode points 50. Pacing target: cap 10 ≈ 5+ real days.
- Respec **free below 10**, then **1 copper (lv10) → 1 silver (lv30) → 1 gold (lv50)**; refunds attribute
  + mode points and reopens milestones.
- Milestone passives are **universal** — no class gating.
- All curves/costs/scales are **tunable data**, never buried Go literals.
- Effects are **additive over the base profile** — no rewrite of combat resolution.
- Migrations are **CREATE-only**; English-only for all code/data/ids/values/comments.

---

## 14. Decisions Log

**Locked:** classless; two spines; one tree per mode per weapon; level-1 free basic attack; node types
+ gating; two XP pools with the heal/support cap; +3 attr/level; universal milestone picks; caps
(char 50/v1 10, mode 50); v1 numbers (wolf 100/200, the tables); respec model + copper/silver/gold.

**Tunable (data, expected to change in balancing):** every number in §6 and §9.

**Deferred (Codex / later docs):** the actual node pools and passive/modifier effects; milestone passive
sets; gear stats; skill trees beyond combat modes; XP banking; cross-weapon/account-wide mastery.

---

## 15. Codex Handoff

- Author node pools per combat mode (`combat_mode_tree_node` + `passive_definition` + `skill_modifier`),
  per weapon, on demand.
- Author the universal milestone sets (`attribute_milestone_passive`).
- Tune the levers in §9; validate against §10 telemetry.
- Wire crossover nodes that grant cross-family damage (waking chemical/biological resistances).

---

## 16. Implementation Conclusion & Tuning Notes (2026-06-27)

### Built and live (the level + attribute spine — Claude)
- **Persistence:** server loads player progression on attach (`PlayerDataService.GetPlayer`) and persists
  dirty players every 10s + on shutdown (`PlayerDataService.UpdatePlayer`). Persistent dev player
  `local_player` ("Wanderer"); `player.creature_instance_id` made nullable. Verified live (read+write).
- **XP on kill:** creatures carry `experience_value` / `weapon_experience_value` in behavior metadata
  (wolf 100/200); a per-creature damage ledger splits **level XP** by damage share on death; the creature
  despawns. (Weapon-XP crediting to the active mode is part of Slice 3 — not yet wired.)
- **Leveling:** cumulative XP curve (v1 table, cap 10) raises level and grants +3 attribute points.
- **Attribute scaling (Slice 5):** Strength → max health (+10/pt), physical damage (+5%/pt), physical
  resistance (+2/pt), additive over the base profile.
- **Spend loop:** `COMMAND_TYPE_ALLOCATE_ATTRIBUTE` spends points into an attribute → derived stats
  recompute immediately → persists. The full playable loop (kill → XP → level → spend → stronger) is
  closed server-side.
- **Presentation:** `SnapshotEntity.player_progression` carries level/xp/attributes/points/coin + XP-bar
  fields for the Unreal HUD.

### Remaining — Codex authoring scope (needs design + content, per-weapon)
- **Slice 3 — combat-mode trees + weapon XP runtime:** the new tables (`combat_mode_tree_node`,
  `passive_definition`, `skill_modifier`, `attribute_milestone_passive`, `player_combat_mode_progress`,
  `player_combat_mode_node`, `player_attribute_milestone_choice`), the per-mode node pools, mode-XP
  crediting to the active combat mode, node unlock/gating, and crossover wiring. Design is locked in
  §3/§11; the **content** (which skills/passives/modifiers per mode) is per-weapon authoring.
- **Milestone passive picks:** author the universal 1-of-3 pools (`attribute_milestone_passive`) + the
  client pick flow.

### Tuning notes (when you return to balance)
- **Wolf evasion is overtuned** — it dodges to the player's back almost every time, making it very hard to
  land hits. This is creature evasion/AI tuning (see `aaa-threat-aggro-runtime-roadmap.md` /
  `aaa-pack-coordination-runtime-roadmap.md`), independent of progression. Tune before judging XP pacing.
- **Dev player has `strength 6`** (demo) so Slice 5 scaling is visible (150 hp, +25% dmg). Real new
  characters start at 1.0; remove/zero this once the attribute UI drives spending in PIE.
- **db-api resets the schema each boot** (dev) → the dev player returns to level 1 on db-api restart;
  progression persists across client reconnects / game-server restarts while db-api stays up.
- All curves/scales (§6, §9) are placeholder v1 values — expected to change in balancing.

---

## 17. Attribute Model — REDESIGNED to 5 (2026-06-28, AUTHORITATIVE; supersedes §4/§5)

The attribute system is redesigned. **This section is the source of truth.** §4/§5 and the as-built
Slice 5 (§16) use the OLD names and are superseded — the rename + impl impact is at the end.

### The 5 attributes
| Attribute (id) | Governs |
| --- | --- |
| **Muscles** (was Strength) | **physical** damage |
| **Nerves** (was Intelligence) | **chemical/alchemical** damage (the immediate "elemental") |
| **Cruelty** (new) | **biological / damage-over-time**: poison, bleed, **trauma** |
| **Kindness** (new) | **healing** power (via skills) |
| **Resilience** (was Endurance) | **base resistances** (all 3 families) + **vitality** (max health, stamina, posture) |

Structure: **3 damage attributes by timing** (Muscles/Nerves = immediate, Cruelty = over-time) +
**1 healing** (Kindness) + **1 defense/vitality** (Resilience). No precision attribute — see below.

### Resistance = Resilience + armor
- **Resilience** gives the base resistance to all three families + vitality.
- **Armor** adds on top. Weight class sets the total: **light < medium < heavy**; heavier = more
  resistance but **more weight** → feeds the inventory encumbrance (tank more, move less).
- Each armor **piece distributes** its resistance across families differently (a light piece with more
  physical than chemical, etc.) → mix armor to cover weaknesses. This is the inventory `equip_stats` +
  the damage doc's `resistance = base + gear` (base = Resilience, gear = armor).

### Crit / armor penetration / attack speed — no dedicated attribute
These are **milestone-passive effects and/or weapon stats**, not an attribute. The same effect (e.g.
armor penetration) can appear in **different attributes' milestone pools** — a Nerves milestone or a
Muscles milestone can both grant it. The old Dexterity is **dropped** as an attribute.

### Milestone passives — per attribute (kept)
Attribute milestones stay: investing in an attribute unlocks that attribute's milestone passive choices
(pick 1 of N), and the same effect can appear across **different attributes' pools**. Content authored later.

### Weapon → attribute scaling
- Weapons **scale with specific attributes** to increase damage (the lever tying build → weapon choice).
- **Confirmed v1:** **sword & shield scales with Muscles + Resilience.** (Other weapons when developed.)
- Scaling shape (letter grades vs linear) + stacking with the family bonus — designed later. Touches
  `weapon_kit` (a scaling spec per weapon/mode) + the damage resolution.

### Trauma
A **biological damage-over-time type** → scales with **Cruelty**, resisted by **Resilience** (biological
resistance). It is the **medic's** offensive tool (needles striking vital points), so a needle-medic
invests in **Cruelty (trauma) + Kindness (healing)** — a real hybrid.

### Implementation / rename impact
- DB `player` columns: rename `strength→muscles`, `intelligence→nerves`, `endurance→resilience`; **add**
  `cruelty`, `kindness`; **drop** `dexterity` as an attribute (crit/pen move to passives/weapon stats).
- Slice 5 scaling (currently "Strength → hp/damage/resistance") splits: **physical damage → Muscles**;
  **hp/stamina/posture + resistances → Resilience**. The spend command + snapshot keys take the new names.
- The §4 table + §16 as-built keep the OLD names until this lands.

### Consolidated incomplete points (for the future review)
- **Attribute rename + new 5-attribute model** (above) — not yet wired (code/DB still use the old 4).
- **Slice 3** — combat-mode trees + weapon-XP runtime (engine + content). Not started.
- **Per-attribute damage scaling** — only the old "Strength" path is wired (→ Muscles/Resilience split);
  **Nerves (chemical), Cruelty (biological/DoT), Kindness (healing)** scaling not wired.
- **Milestone passive pools** (per attribute; crit/pen effects shared across attributes) — not built.
- **Weapon → attribute scaling** — designed (sword+shield = Muscles+Resilience), not wired.
- **Respec, healing→weapon XP, telemetry, real character creation, creature respawn** — not done.
