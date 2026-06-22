# Recuperacao 09 - Hitbox Target Selection And Multi Target Outcomes

Thread: `019ec6ae-6b0f-7441-8023-a4375f9b6e5b`

## Fonte Extraida Do Chat

- Pergunta base:
  - wolf/player hit deve usar hitbox por skill, nao target lock.
- Regra definida:
  - em single-target, B so toma no lugar de A se estiver dentro da hitbox e antes no caminho do ataque.
  - area/multi-target pode acertar varios.
- Implementacao esperada:
  - `HitResult` tem profundidade/distancia projetada na direcao do ataque.
  - `MaxTargets=1` ordena por distancia projetada/forward, nao distancia radial generica.
  - creature combat retorna multiplos `AttackOutcome` quando a skill permitir.
- Testes esperados:
  - runtime de hitbox para interceptacao single-target;
  - combat system de criatura para area/multi-target.

## Status Atual A Validar

- Conferir `internal/hitbox` atual.
- Conferir se `ForwardDistance`/equivalente existe.
- Conferir se creature/player combat nao voltaram a retornar so primeiro hit indevidamente.

