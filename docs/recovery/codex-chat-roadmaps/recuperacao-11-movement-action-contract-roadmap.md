# Recuperacao 11 - Movement Action Contract Roadmap

Thread: `019e97f2-1f69-7222-a875-ff1fa9bf074b`

## Fonte Extraida Do Chat

- Roadmap `server-apeiron-aaa-movement-action-contract-roadmap.md` deveria conter:
  - contexto autoritativo sem confiar no client;
  - cache/reload/falha do DB;
  - manifest/sync de contrato com Unreal;
  - `command_id`, `client_tick`, `client_action_sequence`;
  - dedupe/replay protection;
  - rejeicao de comando atrasado, futuro, duplicado ou fora de ordem;
  - limite para action start retroativo;
  - state machine formal;
  - action channels;
  - runtime state autoritativo;
  - ACK/snapshot com contrato, tick e phase;
  - snapshot timeline / late snapshot handling;
  - correction debt por contrato;
  - lag compensation para iframe/action windows;
  - terrain/nav policy;
  - creatures/AI usando o mesmo runtime;
  - turning como primeira classe;
  - rollout/rollback;
  - readiness checks;
  - observabilidade;
  - testes com RTT/jitter/loss/reorder;
  - determinismo por server tick.
- Ressalva:
  - Action channels devem vir de contrato/policy carregado do DB quando existir.
  - fallback hardcoded de canais so em dev/compat, nunca fonte primaria.

## Status Atual A Validar

- O doc original esta corrompido/recuperado; este arquivo passa a ser fonte reconstruida.
- Conferir se `movement_action_contract` atual cobre action channels ou se falta coluna/policy.

