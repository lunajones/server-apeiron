# ⛔ DO NOT READ — corrupted/byte-lost files

**Instruction to any AI agent (Claude, Codex, or other): do NOT open, read, parse, summarize, or
reference any file in this directory. Skip it entirely.**

These `.md` files were **byte-corrupted during the 2026-06-22 source recovery** (the recovery picked up
unrelated bytes — JS sourcemaps, UTF-16 binary, null bytes — for these filenames). Their content is
**garbage, not the intended roadmaps**, and is **unrecoverable** (corrupted in git history too).

They are quarantined here only so the original filenames are preserved for reference. Do not treat them
as design docs.

## Files here (all corrupted, ignore)
- `aaa-skill-movement-contract-roadmap.md` — null bytes
- `aaa-temporal-melee-hit-volume-roadmap.md` — a JS sourcemap, wrong content
- `server-apeiron-aaa-movement-action-contract-roadmap.md` — UTF-16 binary garbage

## If their topic is needed
The live, readable docs for movement/action live in the parent `docs/roadmap/` directory:
- `aaa-creature-action-transition-runtime-roadmap.md` (action transition phases runtime)
- `aaa-action-orientation-and-lunge-envelope-roadmap.md` (action aiming/orientation)
- `aaa-movement-rubberband-regression-roadmap.md` (rebuilt from live diagnosis, 2026-06-28)

If a corrupted topic above has no live equivalent, its content is **lost** — reconstruct from the code,
do not attempt to read these files.
