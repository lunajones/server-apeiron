# AAA Player Inventory & Equipment Roadmap

Date: 2026-06-28 · Status: design draft (grounded in the existing inventory schema) · Owner: Claude builds the spine, Codex authors item content + tuning.

## Contents
1. Vision
2. What already exists (ground truth)
3. Equipment slots
4. The bag & carrying model (slots = hard cap, weight = soft cap)
5. Weight → movement speed (encumbrance)
6. Database design (exists vs add)
7. Runtime design
8. Implementation slices
9. Connections to other systems
10. Non-negotiable rules
11. Open design decisions

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

**The bag drives capacity.** The item equipped in the `bag` slot grants the backpack its `max_slots`
(and a `max_weight` contribution). No bag equipped → minimal/zero backpack (you can equip gear but not
haul loot). Bigger/rarer bags = more slots. This makes the bag a real, sought-after item.

`max_weight` (carrying capacity) = a small **base** + the bag's contribution + (future) an attribute
bonus from **Strength/Endurance** (progression tie-in — strong/tough characters haul more).

---

## 5. Weight → movement speed (encumbrance)

The server computes **total carried weight** = Σ(`item.weight` × `quantity`) across the player's
backpack **and** equipped gear, then compares to `max_weight`:

| Load | Effect |
| --- | --- |
| `carried ≤ max_weight` | normal movement speed |
| `carried > max_weight` | **over-encumbered**: speed reduced proportionally to the overage, down to a floor |

Proposed (tunable): `speedMultiplier = clamp(1 - (carried - max_weight) / max_weight × k_enc, floor, 1)`
with e.g. `k_enc = 0.5`, `floor = 0.4`. So at 2× capacity you're at the floor (40% speed) — heavy but
never fully stuck. Slots remain the only hard wall.

Runtime: the encumbrance multiplier modulates the player's movement speed each tick (a clean multiplier
over the resolved move speed; never a teleport/rubber-band — respects the movement contracts).

---

## 6. Database design (exists vs add)

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

### `inventory_item` — EXISTS
Use as-is (`slot_index`, `is_equipped`, `quantity`, `durability`). Equipment slots map to fixed
`slot_index` values in the equipment inventory.

### Player link — EXISTS
`inventory.owner_type='player'` + `owner_id = player.id`. No schema change; the player owns inventories
by id.

---

## 7. Runtime design

- **Load on attach:** the game server loads the player's backpack + equipment inventories (extend the
  load that already brought in progression). Compute initial carried weight + capacity.
- **Write RPCs (NEW on InventoryDataService):** add/remove item, move/swap slot, equip/unequip, set
  quantity — the service is read-only today (mirror the `UpdatePlayer` pattern added for progression).
- **Commands (PlayerCommand, like the attribute-spend command):** `EQUIP_ITEM` / `UNEQUIP_ITEM` /
  `MOVE_ITEM` / `DROP_ITEM` — validated server-side (slot category match, slot occupied, two-handed
  rules, slot-count hard cap on pickup).
- **Encumbrance:** recompute carried weight on any inventory change; apply the speed multiplier (§5).
- **Bag:** equipping/unequipping a bag updates the backpack's `max_slots`/`max_weight`; unequipping a
  bag that would orphan items is rejected (or items stay until slots free up — decision §11).
- **Persistence:** inventory is DB-authoritative; persist changes (periodic flush + on change), same
  shape as progression persistence.

---

## 8. Implementation slices

1. **Schema additions.** `item_template.equip_slot` + `weapon_kit_id` + `bag_slots`/`bag_weight` +
   `equip_stats`; seed a few starter items (a bag, basic armor, the sword/shield as weapon items).
2. **Load + persist player inventories** on attach; add inventory write RPCs.
3. **Equip / unequip** command + slot validation (category match, ring/accessory/weapon rules).
4. **Bag → backpack capacity.** Equipped bag sets `max_slots`/`max_weight`; no bag = no haul.
5. **Weight → movement speed.** Carried-weight calc + encumbrance multiplier on movement.
6. **Item pickup (loot).** Add items to the backpack respecting the slot hard cap + stacking.
7. **Presentation.** Publish equipment + backpack in the snapshot for the inventory UI.
8. **(Future) Equipment stats.** Wire `equip_stats` into resistance/attributes (the gear addend) and
   weapon items into the combat kit (→ combat modes → progression mode trees).

---

## 9. Connections to other systems

- **Combat / Progression:** equipping a weapon (`weapon_kit_id`) sets your combat kit → its **combat
  modes** → the **mode trees** from the progression roadmap. The weapon slots are the bridge between
  inventory and the whole combat-mastery spine.
- **Damage / Resistances:** `equip_stats` is the **gear** term in the damage doc's
  `resistance/damage = base + gear + buffs` sum — equipment grants resistance/attribute bonuses.
- **Progression attributes:** Strength/Endurance raise carrying capacity (`max_weight`) — a reason to
  invest beyond combat.
- **Economy:** `item_template.base_value` + the player wallet (`coin`) from progression feed buying/
  selling later.

---

## 10. Non-negotiable rules

- **Slots are the hard cap; weight is the soft cap.** Never hold more stacks than `max_slots`; you may
  exceed `max_weight` but pay movement speed for it (down to a floor, never fully stuck).
- The **bag** drives backpack capacity; no bag = no hauling.
- Inventory is **DB-authoritative** and persisted; the server never invents or loses items.
- Equip validation is **server-side** (category match, two-handed, slot-count) — clients cannot bypass.
- Encumbrance is a **movement-speed multiplier** over the resolved speed — never a teleport/rubber-band.
- All capacities/weights/curves are **tunable data**; English-only for all ids/values; migrations are
  CREATE-only (column adds go in the table's CREATE migration).

---

## 11. Open design decisions

- **Stacking & weight of stacks:** weight = per-unit × quantity (assumed). Confirm.
- **Base carry capacity** without a bag (some small amount, or truly zero?).
- **Encumbrance curve** numbers (`k_enc`, `floor`) and whether it's stepped or smooth.
- **Two-handed weapons:** occupy both weapon slots — confirm the off-hand auto-clears.
- **Unequipping a bag with items still inside:** reject, or allow and lock the overflow until slots free?
- **Equipment weight:** does equipped gear count toward encumbrance (assumed yes) or only the backpack?
- **Durability** (`inventory_item.durability` exists): in scope now or later?
- **Rings/accessories:** any restriction on equipping two of the same item?
