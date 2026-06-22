# Consolidated Reconstruction Roadmap From Codex Chats 1-13

Data: 2026-06-22

Este roadmap substitui roadmaps recuperados corrompidos quando houver conflito. Ele foi reconstruido do historico dos chats `recuperacao 1` a `recuperacao 13`.

## Fase 1 - Movement/Skill Movement Autoritativo

- Confirmar um unico produtor de `LocomotionState`: movement resolver.
- Remover/impedir publisher manual paralelo de combat para locomotion.
- Reconciliation engine global com profiles por categoria.
- `movement_action_contract` referencia reconciliation profile adequado.
- Action channels/policies devem vir de DB quando virarem fonte primaria.
- [x] 2026-06-22: runtime vivo deixou de teleportar grounded skill no `SubmitCommand`.
  Skill movement agora cria `actionMotion` autoritativa e progride por snapshot/tick usando
  `internal/movement.ResolveActionMotionProgress`.
- [x] 2026-06-22: `MOVE` durante owned root respeita `NormalInputPolicy`
  (`blocked_during_owned_root` / `buffer_until_recovery_handoff`) e nao rouba a posicao da skill.
- [x] 2026-06-22: segunda skill durante grounded owned root agora recebe `action_locked`
  ate o handoff completar, evitando substituir a action motion autoritativa em andamento.
- [x] 2026-06-22: wolf nao publica `SkillRuntimeState` a cada orbit/chase; skill runtime da
  criatura so fica ativo em acao comprometida e preserva `StartedAtMs` ate o fim da janela.
- Testes:
  - paridade dos campos de locomotion;
  - dodge/leap/airborne bloqueiam skill/basic;
  - command queue prioriza action sobre movement spam;
  - leap ACK/handoff nao prende player.
  - grounded skill nao teleporta no cast, progride por snapshot e mantem root-motion ownership.

## Fase 2 - Combat Mode/Weapon Kit/HUD

- Combat mode ativo vem de ACK/snapshot.
- `CTRL` troca modo.
- HUD mostra slots do modo ativo confirmado.
- Sem fallback local para Q/R/F quando `combat_mode_enforced=true`.
- Bulwark atual:
  - M1 basic combo;
  - R shield bash;
  - F shield rush;
  - Q vazio por enquanto.
- Vanguard atual:
  - M1 basic combo;
  - demais vazios ate existirem skills reais.

## Fase 3 - Hitbox/Defense/Damage Runtime

- [x] Hitbox por skill, nao target lock.
- [x] Single-target escolhe alvo por profundidade na direcao do ataque.
- [x] Multi-target gera multiplos outcomes pelo runtime de hitbox e aplica `max_targets` por hitbox/skill.
- [x] Block so funciona no arco frontal quando `CombatDefenseContract` esta presente.
- [x] Parry resolvido na pipeline autoritativa de dano/defesa.
- [x] `CombatDefenseContract` exposto no DB API por proto/cache/repository/handler.
- [x] Impact response profile por alvo:
  - player `flesh_blood_red`;
  - creature por entity type (`creature_flesh_blood_red`);
  - evento de dano carrega profile.
- [ ] Evoluir impact response de creature para profile por template quando o template runtime estiver disponivel diretamente no entity/provedor.
- [ ] Conectar `CombatDefenseContract` no provider real de `AttackProfile`; o `AttackProfile` ja carrega o campo, mas o provider separado nao existe no server recuperado atual.

## Fase 4 - Creature Brain/Action Runtime

- Wolf lunge:
  - moving windup;
  - airborne passthrough;
  - post landing inertia;
  - nao encerrar skill antes do movement completo.
- [x] 2026-06-22: eliminada reinicializacao constante de `SkillRuntimeState` no wolf durante
  orbit/chase, que fazia o cliente tratar lunge/dodge visual como sempre recem-iniciado.
- Creature usa hitbox/movement contracts como player.
- Creature dodge/leap/maul respeitam stamina/action runtime.

## Fase 5 - Visual/HUD/VFX

- HUD visual final baseado em source-of-truth Figma.
- Skills usam icons/slots com identidade Apeiron.
- Windup bar solta e limpa.
- Niagara wolf lunge dust usa marrom quente natural e fragmentos curtos/picotados.

## Fase 6 - Recovery Hardening

- Roadmaps de chat devem ficar versionados.
- Docs corrompidos nao sao fonte primaria.
- Tests devem falhar quando seed/contrato obrigatorio sumir.
- Nunca depender de fallback silencioso como runtime final.
