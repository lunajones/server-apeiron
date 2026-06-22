# Recuperacao 06 - Action Locks During Dodge/Leap

Thread: `019ed02a-86f2-79d2-bcd6-0a479bd27b81`

## Fonte Extraida Do Chat

- Problema:
  - player conseguia usar skills/basic attack durante pulo e dodge.
- Regra correta:
  - pulou ou dodgeou, nao pode usar nenhuma skill nem basic attack ate terminar a action/estado autoritativo.
- Implementacao original:
  - server gate bloqueia `BASIC_ATTACK` e `CAST_SKILL` quando movement atual e `dodge`, `leap`, `knockback` ou `movement_mode=airborne`.
  - lock reason esperado: `active_locomotion`.
- Validacao original:
  - leap aceito e basic imediatamente depois negado;
  - dodge aceito e basic imediatamente depois negado.

## Status Atual A Validar

- Conferir gate atual no server reconstruido.
- Garantir teste cobrindo basic/cast durante dodge/leap/airborne.
- Garantir que cliente tambem bloqueia visualmente, mas sem substituir server authority.

