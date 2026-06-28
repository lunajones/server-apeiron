# AAA Player Inventory & Equipment Roadmap

Date: 2026-06-28 · Status: design draft (grounded in the existing inventory schema) · Owner: Claude builds the spine, Codex authors item content + tuning.

## Contents
1. Vision
2. What already exists (ground truth)
3. Equipment slots
4. The bag & carrying model (slots = hard cap, weight = soft cap)
5. Weight → movement speed (encumbrance)
6. Durability & decay
7. Database design (exists vs add)
8. Runtime design
9. Implementation slices
10. Connections to other systems
11. Non-negotiable rules
12. Open design decisions

---

## 1. Vision

The player is a **wanderer who carries their world on their back**. Gear defines your silhouette and
power (15 equipment slots); a **bag** is what lets you loot and haul. Carrying is a real decision:
you can overload yourself and move slower, but you can never carry more *stacks* than your bag holds.
Inventory should feel physical and deliberate — weight matters, space matters, what you equip matters.

This roadmap adds: named equipment slots, equip/unequip, a bag-driven backpack, the weight ↔ speed
trade-off, and item pickup — built on the inventory tables that already exist.

---

## 2. What already exists (ground truth)

`db-apeiron/migrations/026_inventory.sql` already defines a working inventory layer:

- **`inventory`** — a container owned by an entity. `owner_type` (player/creature/chest/bank/loot_container)
  + `owner_id`, `inventory_type` (backpack / **equipment** / bank / loot / storage), `max_slots`
  (default 30), `max_weight` (nullable), `current_weight`. **UNIQUE (owner_type, owner_id, inventory_type)**
  → a player has exactly one backpack and one equipment container.
- **`item_template`** — static item def: `name`, `item_type` (weapon/armor/consumable/material/quest/
  currency/misc), `rarity`, `max_stack`, **`weight`**, `is_tradable`, `is_destroyable`, `base_value`.
- **`inventory_item`** — an item instance: `inventory_id`, `item_id` → template, `slot_index`,
  `quantity`, `durability`, **`is_equipped`**. UNIQUE (inventory_id, slot_index).
- **`InventoryDataService`** (read-only today): GetInventory, GetInventoryByOwner, GetInventoryWithItems,
  GetItemTemplate, ListItemTemplatesByType, GetItem, GetItemsByInventory.

> 📑 **Full technical reference:** every existing table/column/model/RPC, the game-server side, the data
> flow, and the **anti-dup rules** are mapped in
> [aaa-inventory-system-reference.md](aaa-inventory-system-reference.md). This roadmap is the design;
> that is the as-built map.

So slots, per-item weight, current/max weight, and an equipment container are **already modeled**. We
add the named slots, the bag-capacity link, the weight→speed runtime, equip/pickup commands, and write
RPCs (the service is read-only, like PlayerDataService was before progression).

---

## 3. Equipment slots

15 named slots on the player's **equipment** inventory (English ids):

| # | Slot id | Notes |
| --- | --- | --- |
| 1 | `head` | |
| 2 | `chest` | |
| 3 | `gloves` | |
| 4 | `pants` | |
| 5 | `boots` | |
| 6 | `cape` | |
| 7 | `shoulder` | |
| 8 | `amulet` | |
| 9 | `ring_1` | accepts `ring` items |
| 10 | `ring_2` | accepts `ring` items |
| 11 | `accessory_1` | accepts `accessory` items |
| 12 | `accessory_2` | accepts `accessory` items |
| 13 | `weapon_main` | e.g. sword (main hand) |
| 14 | `weapon_off` | e.g. shield (off hand); a two-handed weapon occupies both |
| 15 | `bag` | grants the backpack (see §4) |

**Model:** add `item_template.equip_slot` = the slot **category** an item fits
(`head`/`chest`/`gloves`/`pants`/`boots`/`cape`/`shoulder`/`amulet`/`ring`/`accessory`/`weapon`/`bag`,
NULL for non-equippable). The equipment inventory uses fixed `slot_index` ↔ named-slot mapping (a server
constant). Equipping validates the item's `equip_slot` against the target slot (a `ring` item → `ring_1`
or `ring_2`; a `weapon` item → `weapon_main`/`weapon_off` by its hand type). Equipped items live in the
equipment inventory with `is_equipped = TRUE`.

---

## 4. The bag & carrying model (slots = hard cap, weight = soft cap)

Two independent limits, exactly as specified:

- **Slots = HARD cap.** The backpack's `max_slots` is the maximum number of item *stacks* the player can
  hold. You **cannot** pick up / hold more than this — period.
- **Weight = SOFT cap.** The backpack's `max_weight` is the carrying *capacity*. You **can** exceed it,
  but doing so makes you **over-encumbered** and slows you down (§5). There is no hard weight wall (only
  the slot wall).

**The bag drives capacity.** The item equipped in the `bag` slot grants the backpack its `max_slots`.
**No bag → zero slots → you cannot loot or haul anything, period.** There is no base backpack; the bag
*is* your backpack. Bigger/rarer bags = more slots. This makes the bag a real, sought-after item.

**Carrying capacity (`max_weight`)** = **no flat base** (zero) + the bag's `bag_weight` contribution +
an attribute bonus from **Strength/Endurance** (your body bears load). So a bagless character's capacity
is purely their body (attributes) — enough to wear gear, but no space to loot. The bag adds both the
slots (loot space) and extra weight capacity. *(Tuning note: a fresh character's attribute capacity must
cover basic starting equipment so they aren't immobilized by their own armor — see §5.)*

**Big-haul farming is intentionally gated.** Because the bag is small, you cannot solo-haul a fortune
from the wild. Large-scale farming is meant to need **NPC infrastructure** — hire a farmer NPC / bring a
**cart** into the wilderness and **guard it** while it works. The player protects the operation rather
than personally hauling everything. (Design vision; the NPC/cart systems are future docs — noted in §10.)

---

## 5. Weight → movement speed (encumbrance)

The server computes **total carried weight** = Σ(`item.weight` × `quantity`) across the player's
backpack **and** equipped gear, then compares to `max_weight`:

**Curve (locked):** every **1% over** capacity = **2% speed lost**, reaching **100% loss (immobile) at
50% over**. Formula: `speedMultiplier = clamp(1 - 2 × (carried - max_weight) / max_weight, 0, 1)`.

| Overage (carried vs max_weight) | Speed lost | Speed multiplier |
| --- | --- | --- |
| ≤ 0% (within capacity) | 0% | 1.00 |
| +10% | 20% | 0.80 |
| +20% | 40% | 0.60 |
| +35% | 70% | 0.30 |
| +50% or more | 100% | **0.00 (immobile)** |

So weight never blocks *picking up* (slots do that), but enough overload **can pin you in place** until
you drop weight — a real consequence, not a soft nudge. `k = 2` and the immobilize point (50%) are
tunable.

Runtime: the encumbrance multiplier modulates the player's movement speed each tick (a clean multiplier
over the resolved move speed; never a teleport/rubber-band — respects the movement contracts).

---

## 6. Durability & decay

Durability is what stops the player hoarding forever — **only equipment is permanent; everything else
decays.** Two modes, set by `durability_mode`:

- **Wear (equipment / non-consumable):** durability drops with **use** (combat, wearing). **Repairable**
  (NPC smith / station). `max_durability` varies wildly — a fine blade lasts far longer than a rusty one.
  At 0 → broken: still equipped but grants no benefit until repaired.
- **Decay (consumables, perishables, even quest items):** durability drops with **time**, not use, and is
  **NOT repairable**. At 0 → spoiled/destroyed. Meat carried 3 days in the wild rots and can no longer
  feed you. This is what forces expeditions, profession resources, and managing hunger/thirst instead of
  stockpiling.

**Storage slows decay.** Perishables in a city **bank/chest** decay slowly or not at all; in the field
(your bag) they decay at full rate — provision for a trip, don't haul a pantry on your back.

Net: **gear** is the only thing kept forever (and even it needs repair); food/potions/materials are a
**flow** you must keep replenishing — the heartbeat of the survival/profession loop. *(Caution:
quest-critical items may need exemption or very long decay so quests can't soft-lock — see §12.)*

---

## 7. Database design (exists vs add)

Legend: **EXISTS** = already in migration 026. **ADD** = new column in that table's CREATE migration
(migrations are CREATE-only — no ALTER). All ids/names English.

### `inventory` — EXISTS
Use as-is. A player gets two rows: `inventory_type='backpack'` and `inventory_type='equipment'`.
`max_slots`/`max_weight` on the backpack are driven by the equipped bag at runtime.

### `item_template` — EXISTS + ADD
| column | status | notes |
| --- | --- | --- |
| `weight`, `max_stack`, `rarity`, `item_type`, `base_value` | EXISTS | |
| `equip_slot` | **ADD** TEXT NULL | slot category (head/…/ring/accessory/weapon/bag); NULL = not equippable |
| `weapon_kit_id` | **ADD** TEXT NULL FK → weapon_kit | weapon items: which combat kit equipping grants (ties inventory → combat) |
| `bag_slots` | **ADD** INT NULL | bag items: backpack slots granted |
| `bag_weight` | **ADD** FLOAT NULL | bag items: carrying-capacity contribution |
| `equip_stats` | **ADD** JSONB | gear stat bonuses (resistance/attributes) — authored later; the "gear" addend in the damage doc's base+gear+buffs |
| `durability_mode` | **ADD** TEXT NULL | `wear` (equipment) / `decay` (perishable) / NULL = indestructible |
| `max_durability` | **ADD** FLOAT NULL | full durability; varies widely by item |
| `durability_per_use` | **ADD** FLOAT NULL | wear mode: durability lost per use |
| `decay_per_hour` | **ADD** FLOAT NULL | decay mode: durability lost per in-game hour (slowed/0 in bank) |
| `is_repairable` | **ADD** BOOL DEFAULT FALSE | equipment true; consumables false (never repairable) |

### `inventory_item` — EXISTS (+ ADD)
Use `slot_index`, `is_equipped`, `quantity`, `durability` as-is (current durability lives here).
Equipment slots map to fixed `slot_index` values in the equipment inventory. **ADD** `acquired_at`
TIMESTAMP so decay can be computed from time-held (the bank can pause it by not advancing decay).

### Player link — EXISTS
`inventory.owner_type='player'` + `owner_id = player.id`. No schema change; the player owns inventories
by id.

---

## 8. Runtime design

- **Load on attach:** the game server loads the player's backpack + equipment inventories (extend the
  load that already brought in progression). Compute initial carried weight + capacity.
- **Write RPCs (NEW on InventoryDataService):** add/remove item, move/swap slot, equip/unequip, set
  quantity — the service is read-only today (mirror the `UpdatePlayer` pattern added for progression).
- **Commands (PlayerCommand, like the attribute-spend command):** `EQUIP_ITEM` / `UNEQUIP_ITEM` /
  `MOVE_ITEM` / `DROP_ITEM` — validated server-side (slot category match, slot occupied, two-handed
  rules, slot-count hard cap on pickup).
- **Encumbrance:** recompute carried weight on any inventory change; apply the speed multiplier (§5).
- **Bag:** equipping/unequipping a bag updates the backpack's `max_slots`/`max_weight`; unequipping a
  bag that would orphan items is **rejected** (you must free the slots first — decision resolved, §12).
- **Persistence:** inventory is DB-authoritative; persist changes (periodic flush + on change), same
  shape as progression persistence.

---

## 9. Implementation slices

1. **Schema additions.** `item_template`: `equip_slot` + `weapon_kit_id` + `bag_slots`/`bag_weight` +
   `equip_stats` + durability columns; `inventory_item.acquired_at`. Seed starter items (a bag, basic
   armor, the sword/shield as weapon items, a perishable like meat).
2. **Load + persist player inventories** on attach; add inventory write RPCs.
3. **Equip / unequip** command + slot validation (category match, ring/accessory/weapon rules).
4. **Bag → backpack capacity.** Equipped bag sets `max_slots`/`max_weight`; no bag = no haul.
5. **Weight → movement speed.** Carried-weight calc + encumbrance multiplier (immobile at +50%).
6. **Item pickup (loot).** Add to the backpack respecting the slot hard cap + stacking.
7. **Durability & repair.** Wear-on-use for equipment (broken = no benefit, repairable); decay-by-time
   for perishables (spoils → destroyed, not repairable); bank pauses/slows decay.
8. **Presentation.** Publish equipment + backpack + weight/encumbrance in the snapshot for the UI.
9. **(Future) Equipment stats.** Wire `equip_stats` into resistance/attributes (the gear addend) and
   weapon items into the combat kit (→ combat modes → progression mode trees).

---

## 10. Connections to other systems

- **Combat / Progression:** equipping a weapon (`weapon_kit_id`) sets your combat kit → its **combat
  modes** → the **mode trees** from the progression roadmap. The weapon slots are the bridge between
  inventory and the whole combat-mastery spine.
- **Damage / Resistances:** `equip_stats` is the **gear** term in the damage doc's
  `resistance/damage = base + gear + buffs` sum — equipment grants resistance/attribute bonuses.
- **Progression attributes:** Strength/Endurance raise carrying capacity (`max_weight`) — a reason to
  invest beyond combat.
- **Economy:** `item_template.base_value` + the player wallet (`coin`) from progression feed buying/
  selling later.
- **NPC haulers & carts (future):** because the bag is small and perishables decay, **large-scale
  farming needs NPC infrastructure** — hire a farmer NPC, bring a **cart** into the wild, and **guard
  it** while it gathers/hauls. The player defends the operation instead of personally carrying a fortune.
  This makes wilderness expeditions a protect-the-convoy loop. (Separate future doc; the limited bag here
  is what creates the need.)
- **Survival (future):** decay (§6) feeds hunger/thirst — you provision for a trip, can't hoard food.

---

## 11. Non-negotiable rules

- **Slots are the hard cap; weight is the soft cap.** Never hold more stacks than `max_slots` (the bag).
  You *may* exceed `max_weight`, but speed drops 2% per 1% over and you are **immobile at +50%** — weight
  never blocks pickup, but enough overload pins you until you drop something.
- The **bag** drives backpack capacity; **no bag = zero slots = no hauling.** Carry capacity has no flat
  base — it comes from attributes (Strength/Endurance) + the bag.
- **Only equipment is permanent** (wears, repairable); everything else **decays by time** and is not
  repairable — nothing is hoarded forever except gear.
- Inventory is **DB-authoritative** and persisted; the server never invents or loses items.
- Equip validation is **server-side** (category match, two-handed, slot-count) — clients cannot bypass.
- Encumbrance is a **movement-speed multiplier** over the resolved speed — never a teleport/rubber-band.
- All capacities/weights/curves are **tunable data**; English-only for all ids/values; migrations are
  CREATE-only (column adds go in the table's CREATE migration).

---

## 12. Open design decisions

### Resolved (2026-06-28)
- **Base carry capacity without a bag:** **zero** flat — capacity is attributes + bag; no bag = no slots.
- **Equipped gear counts toward weight:** **yes** (equipped + backpack = carried weight).
- **Encumbrance curve:** **2% speed per 1% over, immobile at +50%** (§5).
- **Unequipping a bag with items inside:** **rejected** — free the slots first.
- **Durability:** **in now**, two modes — wear-on-use (equipment, repairable) vs decay-by-time
  (perishables/quest, not repairable); bank slows/pauses decay (§6).

### Still open
- **Stacking & weight:** weight = per-unit × quantity (assumed) — confirm; do partial stacks count fully?
- **Decay clock:** in-game hours vs real time; exact `decay_per_hour` per item; how the bank pauses it
  (don't advance `acquired_at`, or a flag?).
- **Quest items & decay:** exempt quest-critical items, or very long decay, so quests can't soft-lock?
- **New-character capacity tuning:** ensure starting attribute capacity covers basic starting gear so a
  fresh char isn't immobilized by their own armor.
- **Two-handed weapons:** occupy both weapon slots — confirm the off-hand auto-clears on equip.
- **Rings/accessories:** allow equipping two of the *same* item, or block duplicates?
- **Repair cost/where:** NPC smith only, or field-repair kits? Cost model (coin from progression wallet).
