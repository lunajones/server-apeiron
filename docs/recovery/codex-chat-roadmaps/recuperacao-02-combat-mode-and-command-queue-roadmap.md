# Recuperacao 02 - Combat Mode Switch, Command Queue, Leap Handoff

Thread: `019ee6b1-dcb7-7742-b3d4-439b8a8bf0ad`

## Fonte Extraida Do Chat

- Problemas:
  - `CTRL` nao trocava combat mode corretamente.
  - `F`/skill alterava a barra visual por efeito colateral.
  - pulo ficava preso por fila de movimento e handoff de aterrissagem.
- Correcoes aplicadas no chat original:
  - `/submit-switch-combat-mode` deve passar por `ApplyCommandAckResponse`.
  - HUD nao pode usar `resolved_combat_mode` de skill rejeitada como modo ativo.
  - Com `combat_mode_enforced=true`, Q/R/F nao podem cair em fallback local.
  - fila HTTP deve coalescer `move/turn` antigos.
  - `switch_combat_mode`, `leap`, `dodge`, `cast` tem prioridade sobre spam de movimento.
  - move com `handoff_action=leap` deve ser prioridade.
  - handoff de landing deve ser flushado antes de novo pulo/dodge.

## Invariantes

- Combat mode ativo vem do server.
- HUD mostra slots do modo ativo confirmado.
- Se o slot nao existir no modo atual, fica vazio.
- Comando de acao nao pode ficar preso atras de fila de locomotion continua.

## Status Atual A Validar

- Conferir `ApeironGameServerBridge` para ACK de switch/leap.
- Conferir fila de requests para prioridade/coalescing.
- Conferir HUD view-model para fallback de mode/slots.
- Conferir teste automatizado cobrindo `CTRL` e leap handoff.

