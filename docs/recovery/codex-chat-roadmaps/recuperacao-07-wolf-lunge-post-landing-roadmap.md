# Recuperacao 07 - Wolf Lunge Post Landing Inertia

Thread: `019ed2eb-8538-7a11-aef6-d0a51dd1fcbc`

## Fonte Extraida Do Chat

- Problema:
  - wolf fazia movimento antes do salto, mas nao continuava em linha apos pouso.
- Diagnostico original:
  - contrato de lunge estava fora da timeline real da action;
  - movimento iniciava em `1840ms`/takeoff `2000ms`, enquanto action terminava perto de `1080ms`;
  - server podia encerrar skill antes da parte de movimento esperada.
- Correcoes esperadas:
  - lunge movement cabe dentro da timeline real;
  - skill pendente permanece ate fim do movimento, nao so fim da hitbox;
  - Unreal nao limpa visual antes da janela completa de movement terminar;
  - `landing_lock_ms` funciona como continuidade horizontal, nao pausa.

## Status Atual A Validar

- DB atual usa novo modelo `movement_action_contract` e behavior setup policies; checar equivalencia com esse requisito.
- Confirmar `lunge` tem moving windup, airborne phase e post-landing inertia.
- Confirmar testes de creature combat cobrem fim do movimento.

