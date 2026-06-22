# Recuperacao 12 - Impact Response Profile

Thread: `019e976a-049c-7c53-80b5-3284588fb2f8`

## Fonte Extraida Do Chat

- Objetivo:
  - diferenciar resposta visual/material ao impacto por alvo.
- Regra:
  - player default `flesh_blood_red`;
  - creature vindo de template/entity;
  - evento de dano deve carregar perfil para VFX nao depender de snapshot timing.
- Implementacao original iniciou:
  - coluna `impact_response_profile` em creature template;
  - proto DB;
  - mapper DB;
  - server domain entity/spawn/snapshot/events;
  - proto game snapshot.
- O thread foi interrompido durante geracao de protos.

## Status Atual A Validar

- Conferir se isso existe no DB/server atual. Provavelmente perdido na recuperacao.
- Se ausente, recriar como feature completa:
  - DB migration/seed;
  - DB proto/mapper/repository;
  - server proto/domain/snapshot/event metadata;
  - Unreal bridge/VFX selection;
  - testes.

