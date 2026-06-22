# Recuperacao 05 - Skill Movement Authority Consolidation

Thread: `019ed913-f0c2-7960-914d-c3d4ec407072`

## Fonte Extraida Do Chat

- Problema estrutural:
  - dois produtores de locomotion para skill:
    - movement resolver;
    - combat system manual publisher.
- Campos que precisam ser consistentes:
  - `Action`
  - `MovementMode`
  - `Phase`
  - `ReconciliationMode`
  - `TargetSpeed`
  - `EffectiveSpeed`
  - `PhaseSpeedScale`
  - `ActionDistanceTraveled`
  - `ActionProjectedPosition`
  - `ActionStartPosition`
- Decisao arquitetural:
  - Combat system e dono da skill.
  - Movement resolver e dono da locomotion.
  - Combat decide inicio, timing, alvo, direcao, hitbox, cooldown, lock, pending state.
  - Movement decide/publica locomotion state final.
- Correcoes esperadas:
  - remover publisher manual paralelo em combat.
  - `CommitMovementResult` nao pode resolver pelo movement e depois sobrescrever snapshot.
  - startup/recovery ainda publicam estado da action, mas via `movement`.
  - testes devem provar paridade dos campos acima.

## Invariantes

- Um unico produtor de `LocomotionState` para action movement.
- Combat nunca escolhe `PhaseSpeedScale`, `ReconciliationMode`, `TargetSpeed` ou `EffectiveSpeed`.
- Recovery de grounded skill sem velocidade deve publicar escala zero, nao minimo artificial.

## Status Atual A Validar

- Auditar `internal/combat` e `internal/movement` no server atual.
- Confirmar que nao voltou publisher manual duplicado.
- Adicionar teste se a paridade dos campos ainda nao estiver coberta.

