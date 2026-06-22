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

> 🚫 **Fora de escopo — Niagara/VFX:** decisão do usuário (22/06): *"tudo que for de Niagara, não
> vamos fazer — perda de tempo, quero o funcional de volta, não firula"*. Mudanças de VFX/Niagara
> ficam registradas como histórico, mas **não entram na lista de gaps a re-aplicar**.

## Objetivo final (definição de "pronto")

Mandato do usuário: a reconstrução é para a **versão final e completa do jogo**, não um estado "de
recuperação". Concretamente:
- Recriar **do zero** os arquivos Go corrompidos/ausentes sem fonte, **com base nos outros arquivos
  já recuperados + no que o Unreal exige**.
- **O Unreal é a fonte de verdade que sobreviveu:** o que o cliente consome (comandos enviados,
  campos de snapshot/ack, contratos, `ability_key`) **define** o que server/DB/proto/migrations/seeds
  precisam ter. Dá pra derivar os Go/protos/migrations/seeds faltantes a partir do que o Unreal pede.
- **Criar todos os seeds que faltam** pra o Unreal carregar com o jogo **rodando no PlainTestMap**
  como já estava.
- Nada de fallback silencioso como estado final; contratos de DB/profile são autoritativos.

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
| 4 | **Cor do Niagara** do wolf lunge dust: marrom quente (estava preto fosco). `NS_Wolf_Lunge_GroundDust_WarmGrayBrown_NaturalV1` | `ApeironGroundDustNiagaraBuilderCommandlet.cpp` (+19-17) | 🚫 **FORA DE ESCOPO** (decisão do usuário: nada de Niagara/VFX). Já presente, mas não é gap nem prioridade |

> **Nota crítica sobre o rubberband (#3):** o patch de 16/06 sobreviveu no Unreal, mas foi uma
> **mitigação client-side** no reconciler. A causa-raiz apontada pelo gap-audit (P0 #1) é
> **ownership do resolver de movimento no server** — hoje a locomotion é produzida por múltiplos
> caminhos (`gameapi/runtime.go`, `combat/player_skill_combat_system.go`), e o `internal/movement`
> não tem resolver. Re-aplicar o patch do reconciler **não** substitui reconstruir o resolver.
> Isto liga diretamente à **Fatia 1** da reconstrução.

> **Mapa recuperação ↔ data:** `recuperação 10` e `9` = **14/06** (block arco + hitbox);
> `recuperação 8` = **16/06** (esta seção). Numeração maior = mais antiga; o usuário envia em
> direção ao `recuperação 1` (mais novo, perto do delete).

### recuperação 7 → 5 (semana do delete: terça-noite → quinta)

> Datas aproximadas — nesse trecho o usuário rodava **chats paralelos** na mesma semana
> (~16–18/06). A ordem autoritativa é o número da recuperação (7 mais antigo → 5 mais novo).

#### recuperação 7 — Wolf lunge com andada PÓS-pouso (continuação natural)

**Pedido:** o lunge devia dar uma andada **depois** do salto, continuando em linha como parte do
movimento (pulo natural), não só a andada antes.

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | Contrato do `wolf_lunge_leap` estava **fora da timeline** da action (movimento começava 1840ms, takeoff 2000ms, mas a action acabava ~1080ms → server encerrava a skill antes da parte pós-salto). Reajustado p/ caber: movimento 480ms, takeoff 640ms, deslocamento total 520ms, `landing_lock_ms=140` como **continuação horizontal no chão após o arco** | `db:015_skill_movement_effect_seed.sql:89` | ⚠️ seed `015` renumerado → `db:018_legacy_skill_movement_effect_seed.sql` — **verificar valores** |
| 2 | Server mantém a skill **pendente até o fim do movimento**, não só até o fim da hitbox | `creature_combat_system.go:427`, `creature_combat_system_test.go:377` | ❌ `creature_combat_system.go` sumiu — re-aplicar no wolf runtime (`gameapi/runtime.go`) |
| 3 | Unreal não limpa o estado visual antes da janela completa de movimento | `ApeironCreaturePlaceholder.cpp:458` | ⚠️ verificar no cliente |

#### recuperação 6 — Basic Attack como combo de 3 etapas server-authoritative

**Decisão de design:** M1 continua mandando só `player_basic_attack` (input simples); o **server**
resolve a etapa do combo (`_1/_2/_3`) pela janela de combo + estado. Cada etapa = skill real
separada (anim key, windup/active/recovery, hitbox, dano/posture, stamina, step-in, cancel window,
hitstop) — facilita animação depois.

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | `player_basic_attack_1/2/3` com `combo_group=old_china_basic_attack`, `combo_index`; cada etapa com hitbox/timing/dano/poise/impact/step-in. Step-in: 1=120cm, 2=145cm, 3=175cm. Entram no path de **movimento imediato/preditivo** do player | `player_skill_combat_system.go`, `db:019_old_china_player_vertical_slice_seed.sql`, `db:024_combat_defense_contract_seed.sql` | ⚠️ `player_basic_attack` presente (5 arq) mas `combo_group/index` fracos (1 arq); seed `019_old_china` (320 linhas) **sumiu/renumerado** — verificar etapas |
| 2 | **Bug do dano:** o bridge mandava `player_quick_slash` no left-click, **não** `player_basic_attack` → todo o combo bypassado. Corrigido o roteamento do M1 | `Tools/ApeironGrpcBridge/main.go`, `app/dependencies.go`, `gameapi/services.go` | ❌ **verificar roteamento do M1 hoje** (regressão provável) |
| 3 | **Trava autoritativa:** `BASIC_ATTACK`/`CAST_SKILL` retornam `action_locked` se o player estiver em dodge/leap/knockback/`movement_mode=airborne` (sem skill durante pulo/dodge) | `dependencies.go:546`, `combat_command_gate_test.go:167` | ⚠️ `action_locked`/`airborne` presentes, mas **test `combat_command_gate_test.go` sumiu** — re-aplicar gate + teste |

#### ★ recuperação 5 — Autoridade skill-movement (A SPEC DA FATIA 1 / raiz do rubberband)

Chat mais importante para a reconstrução. Define a arquitetura correta e **por que** o rubberband no
fim das skills acontece.

**Arquitetura correta:**
- **Combat é dono da skill:** início, timing, alvo, direção, hitbox, cooldown, lock, pending state.
- **Movement resolver é dono da locomotion e a publica:** `Action`, `MovementMode`, `Phase`,
  `ReconciliationMode`, `TargetSpeed`, `EffectiveSpeed`, `PhaseSpeedScale`, `ActionDistanceTraveled`,
  `ActionProjectedPosition`, `ActionStartPosition`.

**O que estava quebrado (e continua):** o combat faz as duas coisas —
1. entrega `movement.Intent` ao resolver (`player_skill_combat_system.go:933`),
2. o resolver escreve `Movement.Locomotion` (`resolver.go:261`),
3. **mas o combat sobrescreve o mesmo estado na mão** (`player_skill_combat_system.go:2566`).
A boa lógica de montagem estava em `resolver.go:1103` (`locomotionStateForResolve`).
`CommitMovementResult` resolvia a posição pelo movement e depois **sobrescrevia o snapshot** com
outro cálculo (`player_skill_combat_system.go:1004`).

**Regra:** a correção certa **elimina a duplicidade de autoridade**, não compara campo-a-campo.
Combat fornece intenção; movement publica o estado final. Teste obrigatório = paridade nos 10 campos.

**Status no código atual:**
- ❌ `internal/movement/resolver.go` **não existe** (resolver inteiro sumiu).
- ❌ `locomotionStateForResolve`, `LocomotionStateComponent`, `PhaseSpeedScale` — **sumiram** (0 ocorrências).
- ⚠️ locomotion ainda montada em **3 lugares**: `combat/player_skill_combat_system.go` (3403 linhas),
  `gameapi/contracts.go`, `gameapi/runtime.go` → **autoridade dupla/tripla persiste**.
- **Fatia 1** = reconstruir `internal/movement` como **único produtor** de locomotion (os 10 campos),
  e fazer o combat emitir só intent/timeline.

#### ★ recuperação 4 — Shield Rush (R) / Shield Bash (F): rubberband na avançada (= BUG ATUAL)

**GIGANTE.** Sessão inteira caçando o rubberband do deslocamento de Shield Rush/Bash **sem** tocar em
walk/run/dodge/leap. É a aplicação concreta da arquitetura do chat 5 às duas skills de chão — e
descreve **exatamente** o rubberband que o usuário está tendo agora no R/F.

**Causa-raiz (do chat):** Shield Rush/Bash eram deslocamento autoritativo no server, **mas não eram
publicados como `Movement.Locomotion` reconciliável** → o cliente via como correção genérica. Bash
tinha previsão local hardcoded fora do replay; Rush não tinha previsão local nenhuma.

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | Server preserva `command_id`/`sequence` no intent de skill | `internal/skill/context.go:36` | ❌ `skill/context.go` **sumiu** |
| 2 | Shield skill movement publica `grounded_action` com `ReconciliationMode=SkillGroundedAction`, action root, distância, curva, `LastProcessedSequence/ClientTick` | `player_skill_combat_system.go:1601` | ❌ `SkillGroundedAction` **0 refs no server** — não publica mais |
| 3 | Unreal Bridge: modo `SkillGroundedAction`, registra casts de Rush/Bash no input buffer, replay ativo só pra essas skills | `ApeironGameServerBridge.cpp:103` | ⚠️ `SkillGroundedAction` presente no UE (3 refs) — **client pronto, server não** |
| 4 | Player local prevê Bash/Rush por distância/curva alinhado ao server | `ApeironTestPlayerCharacter.cpp:2132` | ❌ `SkillDash` **0 refs no UE** — previsão local sumiu |
| 5 | **Câmera descola no dodge+hit** (vira MMO/WoW, fica solta): cortar `CameraVisualCorrectionOffset`, prender no `CameraBoomLocalOffset`, só o mesh suaviza | `ApeironTestPlayerCharacter.cpp/.h`, `ApeironGameServerBridge.cpp` | ⚠️ `CameraVisualCorrectionOffset` ainda presente (3 refs) — **bug pode voltar** |
| 6 | Rush snap: server publicava `base_speed=10.6/199.6` em vez do deslocamento real. Derivar `EntrySpeed/TargetSpeed` de `Distance/Duration` | `player_skill_combat_system.go:1652` | ❌ `EntrySpeed` **0 refs** — sumiu |
| 7 | Rush/Bash começam deslocamento autoritativo em `action_start` (não startup/active); seed Rush `movement_start_phase='action_start'`. Bash vibrando parado: cooldown local + rollback de ACK rejeitado | `player_skill_combat_system.go`, `db:026_player_shield_rush_seed.sql` | ❌ seed `026` sumiu (config em `013`); cooldown/rollback a verificar |

> **★ Por que o R/F rubberbanda AGORA:** a assimetria. O Unreal ainda conhece `SkillGroundedAction`
> (pronto pra reconciliar), mas o server **não publica mais** esse modo (`0 refs`), `skill/context.go`
> sumiu (sem `command_id` no intent), e a derivação de velocidade (`EntrySpeed`) sumiu. Deslocamento
> autoritativo **sem** locomotion reconciliável → cliente trata como correção genérica → rubberband.
> Isto é **Fatia 1**: o movimento de skill tem que ser publicado pelo resolver único como
> `SkillGroundedAction`, não sobrescrito pelo combat. A câmera descolando (#5) é um bug separado mas
> da mesma família (correção visual vazando pra câmera).

#### recuperação 3 — (Niagara/VFX) — 🚫 IGNORADO

Fora de escopo por decisão do usuário ("nada de Niagara/firula"). Não mapeado.

#### recuperação 2 — Boneco preso (leap, 3º hit, ataque pesado) + mode_slots (sáb 20/06)

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | **Leap/jump preso:** removido o congelamento horizontal no fim do leap enquanto ainda no ar; mantém queda + air control limitado em vez de zerar XY | `ApeironTestPlayerCharacter.cpp:3919` | ✅ `AirControl` presente (4 refs) |
| 2 | **3º hit do basic preso:** reconciliação grounded volta a aceitar correção autoritativa em recovery/fase terminal depois que a prediction local acaba | `ApeironTestPlayerCharacter.cpp:4420` | ✅ `LandingHandoff`/recovery presentes (7 refs) |
| 3 | **Ataque pesado/grounded preso:** prediction local do ActionMovement durava a ação inteira (incl. recovery) e zerava velocidade. Agora só startup+active; em recovery já anda, action lock ainda bloqueia skill/dodge nova | `ApeironTestPlayerCharacter.cpp:747`, `.h:316` | ✅ `StartupActive` presente (3 refs) — verificar janela |
| 4 | **mode_slots:** HUD/input só usa `mode_slots[]` quando combat mode enforced; sem slots → vazio, sem fallback local errado (Bulwark não tenta em Vanguard) | `ApeironTestPlayerCharacter.cpp:1132`, `ApeironCombatHudViewModel.cpp:256` | ✅ `mode_slots` presente (UE 4, server 2) |

> Seed confirma Bulwark = slot1 `player_shield_bash`, slot2 `player_shield_rush`, slot3 vazio
> (`db:017_weapon_kit_seed.sql`). `weapon_heavy` real ainda **não** está ativo (seed FALSE, sem fluxo
> hold/release) — heavy autoritativo é fase separada.

#### ★ recuperação 1 — CTRL troca de combat mode + Shield Rush meio-cilindro (sáb 20/06, mais novo numerado)

**O CTRL é outro bug recorrente** (usuário pediu 3x). Causa real: **bridge stale (404)** + Unreal não
processava o **ACK** de `/submit-switch-combat-mode`.

| # | Mudança | Arquivos originais (chat) | Status |
| --- | --- | --- | --- |
| 1 | Unreal processa o ACK do CTRL: `active_combat_mode`, `target_combat_mode`, `combat_mode_phase`, timers, em `ApplyCommandAckResponse` | `ApeironGameServerBridge.cpp:2365` | ⚠️ símbolos presentes (1 ref cada) — **verificar se realmente alterna** |
| 2 | HUD não usa mais `resolved_combat_mode` de skill **rejeitada** como modo ativo (era o que fazia o F "mudar a barra" sem mudar o modo) | `ApeironCombatHudViewModel.cpp:263` | ⚠️ `resolved_combat_mode` presente (1 ref) — verificar |
| 3 | Com `combat_mode_enforced=true`, Q/R/F não caem em fallback local: vêm de `mode_slots[]` do modo ativo ou ficam vazios | `ApeironCombatHudCanvasRenderer.cpp:283` | ✅ `combat_mode_enforced`/`mode_slots` presentes |
| 4 | Fila HTTP **coalesce** move/turn velho e **prioriza** switch_combat_mode/leap/dodge/cast (conserta pulo preso atrás de spam). Handoff de pouso flushado antes de novo pulo/dodge | `ApeironTestPlayerCharacter.cpp:798` | ❌ `coalesce/Prioritize` **0 refs** — sumiu |
| 5 | **Shield Rush dano a meio cilindro** à frente (não cilindro inteiro): `offset_x=48cm`, `radius=96cm`, mantendo movimento/knockback/carry/front_plow | `db:026_player_shield_rush_seed.sql` | ⚠️ seed `026` sumiu (config em `013/015`) — **verificar 48/96** |
| 6 | Proteger leap/dodge/turn como baseline intocável | `apeiron-skill-movement-audit/SKILL.md` | ℹ️ regra de processo (skill do Codex) |

> **Design futuro:** o usuário quer um **3º combat mode** (jogar com 2 de N → variedade de build).
> Primeiro o CTRL tem que funcionar.
> **Identidade visual (referência canônica):**
> `C:/Users/elmir/.codex/skills/apeiron-aaa-roadmap-governor/assets/apeiron-game-visual-reference.jpg`
> — quando o usuário falar "identidade visual do meu game", abrir essa imagem. (HUD visual = 🚫 adiado.)

#### ★★ CHAT ATUAL (o do delete) — rubberband em TODA skill de movimento do player

Último chat antes do delete. Dedicado **só** ao rubberband no fim **e durante** cada skill de
movimento do player. 4 tentativas, nenhuma resolveu antes do delete — e a conclusão do usuário é a
direção da Fatia 1.

| Tentativa | Hipótese / fix | Resultado |
| --- | --- | --- |
| 1 | `MovementLocked` de skill virava correção: sem input, server gerava `StopIntent`, validator rejeitava "movement locked", gravava `movement_correction`, Unreal lia como erro autoritativo → snap. Fix: `ServerMovementHold` (`intent.go:101`, `system.go:62`, `validator.go:45`, `resolver.go:919`) | ❌ "nada, mesma merda" |
| 2 | Skill locomotion passava **fora** do resolver e vazava `LastCorrection` antigo no snapshot no fim | ❌ não resolveu |
| 3 | Trace pesado persistente (`logs/movement-trace.enabled`): resolve_start, candidate, navmesh, collision_probe, depenetration, `hard_enemy_body_block` | (diagnóstico) |
| 4 | **Causa central:** player skill movement **mutava `SetPosition` FORA do `movement.Resolver`**, enquanto dodge/jump passam pelo pipeline correto. Fix: skill movement pelo resolver (navmesh/body-block/depenetration/correction/trace); `MovementLocked` libera só root motion da própria skill; contato respeita separação de cápsulas (source+target radius); resolvido auto-conflito `action channel conflict player_basic_attack_1 -> player_basic_attack_1` | ❌ "rubberbanding até entupir, desisto" |

**Conclusão do usuário (= direção da Fatia 1):** *"o caminho é INTEGRAR a movimentação normal com a
de skill movement"* — comparar o que cada skill-movement faz vs movement normal (walk/run, fluido) e
achar a divergência server↔client. Skills com movimento rubberbandam **até durante a execução**.

**Status no código atual — o subsistema inteiro SUMIU:**
- `internal/movement` hoje só tem `action_contract_registry`, `action_timeline`, `types`.
- ❌ SUMIRAM: `resolver.go`, `intent.go`, `system.go`, `validator.go`, `depenetration.go`.
- ❌ `ServerMovementHold`, `hard_enemy_body_block`, `movement_trace`, depenetration, action-channel-conflict — **0 refs**.
- `SetPosition` em combat só aparece em testes. **A Fatia 1 é reconstruir o resolver do zero**, com skill movement no MESMO pipeline de walk/run/dodge/jump. Chat final + chat 5 + chat 4 = brief completo.

---

## Síntese: por que as skills de movimento rubberbandam (brief da Fatia 1)

Cruzando os chats **5, 4 e o final**, a causa é **uma só**: movimento normal (walk/run), dodge e jump
passam por um pipeline reconciliável (resolver); **o movimento de skill NÃO** — é produzido/sobrescrito
fora do resolver. Daí:
- o combat monta/sobrescreve locomotion na mão (chat 5: `player_skill_combat_system.go:2566`);
- o deslocamento de skill não é publicado como `SkillGroundedAction` reconciliável (chat 4);
- o skill movement muta `SetPosition` fora do resolver, vazando `LastCorrection` (chat final).

O cliente — que sabe reconciliar movimento normal — recebe a skill como **correção autoritativa
genérica** → snap → rubberband (no fim **e** durante).

**Fatia 1 (conserto definitivo, sem hardcode, replicável):**
1. Reconstruir `internal/movement` como **único produtor** de locomotion (os 10 campos do chat 5).
2. **Todo** movimento — walk/run, turn, dodge, leap **e skill** — passa pelo MESMO resolver (navmesh, body-block, depenetration, separação de cápsula, correction/LastCorrection).
3. Combat emite só **intent/timeline**; nunca monta nem sobrescreve locomotion.
4. Skill movement publicado como `SkillGroundedAction` reconciliável, com `command_id`/sequence no intent e velocidade de Distance/Duration; cliente prevê local e dá replay.
5. `MovementLocked` libera só a root motion da própria skill ativa.
6. Resolver auto-conflito de action channel (`skill -> mesma skill`).
7. **Teste obrigatório:** paridade dos 10 campos de locomotion entre walk/run e skill movement.

Não é patch campo-a-campo — é unificar os dois caminhos num resolver só.

## Tabela consolidada de gaps (o que falta re-aplicar)

Prioridade do que **sumiu** e precisa voltar:

| Prioridade | Gap | Origem | Onde re-aplicar |
| --- | --- | --- | --- |
| **Crítica** | **Ownership do resolver de movimento** — combat só emite intent/timeline; movement publica locomotion (10 campos, com teste de paridade). Eliminar autoridade dupla, não patch campo-a-campo | **chat 5** + 16/06 #3 | `internal/movement` (resolver a reconstruir) — **Fatia 1** |
| **Crítica** | **Shield Rush/Bash reconciliáveis** — server publica `SkillGroundedAction` + `command_id` no intent (`skill/context.go`) + `EntrySpeed` de Distance/Duration; client prevê local. **= rubberband atual do R/F** | **chat 4** | server `player_skill_combat_system.go` + `skill/context.go` (sumiu); UE já tem o modo — parte da **Fatia 1** |
| Alta | **Câmera descola no dodge+hit** — prender no `CameraBoomLocalOffset`, só o mesh suaviza | chat 4 #5 | `ApeironTestPlayerCharacter.cpp` (`CameraVisualCorrectionOffset`) |
| Alta | **Wolf lunge pós-pouso**: timeline do movimento dentro da action; `landing_lock` como continuação horizontal; skill pendente até o fim do movimento | chat 7 | `db:018_..._seed.sql`, wolf runtime em `gameapi/runtime.go` |
| Alta | **Basic Attack combo 3 etapas** server-authoritative (`_1/_2/_3`, combo_group/index, step-in) + seed `old_china` (320 linhas sumiu) | chat 6 #1 | `combat/player_skill_combat_system.go`, `db:bootstrap` |
| Alta | **Roteamento do M1**: left-click tem que sair como `player_basic_attack` (não `player_quick_slash`) | chat 6 #2 | `Tools/ApeironGrpcBridge/main.go`, `app/dependencies.go`, `gameapi/services.go` |
| Alta | **Gate de skill em jump/dodge**: basic/cast = `action_locked` se dodge/leap/knockback/airborne (+ teste) | chat 6 #3 | `app/dependencies.go` (`combat_command_gate_test.go` sumiu) |
| Alta | Parry por **direção de defesa** (`SubmitBlock`), não yaw | 13/06 #1 | `systems/defense_system.go`, `controllers/defense_controller.go` |
| Alta | **Redução de velocidade no block** (0.5x/0.55x/0.75x) autoritativa + seed | 13/06 #3 | resolver de movimento + `bootstrap/019_..._seed.sql` |
| Alta | **Despawn grace** da creature (não apagar por ausência temporária) | 04/06 #9 | `ApeironGameServerBridge.cpp` |
| Alta | **CTRL troca de combat mode** — processar ACK (active/target/phase) + bridge fresco; Q/R/F só do modo ativo | chat 1 #1-3 | `ApeironGameServerBridge.cpp` (`ApplyCommandAckResponse`), HUD viewmodel |
| Alta | **Fila HTTP coalesce/prioriza** (switch_mode/leap/dodge/cast) — conserta pulo preso atrás de spam de movimento | chat 1 #4 | `ApeironTestPlayerCharacter.cpp` (`coalesce` sumiu) |
| Média | **Shield Rush dano a meio cilindro** (`offset_x=48`, `radius=96`) | chat 1 #5 | `db:bootstrap` (seed 026 sumiu) |
| Média | Confirmar **arco 180° / back-bypass** no block | 14/06 #1 | onde `FrontalArc` é avaliado |
| Média | Confirmar **hitbox-decide-hit** (não target lock) no wolf | 14/06 #2 | `gameapi/runtime.go` |
| Média | **Reconciliação**: smoothing acima da dead-zone, snap só severo | 04/06 #4-5 | `ApeironGameServerBridge.cpp` |
| Média | Verificar **Shield Bash gap-close** (75/80/940) | 16/06 #1 | `db:bootstrap/013_player_sword_shield_skill_seed.sql` |
| Média | Verificar **direção horizontal das skills de chão** (yaw, não mouse) | 16/06 #2 | `combat/player_skill_combat_system.go` |
| Baixa | **HUD painterly** (refazer; versão metálica rejeitada) | 05/06 | `ApeironDebugHud.cpp/.h` |
| Baixa | Recuperar doc de request sumido | 04/06 | `docs/reviews/unreal-movement-prediction-reconciliation-requests-2026-06-04.md` |
| 🚫 | **Niagara/VFX** — fora de escopo (decisão do usuário) | 16/06 #4 | — |

## Anexo: meta-sessão de recuperação (provenance do HEAD atual)

O HEAD atual dos repos foi produzido por uma sessão de recuperação do Codex (pós-delete), em commits
locais **sem push** (`gh` ausente / push HTTPS travou). O que ela fez:

**server-apeiron:** `71395c0` movement action contract registry (tira duplicação de classificação/hash
do gameapi) · `add58f5`/`8fdd1b2` load+cover creature skill movement contracts · `67d3fc3` wolf bite
runtime contract · `a85aa68` temporal hitbox sweep · `678b334` movement validation runtime (+ modo
`MovementValidation`; fix do client buscando contrato pra `action=grounded_skill` genérico em vez de
`ability_key`) · `cf8ebec` reconciliation change ledger · `7508918` guard required runtime contract ·
`4066899` enforce runtime contract coverage (falha claro se faltar move/turn/dodge/jump, basic combo,
shield_bash/rush, bite/lunge/wolf_dodge/maul) · `ebd505f` combat hitbox runtime (CombatDefenseContract
no proto/API/cache/repo; block arco frontal; parry na pipeline; impact response player vs creature) ·
`0b1ae78` recovery thread gap audit.

**db-apeiron:** `4f01e98`/`87977fb` wolf maul+bite action contracts · `14c3e5a` recovered DB movement
schemas · `cf97af3`/`987f702` sortable migration filenames (`20_spawn_zone.sql` → `020_`) · `f54f297`
normalize runtime schema (`028_temporal_melee_hitbox.sql` como modelo final) · `9bbfbaf` combat defense
contract api.

A própria recuperação confirmou: **`internal/movement/resolver.go` não existe** (P0); fallbacks ainda
alcançáveis (`RecoveredRuntimeContracts`, AttackProfile); muitos arquivos winfr têm nome certo mas
conteúdo corrompido. Template reutilizável: `docs/recovery/continuous-recovery-request-template.md`.

> Este HEAD **compila**, mas é a base da recuperação — **não** a versão final. A reconstrução
> (Fatia 1+) parte daqui em direção ao **Objetivo final** lá em cima.

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
- [x] `recuperação 7` — wolf lunge pós-pouso (continuação natural)
- [x] `recuperação 6` — basic attack combo 3 etapas + roteamento M1 + gate jump/dodge
- [x] `recuperação 5` — ★ autoridade skill-movement (spec da Fatia 1)
- [x] `recuperação 4` — ★ Shield Rush/Bash rubberband (= bug atual do usuário; implementação do chat 5)
- [x] `recuperação 3` — 🚫 Niagara/VFX, ignorado por decisão do usuário
- [x] `recuperação 2` — boneco preso (leap/3º hit/pesado) + mode_slots (sáb 20/06)
- [x] `recuperação 1` — ★ CTRL combat mode + Shield Rush meio-cilindro (sáb 20/06)
- [x] **chat ATUAL** (o do delete) — ★★ rubberband de skill movement; resolver inteiro sumiu = Fatia 1

- [x] **meta-sessão de recuperação** (pós-delete) — ver Anexo (provenance do HEAD + Objetivo final)
- [ ] continuação da meta-sessão — usuário avisou "ainda tem mais"

**Chats de gameplay (10→1 + skill-rubberband): completos e validados** (`04/06 → delete`). Falta só a
continuação da meta-sessão de recuperação. Direção travada: **Fatia 1 = reconstruir o resolver de
movimento** rumo ao Objetivo final.
