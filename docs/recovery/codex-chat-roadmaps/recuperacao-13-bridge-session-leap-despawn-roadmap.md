# Recuperacao 13 - Bridge Session, Leap ACK, Creature Despawn Grace

Thread: `019e92d3-e7d9-79d1-89c7-b5eb11b98cd8`

## Fonte Extraida Do Chat

- Problemas:
  - sessao morrendo/bridge instavel;
  - creature sumindo por full snapshot temporario sem placeholders;
  - `/submit-leap` nao era tratado como rota com `CommandAck`.
- Regra correta:
  - visual da creature nao deve sumir sem evento autoritativo de despawn se runtime ainda indica entidade viva.
  - full snapshot com `creature_placeholders=0` so aplica despawn grace se runtime concordar que nao ha entidade extra viva.
  - `/submit-leap` deve passar por ACK handler como outros comandos de action.

## Status Atual A Validar

- Conferir `ApeironGameServerBridge.cpp` atual:
  - `/submit-leap` em `ApplyCommandAckResponse`;
  - despawn grace considera runtime active entities.
- Conferir testes/log scanner para regressao.

