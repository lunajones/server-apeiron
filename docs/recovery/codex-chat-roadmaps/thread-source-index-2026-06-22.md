# Apeiron Codex Thread Source Index - 2026-06-22

This file tracks the Codex recovery threads used as reconstruction sources. It is not a replacement for code review; it is the source map for rebuilding Apeiron from chat decisions after the project deletion incident.

## Read Order

Newest-to-oldest thread labels are the user's recovery naming. When rebuilding, prefer newer decisions over older ones unless the newer thread explicitly reverted the design.

| Label | Thread ID | Status | Primary Themes | Notes |
| --- | --- | --- | --- | --- |
| recuperacao 1 | `019ee718-0dbd-7791-b75e-32015f3ca5d8` | partial read | reconciliation architecture, HUD source of truth | Defines reconciliation families: `locomotion_default`, `evasive_burst`, `airborne_commit`, `grounded_contact_rush`, `short_grounded_impulse`, `micro_lunge_commit`. |
| recuperacao 2 | `019ee6b1-dcb7-7742-b3d4-439b8a8bf0ad` | partial read | combat mode CTRL, command queue, leap handoff, local action prediction | Critical: `/submit-switch-combat-mode` ACK was not consumed by Unreal; local fallback mode must not override authoritative state. |
| recuperacao 3 | `019edb84-a5ad-7693-8bab-38ddd3b27363` | partial read | sword/shield skill concepts, shield rush VFX identity | Source for non-cliche sword/shield skill identity and stylized grass/dust direction. |
| recuperacao 4 | `019ed28f-5e82-76d0-9fd7-0a5d9266dffe` | partial read | Codex skills, roadmap governor, workspace ops | Source for process skills and "best long-term solution, not patch" rule. |
| recuperacao 5 | `019ed913-f0c2-7960-914d-c3d4ec407072` | read | skill movement authority, rubberband root cause | P0 architecture: combat owns skill intent/timeline; movement owns locomotion publication. No dual publisher. |
| recuperacao 6 | `019ed02a-86f2-79d2-bcd6-0a479bd27b81` | partial read | basic attack combo, skill lock during leap/dodge, bridge basic attack mapping | Basic attack input is alias; server resolves `_1/_2/_3`. Skills are blocked during leap/dodge/airborne. |
| recuperacao 7 | `019ed2eb-8538-7a11-aef6-d0a51dd1fcbc` | read | wolf lunge pre-run, leap, landing carry | Lunge must keep motion alive through full movement window, including post-landing inertia. |
| recuperacao 8 | `019ec2b0-efc6-76d2-8905-b4a7469c3d65` | partial read | parry, movement regression, VFX/Niagara, ground movement facing | Contains key fix idea: grounded skill movement uses horizontal character facing/contract distance, not mouse ground point. |
| recuperacao 9 | `019ec6ae-6b0f-7441-8023-a4375f9b6e5b` | read | hitbox vs target lock | AI target is aim/decision only; hitbox resolves actual hit target. Single target chooses first valid target along attack direction. |
| recuperacao 10 | `019ec622-0b0d-7713-a69a-91c548dc9295` | read | directional block | Block applies only inside frontal arc. Back arc hit must bypass block. |
| recuperacao 11 | `019e97f2-1f69-7222-a875-ff1fa9bf074b` | partial read | movement action contract roadmap, DB/server split, command sequence, action channels | DB stores static contracts/policies; server owns runtime state, sequence, anti-replay, state machine, channel occupancy. |
| recuperacao 12 | `019e976a-049c-7c53-80b5-3284588fb2f8` | partial read | impact response profile, target material VFX | Target material/profile must combine with impact type. Player default is `flesh_blood_red`; creature template can be `bone_dust`, `stone_chips`, etc. |
| recuperacao 13 | `019e92d3-e7d9-79d1-89c7-b5eb11b98cd8` | read | authoritative movement, desired position, leap route, snapshot correction, despawn guard | `desired_position` is prediction evidence only. Snapshot movement correction must smooth normal error and snap only severe error. |

## Source Priority Rules

1. Use newest concrete implementation decision when two threads conflict.
2. Prefer architecture decisions that eliminate duplicate authority over field-by-field patches.
3. Treat recovered fallback code as temporary unless the thread explicitly says it is final.
4. If a recovered file contradicts a thread and has "legacy", "fallback", "compat", or "recovered" markers, assume the thread is closer to the intended final design.
5. If a thread references a file that is missing today, record the missing file as a reconstruction gap before inventing a replacement.

## Next Threads To Page Further

- `recuperacao 1`: page older movement/HUD decisions.
- `recuperacao 2`: page older combat mode queue and prediction details.
- `recuperacao 3`: page skill concepts before VFX-only tail.
- `recuperacao 8`: page older parry and movement rubberband debugging.
- `recuperacao 11`: page older DB movement action roadmap requirements.
- `recuperacao 12`: page older impact VFX and profile propagation implementation.

