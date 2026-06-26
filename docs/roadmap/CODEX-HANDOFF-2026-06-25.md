# Codex Handoff — 2026-06-25

Index of the AAA roadmaps worked this session, their status, and the build order. Read this first.

## Docs to review (worked this session)

| Doc | What it is | Status |
| --- | --- | --- |
| `aaa-action-orientation-and-lunge-envelope-roadmap.md` | Body/focus/attack yaw separation, lunge envelope, attack-yaw latch, takeoff re-aim | **Implemented + pushed.** Creature side complete; player generalization structural only (player wiring/publish + prediction tuning = PIE). |
| `aaa-creature-action-transition-runtime-roadmap.md` | Action -> tactical-movement handoff (no snap) | **Server ~complete (Codex), PIE pending.** Annotated with 3 latch coupling points + status audit. |
| `aaa-threat-aggro-runtime-roadmap.md` | Who a creature fights: threat table, selection, leash | **Design done (pass A). Implementation in progress — see Build Order.** |
| `aaa-pack-coordination-runtime-roadmap.md` | Group authority: surround, commit budget, role rotation | **Design done (pass A). Implementation pending.** |

## Corrupted docs (winfr byte-loss — names only, ignore content)

`aaa-movement-rubberband-regression-roadmap.md`, `server-apeiron-aaa-movement-action-contract-roadmap.md`,
`aaa-skill-movement-contract-roadmap.md`, `aaa-temporal-melee-hit-volume-roadmap.md`,
`temp-reconciliation-contract-migration-roadmap.md` — these are binary garbage. Their topics are
already covered by existing code/tables; do not try to read them.

## Build order — DONE (server) except PIE tuning

```
Threat S1-S5  (table, selection, proximity/leash, focus aggregate)   [DONE, tested]
Pack   S1-S5  (formation, slotting, commit budget, rotation, focus)  [DONE, tested]
   v
Tuning in PIE (cadence, spacing, decay)   [REMAINING - yours/Codex, needs the game running]
```

All slices implemented server-side in `threat.go` / `pack.go`, data-driven from the wolf behavior
contract metadata (`ThreatRuntimeProfile`, `PackRuntimeProfile`), 17 unit tests green, single-player
and pack-of-one no-regression guarantees tested. What remains is PIE feel tuning: commit cadence
(`commitTokenCooldownMs`), surround spacing, threat decay/switch — tune in `bootstrap/016_wolf_behavior_contract_seed.sql`
metadata and restart, no code needed. Suggested PIE check: spawn 3 wolves, confirm they surround,
only one commits at a time, turns rotate, and pulling far resets them.

## Known test state

`go test ./internal/gameapi/` has pre-existing failures from prior Codex commits — NOT regressions:
- 3 coverage tests: fake-data gaps (wolf turn rate, combat-core stamina).
- 2 behavior tests are intentionally stale (encode pre-transition behavior): `TestWolfLungeActivePhaseUsesSkillRootMotionOwner` (lunge is now a vertical leap) and `TestWolfMaulContactStopsBeforeOverlappingTarget...` (maul is now a target-carry). Update these to the new behavior once feel is confirmed in PIE.

New work in this session adds passing unit tests and must not increase the failure count.
