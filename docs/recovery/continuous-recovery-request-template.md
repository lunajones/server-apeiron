# Continuous Apeiron Recovery Request Template

Use this prompt when continuing reconstruction in a new Codex chat:

```text
Continue Apeiron recovery from the deletion incident.

Projects:
- Unreal: B:\Unreal Projects\PlainTestMap
- server: C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron
- db: C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\db-apeiron

Use the Apeiron skills:
- apeiron-aaa-roadmap-governor
- apeiron-workspace-ops
- apeiron-authoritative-movement-debug
- apeiron-skill-movement-audit
- apeiron-db-contract-seeding
- apeiron-creature-combat-ai
- apeiron-reconciliation-change-ledger when debugging rubberbanding

Hard rules:
- No destructive deletes or cleanup commands.
- Do not trust "it compiles" as "it is restored".
- Do not leave gameplay fallbacks as final behavior.
- DB/profile contracts are authoritative for shared tuning.
- If a recovered file says legacy/fallback/compat/recovered, audit it before treating it as final.
- Prefer the newest Codex recovery thread decision when sources conflict.
- If a required old file is missing, reconstruct it from current code, Unreal protocol, DB API needs, and the recovery thread decisions.

Source docs to read first:
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\recovery\codex-chat-roadmaps\thread-source-index-2026-06-22.md
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\recovery\reconstruction-gap-audit-2026-06-22.md
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\recovery\codex-chat-roadmaps\consolidated-reconstruction-roadmap.md
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\recovery\chronological-chat-reconstruction-ledger-2026-06-22.md
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\roadmap\server-apeiron-aaa-movement-action-contract-roadmap.md
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\roadmap\aaa-skill-movement-contract-roadmap.md
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\roadmap\aaa-movement-rubberband-regression-roadmap.md
- C:\Users\elmir\OneDrive\Documentos\Projetos\mmo\server-apeiron\docs\roadmap\aaa-temporal-melee-hit-volume-roadmap.md

Current highest-priority suspected holes:
1. Restore movement resolver ownership. Combat owns skill intent/timeline; movement owns locomotion publication. Eliminate duplicate locomotion writers.
2. Make recovered runtime fallbacks explicit recovery/dev only. In normal runtime, missing DB contracts must fail loudly.
3. Remove final gameplay dependency on fallback player attack profiles. Basic attack alias must resolve to DB-backed combo stages.
4. Audit `gameapi/runtime.go` versus combat/domain runtime. If it is a temporary vertical slice, isolate it; if it is real runtime, move duplicated action/movement logic into proper packages.
5. Revalidate creature brain/action runtime parity, wolf lunge damage/timing/pass-through/landing inertia, hitbox target resolution, directional block, impact response profile, combat mode HUD, and temporal melee hit volumes.

Process:
1. Read the source docs above.
2. Re-read relevant Codex recovery threads from `thread-source-index-2026-06-22.md` when a decision is unclear.
3. Audit code before changing it.
4. List all competing root-cause hypotheses/gaps together before fixing systemic movement/combat bugs.
5. Apply the smallest architecture-correct change that removes duplicate authority.
6. Add or update tests that would have caught the deletion/regression.
7. Run server/db tests and Unreal build when touched.
8. Commit safe checkpoints to Git after validation.

Goal:
Recover Apeiron to the latest pre-deletion gameplay state: smooth authoritative movement, AAA dodge/leap/turn, skill movement without rubberband, sword/shield basic combo/F/R, wolf behavior, DB-driven contracts/seeds/protos, temporal hitboxes, combat modes/HUD, and no silent hardcoded gameplay fallbacks.
```

