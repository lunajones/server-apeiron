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

## Build order (in progress)

```
Threat S1-S4  (table, selection, proximity/leash)   <- foundation, solo-testable  [STARTED]
   v
Pack S1-S4    (formation, slotting, commit budget, rotation)
   v
Threat S5 + Pack S5  (multi-target focus together)
   v
Tuning in PIE (cadence, spacing, decay)
```

Threat goes first: it owns "which target", which the pack consumes (pack Slice 5 depends on threat
Slice 5), it is lower-risk with a single-player no-regression guarantee, and it improves the solo
creature (leash/reset) testably now.

## Known test state

`go test ./internal/gameapi/` has pre-existing failures from prior Codex commits — NOT regressions:
- 3 coverage tests: fake-data gaps (wolf turn rate, combat-core stamina).
- 2 behavior tests are intentionally stale (encode pre-transition behavior): `TestWolfLungeActivePhaseUsesSkillRootMotionOwner` (lunge is now a vertical leap) and `TestWolfMaulContactStopsBeforeOverlappingTarget...` (maul is now a target-carry). Update these to the new behavior once feel is confirmed in PIE.

New work in this session adds passing unit tests and must not increase the failure count.
