# Recuperacao 10 - Directional Block Arc

Thread: `019ec622-0b0d-7713-a69a-91c548dc9295`

## Fonte Extraida Do Chat

- Problema:
  - player bloqueava dano mesmo de costas para a creature.
- Regra correta:
  - block so funciona no arco frontal.
  - fallback sem policy explicita: `180` graus, 50% frontal de 360.
  - hit pelas costas atravessa block e aplica dano.
- Implementacao esperada:
  - pipeline autoritativa usa `blocking && insideBlockArc`.
  - posture pressure fora do arco nao pode manter HP damage bloqueado.
- Testes esperados:
  - block de frente funciona;
  - block de costas nao funciona;
  - guard break setup usa geometria explicita.

## Status Atual A Validar

- Conferir `internal/combat/pipeline.go`.
- Conferir defense contracts no DB (`BlockFrontArcDeg`).

