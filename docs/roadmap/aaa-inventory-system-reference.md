# AAA Inventory System — Technical Reference (current state, fully mapped)

Date: 2026-06-28 · Companion to `aaa-player-inventory-equipment-roadmap.md` (that = design/plan; this =
**everything that exists today, mapped**: tables, columns, models, the gRPC surface in/out, the
game-server side, the data flow, and the anti-duplication rules). Source of truth: `db-apeiron`
migration 026 + the inventory repo/handler/proto + `server-apeiron` systems.

## Contents
1. Component map (where everything lives)
2. Tables — full column/constraint/index reference
3. Data-model mapping (table ↔ Go struct ↔ proto)
4. gRPC service surface — what goes IN / OUT
5. Repository surface (read + write that exists)
6. Game-server side
7. End-to-end data flow
8. Anti-duplication & integrity rules
9. Gaps vs the design (what must be built)

---

## 1. Component map (where everything lives)

| Layer | File | Role |
| --- | --- | --- |
| **Schema** | `db-apeiron/migrations/026_inventory.sql` | the 3 tables (inventory, item_template, inventory_item) |
| **Repo (DB ops)** | `db-apeiron/internal/repository/postgres/inventory_repository.go` | all SQL read+write + Go models |
| **Handler (gRPC)** | `db-apeiron/internal/grpc/handlers/inventory_data_handler.go` | repo → proto mapping (read-only today) |
| **Service (proto)** | `db-apeiron/proto/apeiron/v1/inventory_data_service.proto` + `common.proto` | RPCs + messages |
| **Item cache** | `db-apeiron/internal/cache/item_cache.go` | caches static `item_template` reads |
| **Server client** | `server-apeiron/internal/dbapeiron/client.go` | the InventoryDataService client (read) |
| **Server system** | `server-apeiron/internal/systems/inventory_system.go` | **stub only** — a `System` at `PhasePersistence`; no logic yet |

---

## 2. Tables — full column/constraint/index reference

### `apeiron.inventory` — a container owned by an entity
| column | type | default | notes |
| --- | --- | --- | --- |
| `id` | TEXT | — | **PK** |
| `owner_type` | TEXT | — | NOT NULL · CHECK ∈ player/creature/chest/bank/loot_container |
| `owner_id` | TEXT | — | NOT NULL (entity id; no FK — generic) |
| `inventory_type` | TEXT | `backpack` | NOT NULL · CHECK ∈ backpack/equipment/bank/loot/storage |
| `max_slots` | INT | 30 | NOT NULL · CHECK > 0 |
| `max_weight` | FLOAT | NULL | nullable · CHECK NULL or ≥ 0 |
| `current_weight` | FLOAT | 0.0 | NOT NULL · CHECK ≥ 0 |
| `is_locked` | BOOLEAN | FALSE | NOT NULL — freeze flag (anti-dup, §8) |
| `created_at` / `updated_at` | TIMESTAMP | NOW() | NOT NULL |

- **UNIQUE (owner_type, owner_id, inventory_type)** → an owner has at most one of each type (one backpack,
  one equipment, …).
- Indexes: `idx_inventory_owner(owner_type, owner_id)`, `idx_inventory_type(inventory_type)`.

### `apeiron.item_template` — static item definition
| column | type | default | notes |
| --- | --- | --- | --- |
| `id` | TEXT | — | **PK** |
| `name` | TEXT | — | NOT NULL |
| `description` | TEXT | `''` | NOT NULL |
| `item_type` | TEXT | — | NOT NULL · CHECK ∈ weapon/armor/consumable/material/quest/currency/misc |
| `rarity` | TEXT | `common` | NOT NULL · CHECK ∈ common/uncommon/rare/epic/legendary/unique |
| `max_stack` | INT | 1 | NOT NULL · CHECK ≥ 1 |
| `weight` | FLOAT | 0.0 | NOT NULL · CHECK ≥ 0 |
| `is_tradable` | BOOLEAN | TRUE | NOT NULL |
| `is_destroyable` | BOOLEAN | TRUE | NOT NULL |
| `base_value` | BIGINT | 0 | NOT NULL · CHECK ≥ 0 |
| `created_at` / `updated_at` | TIMESTAMP | NOW() | NOT NULL |

- Indexes: `idx_item_template_item_type`, `idx_item_template_rarity`.

### `apeiron.inventory_item` — an item INSTANCE in a slot
| column | type | default | notes |
| --- | --- | --- | --- |
| `id` | TEXT | — | **PK** — the unique instance id (key anti-dup primitive, §8) |
| `inventory_id` | TEXT | — | NOT NULL · **FK → inventory(id) ON DELETE CASCADE** |
| `item_id` | TEXT | — | NOT NULL · **FK → item_template(id)** |
| `slot_index` | INT | — | NOT NULL · CHECK ≥ 0 |
| `quantity` | INT | 1 | NOT NULL · CHECK > 0 |
| `durability` | FLOAT | 1.0 | nullable · CHECK NULL or ≥ 0 |
| `is_equipped` | BOOLEAN | FALSE | NOT NULL |
| `created_at` / `updated_at` | TIMESTAMP | NOW() | NOT NULL |

- **UNIQUE (inventory_id, slot_index)** → one stack per slot (anti-dup, §8).
- Indexes: `idx_inventory_item_inventory(inventory_id)`, `idx_inventory_item_item_template(item_id)`.

---

## 3. Data-model mapping (table ↔ Go struct ↔ proto)

All three map 1:1; the proto omits the timestamps. `sql.NullFloat64` ↔ proto `double` via `nullFloat64`
(NULL → 0 on the wire).

| Entity | table column | Go (`postgres.*`) | proto (`apeironv1.*`) |
| --- | --- | --- | --- |
| **Inventory** | id/owner_type/owner_id/inventory_type | ID/OwnerType/OwnerID/InventoryType | id/owner_type/owner_id/inventory_type |
| | max_slots/max_weight/current_weight/is_locked | MaxSlots(int)/MaxWeight(NullFloat64)/CurrentWeight/IsLocked | max_slots(int32)/max_weight(double)/current_weight/is_locked |
| **ItemTemplate** | id/name/description/item_type/rarity | ID/Name/Description/ItemType/Rarity | same |
| | max_stack/weight/is_tradable/is_destroyable/base_value | MaxStack(int)/Weight/IsTradable/IsDestroyable/BaseValue(int64) | max_stack(int32)/weight/is_tradable/is_destroyable/base_value(int64) |
| **InventoryItem** | id/inventory_id/item_id/slot_index | ID/InventoryID/ItemID/SlotIndex(int) | id/inventory_id/item_id/slot_index(int32) |
| | quantity/durability/is_equipped | Quantity(int)/Durability(NullFloat64)/IsEquipped | quantity(int32)/durability(double)/is_equipped |
| **InventoryWithItems** | (join) | Inventory + []InventoryItem | inventory + repeated items |

---

## 4. gRPC service surface — what goes IN / OUT

`InventoryDataService` — **READ-ONLY today** (no write RPCs; mirrors how PlayerDataService was before
the progression work added `UpdatePlayer`).

| RPC | IN | OUT |
| --- | --- | --- |
| `GetInventory` | `IdRequest{id}` | `InventoryResponse{found, inventory, error}` |
| `GetInventoryByOwner` | `InventoryOwnerRequest{owner_type, owner_id, inventory_type}` | `InventoryResponse` |
| `GetInventoryWithItems` | `IdRequest{id}` | `InventoryWithItemsResponse{found, InventoryWithItems{inventory, items[]}, error}` |
| `GetItemTemplate` | `IdRequest{id}` | `ItemTemplateResponse{found, item_template, error}` |
| `ListItemTemplatesByType` | `IdRequest{id = item_type}` | `ItemTemplatesResponse{item_templates[], error}` |
| `GetItem` | `IdRequest{id}` | `InventoryItemResponse{found, item, error}` |
| `GetItemsByInventory` | `IdRequest{id = inventory_id}` | `InventoryItemsResponse{items[], error}` |

Pattern: every response carries `found`/`error` (never a gRPC error for not-found — returns `found=false`).

---

## 5. Repository surface (read + write that already exists)

The repo has the full DB operation set; the **writes are NOT yet exposed via the service** (the handler
interface only wires the 7 reads). Building the inventory runtime = exposing these behind transactional
write RPCs (see §8/§9).

**Inventory:** `GetInventoryByID`, `GetInventoryByOwner`, `CreateInventory`, `UpdateInventoryWeight`,
`SetInventoryLocked`.
**ItemTemplate:** `GetItemTemplateByID`, `ListItemTemplatesByType`, `CreateItemTemplate`.
**InventoryItem:** `GetItemByID`, `GetItemsByInventoryID`, `GetInventoryWithItems`, `CreateItem`,
`UpdateQuantity`, `MoveSlot`, `UpdateDurability`, `SetEquipped`, `DeleteItem`.

All writes are single-row `UPDATE/INSERT/DELETE ... WHERE id = $` — **unconditional** (no version/CAS, no
multi-row transaction wrapper at this layer yet). That is the seam where anti-dup hardening goes (§8).

---

## 6. Game-server side

- `server-apeiron/internal/systems/inventory_system.go` — **a stub**: `NewInventorySystem(fn)` registers
  a `System` named "inventory" in `clock.PhasePersistence`. There is **no inventory logic in the runtime
  yet** (no load-on-attach, no equip, no weight calc). The persistence phase is where periodic inventory
  flush will live.
- The server's `dbapeiron` client exposes the InventoryDataService client (read), like `Players`.
- **Planned (mirror the progression build):** load the player's backpack + equipment on attach (read
  RPCs exist); add **write RPCs** for mutations; recompute carried weight → encumbrance each relevant
  tick; persist in the persistence phase. The roadmap's slices cover this.

---

## 7. End-to-end data flow

**Read (exists):**
`Postgres tables → InventoryRepository (SQL + scan into postgres.* models) → InventoryDataHandler
(map* → proto) → InventoryDataService (gRPC) → server dbapeiron client → game runtime → snapshot →
Unreal client.` Item templates are static → served through `item_cache`.

**Write (to build):**
`game runtime (validated intent) → new inventory write RPC (transactional) → InventoryRepository writes
→ Postgres.` The DB is the **single source of truth**; the runtime holds a working copy and persists
through the service.

---

## 8. Anti-duplication & integrity rules

Item dupe exploits are the classic MMO economy killer. The model already has good primitives; the rules
below are what keep it safe (★ = already enforced by the schema; ☆ = must be built into the write path).

- ★ **Items are unique instances.** `inventory_item.id` is a PK — every item is a tracked row, not a free
  count. **Instance ids are generated server-side only** (never by the client). ☆ enforce server-gen.
- ★ **One stack per slot.** `UNIQUE (inventory_id, slot_index)` makes a slot collision impossible — you
  cannot write two items into the same slot to dupe.
- ★ **Referential integrity.** `inventory_id` FK (ON DELETE CASCADE) and `item_id` FK mean items can't
  point at a non-existent inventory/template; deleting an inventory cascades its items (no orphans).
- ★ **Value bounds.** CHECKs (`quantity > 0`, `max_stack ≥ 1`, weights ≥ 0) stop negative/zero-stack
  tricks at the DB.
- ☆ **Server-authoritative mutations.** Clients send *intents* (pick up, move, equip, split, trade); the
  server validates against current DB state and applies. No client-set quantities or client-created rows.
- ☆ **Atomic multi-step ops = one transaction.** Move, equip, split, merge, **loot**, and **trade** must
  be all-or-nothing in a single DB transaction. Never "add to destination, then remove from source" as
  two calls — a crash/disconnect between them dupes. Always remove+add in the same tx.
- ☆ **Trade/transfer locks both inventories.** Use `inventory.is_locked` to freeze both sides during a
  trade/transfer so concurrent operations can't interleave and dupe; unlock on commit/rollback.
- ☆ **Idempotency on commands.** Pickup/move/trade commands carry a client `command_id`/`sequence`; the
  server rejects duplicates/replays (same pattern as the game runtime's command-replay state) so a network
  retry never grants an item twice.
- ☆ **Optimistic concurrency.** Today writes are unconditional `WHERE id=$`. Add a version (or use
  `updated_at` as a CAS guard) so two concurrent edits to the same item can't both succeed and dupe.
- ☆ **Stack conservation.** Split/merge must conserve total quantity (Σ before = Σ after); validate, and
  do it in one tx.
- ☆ **Loot atomicity.** Removing an item from a loot container and adding it to the player happen in one
  tx; the loot row is deleted in the **same** tx it is granted (no grant-then-delete window).
- ☆ **Equip = move, not copy.** Equipping changes an instance's `inventory_id`/`is_equipped`/`slot_index`
  — it never creates a second instance.
- **No accumulation forever** (design, see roadmap §6): durability decay destroys perishables over time,
  which also caps how much value can be hoarded/duped-then-stockpiled.

---

## 9. Gaps vs the design (what must be built)

- **Write RPCs** on InventoryDataService (add/remove/move/equip/unequip/split/transfer) — transactional,
  idempotent — the service is read-only today.
- **Equipment slots, bag capacity, durability columns** — `item_template.equip_slot`, `weapon_kit_id`,
  `bag_slots`/`bag_weight`, `equip_stats`, `durability_mode`/`max_durability`/`durability_per_use`/
  `decay_per_hour`/`is_repairable`; `inventory_item.acquired_at` (roadmap §7).
- **Runtime:** load on attach, equip command, weight→encumbrance, pickup, durability tick — the server
  system is a stub.
- **Anti-dup hardening:** transactions around multi-step ops, idempotency keys, version/CAS, `is_locked`
  usage in trades.
- **Snapshot publishing** of equipment/backpack/weight for the Unreal HUD.

See `aaa-player-inventory-equipment-roadmap.md` for the design + slices that fill these.
