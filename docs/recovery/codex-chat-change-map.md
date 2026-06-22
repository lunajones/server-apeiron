# Mapa Cronológico de Mudanças dos Chats do Codex

Doc vivo — iniciado em 2026-06-22. Cresce conforme novos chats forem colados (esta leva cobre
os chats de **04/06 a 14/06**; faltam ainda `recuperação 7 → 1`, que serão apêndados aqui).

## Objetivo

Agrupar, em ordem cronológica (mais antigo → mais novo), **todas** as alterações que foram feitas
nos chats do Codex ao longo do tempo, e cruzá-las com o **código atual** (o scaffold que a
recuperação reconstruiu *depois* do delete). Para cada mudança registramos:

- o que foi feito e por quê (na voz do que o usuário pediu / o Codex entregou);
- os arquivos e linhas **originais** citados no chat (referência histórica — esses caminhos podem
  não existir mais hoje);
- o **status atual** no código recuperado: presente, sumiu ou parcial;
- onde re-aplicar, quando o arquivo original sumiu mas a lógica deve voltar.

> ⚠️ Importante sobre git: o repositório git destes projetos **só existe a partir da recuperação**,
> criado *depois* do delete. Não há nenhuma linha do jogo pré-delete no histórico git. Este doc é a
> fonte de verdade do "o que existia antes", não o `git log`.

## Como usar / precedência de fontes

1. Relato direto do usuário no chat + valor testado em runtime.
2. Devolutivas limpas geradas na época (ex.: `Docs/unreal-player-locomotion-ready-2026-06-04.md`).
3. Este mapa.
4. Código atual que compila (mostra o que sobreviveu, não o que era o design final).

Docs `.md` recuperados por winfr que contêm bytes binários/NUL **não** são fonte confiável — só
provam que o nome existiu. Os ledgers/roadmaps **escritos** pela sessão de recuperação são limpos.

## Legenda de status

- ✅ **Presente** — a mudança (ou seu símbolo/efeito) existe no código atual.
- ❌ **Sumiu** — não há vestígio no código atual; precisa ser re-aplicada.
- ⚠️ **Parcial / verificar** — o conceito existe, mas em arquivo/forma diferente, ou o arquivo
  original sumiu e a equivalência ainda não foi confirmada no código novo.

## Convenção de re-aplicação

Muitos arquivos de combate/movimento do server foram **reorganizados** na recuperação. Mapa de
"de → para" observado até agora:

| Arquivo original (chat) | Situação hoje |
| --- | --- |
| `internal/combat/pipeline.go` | sumiu — lógica de dano/defesa migrou p/ `player_skill_combat_system.go` (monólito 127KB), `systems/defense_system.go`, `controllers/defense_controller.go`, `gamefsm/core_fsms.go` |
| `internal/combat/defense.go` | sumiu |
| `internal/combat/defense_intent.go` | sumiu |
| `internal/combat/creature_combat_system.go` | sumiu — wolf hoje vive inline em `gameapi/runtime.go` (política in-memory recuperada) |
| `internal/combat/config.go` | sumiu |
| `internal/movement/resolver.go` | sumiu — `internal/movement` hoje só tem registry/types/timeline, **sem resolver** |
| `internal/movement/config.go` | sumiu |
| `internal/hitbox/types.go` | sumiu — `internal/hitbox/runtime.go` existe |
| `internal/app/dependencies.go` | sumiu |
| `bootstrap/024_combat_defense_contract_seed.sql` | renumerado p/ `bootstrap/019_combat_defense_contract_seed.sql` (recovery-compacted) |

---

## Linha do tempo

### 2026-06-04 — Movimento: predição & reconciliação autoritativa (Unreal ↔ bridge)

**Contexto:** usuário queria o caminho AAA, não paliativo. O server pediu o protocolo de
input-prediction autoritativo; o Unreal implementou. Devolutiva limpa:
`B:/Unreal Projects/PlainTestMap/Docs/unreal-player-locomotion-ready-2026-06-04.md`.
Doc de origem do request: `server-apeiron/docs/reviews/unreal-movement-prediction-reconciliation-requests-2026-06-04.md` (**❌ não localizado hoje** — recuperar se possível).

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | `MoveCommand.desired_position` volta por default, tratado como **predição local** da capsule/root (não cursor/nav/mesh/destino autoritativo) | `ApeironGameServerBridge.cpp` | ✅ `bSendDesiredPositionWithMove` presente (2 arq) |
| 2 | `BeginPlay` força `bSendDesiredPositionWithMove=true` (evita valor antigo serializado no mapa) | `ApeironGameServerBridge.cpp` | ⚠️ símbolo presente; forçar-em-BeginPlay a confirmar |
| 3 | Cada `SubmitMove` registra predição pendente por `sequence`; `CommandAck`/`movement_correction` reconciliam e removem predições confirmadas | `ApeironGameServerBridge.cpp/.h` | ✅ `movement_correction` presente (1 arq) |
| 4 | Correção pequena = smoothing conservador; **snap só em erro severo** ou `movement_correction.policy=snap`. Removido o comportamento antigo que segurava erro até 2200cm e só então dava snap | `ApeironGameServerBridge.cpp` | ⚠️ verificar lógica de dead-zone/threshold no código atual |
| 5 | Correção por snapshot também suaviza erro normal acima da dead zone | `ApeironGameServerBridge.cpp` | ⚠️ verificar |
| 6 | `/submit-leap` vira rota de comando no bridge HTTP e no C++ (`COMMAND_TYPE_LEAP`, `LeapCommand.direction/charge`); `Space` chama `SubmitLeap` + `Jump()` local | `ApeironGameServerBridge.cpp`, `Tools/ApeironGrpcBridge/main.go` | ✅ `submit-leap` presente (1 arq) |
| 7 | Bridge repassa `last_processed_command_sequence`, `last_processed_client_tick`, `movement_correction` do snapshot | `Tools/ApeironGrpcBridge/main.go` | ⚠️ verificar main.go |
| 8 | `RunValidationSequenceEditor` passa a validar leap (além de move/dodge/basic/skills) | `ApeironGameServerBridge.cpp` | ⚠️ verificar |
| 9 | **Despawn grace:** full snapshot sem creature não apaga visual se `RuntimeStatus.ActiveEntities > PlayerCount`. Corrige creature sumindo por `removed snapshot actor ... missing_from_3_full_snapshots` | `ApeironGameServerBridge.cpp` | ❌ string `missing_from_3_full_snapshots` ausente (0 arq) — re-aplicar/confirmar |

**Pendências da época (PIE com server rodando) — ainda válidas:** confirmar `movement_protocol=server_authoritative_input_prediction_v1`, `ability_key=jump/dodge/sprint`, `movement_correction` chegando no snapshot, e ausência de rubberband visível em WASD.

### 2026-06-05 — HUD inferior + feedback visual + **direção de arte**

**Contexto:** avançar em visual/feedback sem tocar em cálculo de movimento/dano. O dano fica
**autoritativo no server**; o cliente só exibe evento.

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | HUD inferior refeita: painel de recursos, barras VIT/FOC, hotbar central com glifo por skill + overlay de cooldown/lock, painel de comando compacto; flags `bShowPlayerHudNumbers`, `bShowPlayerHudCommandPanel` | `ApeironDebugHud.cpp/.h` | ⚠️ flags presentes (2 arq) — **mas direção visual REJEITADA** (ver decisões abaixo) |
| 2 | Stamina HUD com smoothing/flash/delta; usa stamina já vinda do snapshot/ack (sem mexer em cálculo) | `ApeironDebugHud.cpp`, `ApeironGameServerBridge.cpp` | ⚠️ verificar (stamina recovery ainda não visível em tela na época) |
| 3 | Hit splash + número de dano flutuante (ex.: `128`), só exibição do evento `DAMAGED` recebido | `ApeironCreaturePlaceholder.cpp` | ✅ `DAMAGED` presente (2 arq) — aprovado pelo usuário |

> Estas decisões são **requisitos de direção**, não código — registrar para não repetir erro:
> - HUD deve ser **stylized painterly (Zelda Wind Waker / BOTW)**, não dark-fantasy/metal. A 1ª
>   passada saiu metálica escura e foi **rejeitada**; refazer leve, respirada, cores chapadas com
>   leve textura, ícones de silhueta forte.
> - **Sem indicadores que facilitem** (nada de flash no nome, nada de marcador "creature no ar").
>   O hunter tem que *sentir* a criatura.
> - Barra de vida e nome da creature: **com flag pra desligar** (usuário prefere sem; vai associar
>   às configurações do player).
> - **Cor do corpo por aggro/combat state: rejeitada.**
> - Hit splash: **aprovado.**

### 2026-06-13 — Parry por direção, barras de windup, redução de velocidade no block

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | **Parry usa a direção de defesa enviada no `SubmitBlock`**, não o yaw de locomoção. Log mostrava `target_parry_active=true` + `parry_failure_reason="wrong_guard_arc"`: janela existia, arco julgado pela direção errada | `pipeline.go:121`, `defense.go:47`, `defense_intent.go:73`, `pipeline_test.go:169` | ❌ `wrong_guard_arc` ausente (0 arq) + arquivos sumiram — **re-aplicar** em `systems/defense_system.go` / `controllers/defense_controller.go` |
| 2 | Barra de windup acima da creature: `WINDUP [####----] 620ms lunge` → `ACTIVE` → `RECOVERY`. Liga por `skill_state=active` **ou** AI em `attack_window/punish` com skill selecionada; reconhece id com prefixo (`steppe_wolf:lunge`). Componente separado `AttackWindupLabel` | `ApeironCreaturePlaceholder.h:85`, `ApeironCreaturePlaceholder.cpp:729/781` | ✅ `AttackWindupLabel`/`WindupLabel` presentes (2 arq) |
| 3 | **Redução de velocidade no block** (autoritativa no resolver): sprint→`0.5x` walk base; ré→`0.55x` walk; lateral→`0.75x` walk. `DefenseMovementSpeedMultiplier` com cap 0.5 enquanto bloqueando | `resolver.go:142`, `config.go:90`, `dependencies.go:152`, `defense.go:35`, `024_..._seed.sql:560` | ❌ `DefenseMovementSpeedMultiplier` ausente (0 arq); seed `019` **não tem** os caps — **re-aplicar** |

**Janelas registradas (úteis p/ tuning):** shield parry `60–300ms` após block start (240ms de janela);
lunge windup `480ms`, active `480–720ms`; bite windup `360ms`, active `360–540ms`.

### 2026-06-14 — Block direcional (arco 180°) + hitbox vs target lock

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | **Block só no arco frontal.** Fallback sem policy explícita = **180°** (50% frontais dos 360°). Hit pelas costas enquanto bloqueando para o outro lado **não** fica `Blocked` | `pipeline.go:234`, `creature_combat_system_test.go:258`, `pipeline_test.go:506` | ⚠️ `FrontalArc` presente (2 arq), mas arquivos originais sumiram — **verificar** se o 180°/back-bypass está correto no código novo |
| 2 | **Hitbox decide o hit, não target lock.** Dano aplicado no alvo real retornado pela hitbox (`hit.TargetID`), não no target travado da IA | `creature_combat_system.go:534`, `creature_combat_system_test.go:50` | ⚠️ `creature_combat_system.go` sumiu — verificar equivalente em `gameapi/runtime.go` |
| 3 | **Single-target (`MaxTargets=1`)** escolhe alvo por **distância projetada na direção do ataque** (`ForwardDistance`): B só toma no lugar de A se estiver antes no caminho do golpe | `runtime.go:87`, `runtime_test.go:53` | ✅ `ForwardDistance` presente (3 arq) |
| 4 | **Multi-target/área** gera múltiplos `AttackOutcome` (antes parava no 1º hit) | `creature_combat_system.go:516`, `creature_combat_system_test.go:82` | ✅ `AttackOutcome` presente (2 arq) — verificar se gera múltiplos no caminho do wolf |

**Regra de design (Souls-like / New World PvP):** target = só IA mirar/aproximar/escolher skill/orientar
corpo; **hitbox por skill decide quem tomou hit**. Cada skill declara política: `primary_target_only`
(mordida/agarrão/execução), `any_valid_enemy_in_hitbox` (cone/cleave/salto/área), `max_targets`/`pierce`.
Cliente nunca escolhe quem levou hit.

### 2026-06-16 (terça) — Shield Bash gap-close, direção das skills de chão, **regressão do rubberband**, cor do Niagara

**Contexto:** sessão de combate + movimento (= `recuperação 8`). Aqui aconteceu a **regressão do
rubberbanding** que foi o foco do usuário até o delete.

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | **Shield Bash vira gap-close curto:** passo curto pra frente no início da ação, **por contrato** (sem hardcode). `movement_start_phase=action_start`; passo 75cm / 80ms / speed 940; windup 110ms; hit 110–240ms; stun 1.5s | `db:bootstrap/027_player_shield_bash_seed.sql:61`, `player_skill_combat_system_test.go:428` | ⚠️ seed `027` sumiu (config foi p/ `db:bootstrap/013_player_sword_shield_skill_seed.sql`); `MovementStartPhase` presente (4 arq) — **verificar valores 75/80/940** |
| 2 | **Skills de movimento coladas ao chão usam direção HORIZONTAL do boneco** (yaw autoritativo da locomoção → fallback `Transform.RotationY` → fallback AimDirection horizontal), **não** o ponto 3D do mouse. Bug: `commitPendingPlayerSkillMovementTarget` usava `TargetPosition` do mouse → encurtava a distância ao mirar no chão perto. Removido o stop-distance quando não há alvo/contato real. Genérico p/ dash/charge (shield rush, shield bash, maul-like) | `player_skill_combat_system.go` (+58), `player_skill_combat_system_test.go` (+35) | ⚠️ `commitPendingPlayerSkillMovementTarget` presente (1 arq) — **verificar** se usa yaw horizontal e não o mouse |
| 3 | **Regressão do rubberband** (andar/curva/pulo/dodge juntos): client tinha backlog de inputs locais (`pending_after=10/11`) e o reconciler aplicava correções pequenas demais na cápsula. Fix no Unreal: replay de movimento pendente também pode virar **defer/correction debt** usando `ModeDeadZone` + `CorrectionMaxStep` do perfil, **sem hardcode**. Também diagnosticado: restart do server durante PIE causa mismatch de epoch/baseline | `ApeironGameServerBridge.cpp` (+14-5) | ✅ `ModeDeadZone`/`CorrectionMaxStep`/`correction debt`/`defer` presentes (Unreal) — **mas ver nota abaixo** |
| 4 | **Cor do Niagara** do wolf lunge dust: marrom quente (estava preto fosco). `NS_Wolf_Lunge_GroundDust_WarmGrayBrown_NaturalV1` | `ApeironGroundDustNiagaraBuilderCommandlet.cpp` (+19-17) | ✅ `FLinearColor(0.62, 0.45, 0.27)` no commandlet + assets `NS_..._WarmGrayBrown_*` no Content |

> **Nota crítica sobre o rubberband (#3):** o patch de 16/06 sobreviveu no Unreal, mas foi uma
> **mitigação client-side** no reconciler. A causa-raiz apontada pelo gap-audit (P0 #1) é
> **ownership do resolver de movimento no server** — hoje a locomotion é produzida por múltiplos
> caminhos (`gameapi/runtime.go`, `combat/player_skill_combat_system.go`), e o `internal/movement`
> não tem resolver. Re-aplicar o patch do reconciler **não** substitui reconstruir o resolver.
> Isto liga diretamente à **Fatia 1** da reconstrução.

> **Mapa recuperação ↔ data:** `recuperação 10` e `9` = **14/06** (block arco + hitbox);
> `recuperação 8` = **16/06** (esta seção). Numeração maior = mais antiga; o usuário envia em
> direção ao `recuperação 1` (mais novo, perto do delete).

---

## Tabela consolidada de gaps (o que falta re-aplicar)

Prioridade do que **sumiu** e precisa voltar:

| Prioridade | Gap | Origem | Onde re-aplicar |
| --- | --- | --- | --- |
| Alta | Parry por **direção de defesa** (`SubmitBlock`), não yaw | 13/06 #1 | `systems/defense_system.go`, `controllers/defense_controller.go` |
| Alta | **Redução de velocidade no block** (0.5x/0.55x/0.75x) autoritativa + seed | 13/06 #3 | resolver de movimento (a reconstruir) + `bootstrap/019_..._seed.sql` |
| Alta | **Despawn grace** da creature (não apagar por ausência temporária) | 04/06 #9 | `ApeironGameServerBridge.cpp` |
| Média | Confirmar **arco 180° / back-bypass** no block do código novo | 14/06 #1 | onde `FrontalArc` é avaliado |
| Média | Confirmar **hitbox-decide-hit** (não target lock) no wolf atual | 14/06 #2 | `gameapi/runtime.go` |
| Média | **Reconciliação**: smoothing acima da dead-zone, snap só severo | 04/06 #4-5 | `ApeironGameServerBridge.cpp` |
| Média | **Rubberband — causa-raiz**: ownership do resolver de movimento no server (patch client de 16/06 é só mitigação) | 16/06 #3 | `internal/movement` (Fatia 1) |
| Média | Verificar **Shield Bash gap-close** (75/80/940, `movement_start_phase=action_start`) | 16/06 #1 | `db:bootstrap/013_player_sword_shield_skill_seed.sql` |
| Média | Verificar **direção horizontal das skills de chão** (yaw, não mouse) | 16/06 #2 | `combat/player_skill_combat_system.go` (`commitPendingPlayerSkillMovementTarget`) |
| Baixa | **HUD painterly** (refazer; a versão metálica foi rejeitada) | 05/06 | `ApeironDebugHud.cpp/.h` |
| Baixa | Recuperar doc de request sumido | 04/06 | `docs/reviews/unreal-movement-prediction-reconciliation-requests-2026-06-04.md` |

## Fontes / docs relacionados (limpos)

- `B:/Unreal Projects/PlainTestMap/Docs/unreal-player-locomotion-ready-2026-06-04.md`
- `server-apeiron/docs/recovery/reconstruction-gap-audit-2026-06-22.md`
- `server-apeiron/docs/recovery/full-project-reconstruction-roadmap-2026-06-22.md`
- `db-apeiron/docs/recovery/chat-recovery-ledger-2026-06-22.md`
- `server-apeiron/docs/recovery/codex-chat-roadmaps/thread-source-index-2026-06-22.md`

## Próximos chats a integrar (placeholder)

Apêndar aqui, mantendo a ordem cronológica, conforme forem colados:

- [x] `recuperação 10` + `9` — integrados como **14/06** (block arco frontal + hitbox vs target lock)
- [x] `recuperação 8` — integrado como **16/06** (shield bash gap-close, direção das skills de chão, rubberband, niagara)
- [ ] `recuperação 7`
- [ ] `recuperação 6`
- [ ] `recuperação 5`
- [ ] `recuperação 4`
- [ ] `recuperação 3`
- [ ] `recuperação 2`
- [ ] `recuperação 1`
