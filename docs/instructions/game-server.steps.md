# APEIRON GAME-SERVER - ROADMAP

Objetivo: construir o servidor autoritativo de gameplay do Apeiron. O game-server consome o db-apeiron por gRPC, mantem estado quente de mundo em memoria, simula regioes/entidades/combate/IA e envia snapshots/eventos para o cliente Unreal 5.

## Principios fixos

- O game-server nao acessa PostgreSQL direto.
- O game-server consome db-apeiron via gRPC.
- O game-server e server-authoritative: posicao, dano, hitbox, cooldown, PvP, inventario e estado final sao decididos no servidor.
- O cliente Unreal envia intencao/input/comando, nunca verdade final.
- Dados estaticos ficam no db-apeiron e sao aquecidos/cacheados no game-server.
- Estado quente de simulacao fica em memoria no game-server.
- PostgreSQL nao vira fonte de tick runtime em alta frequencia.
- Persistencia acontece por eventos importantes, logout, intervalos controlados e checkpoints.
- Runtime usa System + Controller + FSM, mas sem exagerar antes do vertical slice.
- Loose Quadtree + Spatial Hash auxiliar entram primeiro; Octree fica opcional se profiling provar necessidade.
- Navmesh do servidor usa formato proprio importado/exportado a partir do pipeline Unreal 5.
- A primeira meta real e uma regiao viva testavel.

## Status

FASE 1 - PROJECT FOUNDATION
[x] Criar estrutura base do game-server
[x] Criar cmd/game-server/main.go como composition root
[x] Criar go.mod, .gitignore e .env.example
[x] Criar config/env.go e config/config.go
[x] Criar logging/logger.go reaproveitando o padrao forte do db-apeiron: zerolog, pretty console, level por env e WithComponent
[x] Criar app/app.go, app/dependencies.go e app/lifecycle.go
[x] Implementar graceful shutdown por context/signal
[x] Implementar global panic recovery
[x] Criar modelo base de errors/result/codes
[x] Criar scripts/dev/run_game_server.bat e scripts/test/test_all.bat
Gate: app sobe, carrega config, loga lifecycle e encerra limpo.

FASE 2 - CLOCK, TICK E RUNTIME BASE
[x] ServerClock monotonic-safe
[x] TickConfig com tick rate inicial 20Hz
[x] FixedTickLoop
[x] TickPhase model
[x] TickBudget por fase
[x] Slow tick logging
[x] Panic isolation por fase
[x] RuntimeScheduler base
[x] Teste de tick deterministico
Gate: loop roda sem gameplay, mede custo por fase e nao derruba processo por panic isolado.

FASE 3 - DB-APEIRON CLIENT
[x] dbapeiron/client.go
[x] Config de endpoint, timeout, retry e backoff
[x] Health/readiness client
[x] Observability client
[x] CacheService client
[x] CreatureDataService client
[x] PlayerDataService client
[x] WorldDataService client
[x] ProfileDataService client
[x] SkillDataService client
[x] InventoryDataService client
[x] Error mapping db-apeiron -> game-server
[x] Startup validation contra db-apeiron
[x] Startup static warmup call
[ ] Metricas de latencia/falha por metodo
Gate: game-server falha rapido se db-apeiron estiver indisponivel, exceto quando config permitir modo dev offline.

FASE 4 - STATIC DATA REGISTRY
[x] StaticDataRegistry agregado
[x] CreatureTemplateRegistry
[x] ProfileRegistry: movement, combat_core, combat_style, behavior, needs, personality, ai_decision
[x] SkillRegistry e SkillSetRegistry
[x] StatusEffectRegistry
[x] ItemTemplateRegistry
[x] WorldDataRegistry: regions, biomes, spawn zones
[x] Validacao cruzada de referencias
[ ] Admin/dev reload static data
[x] Snapshot de versao/hash dos dados estaticos
Gate: templates necessarios para a regiao inicial carregam e validam antes do mundo ativar.

FASE 5 - MATH, GEOMETRY E IDS
[x] Vec2, Vec3, Transform, Position, Rotation, Velocity
[x] AABB, Sphere, Capsule, Cone, Ray/Segment
[x] Helpers de distancia, direcao, angulo, clamp e interseccao
[x] RuntimeEntityID generator
[x] PlayerID, CreatureID, RegionID, SkillID aliases
[x] Politica de float deterministico/pratico: usar float64 server-side com Epsilon; sem lockstep deterministico antes do profiling/necessidade real
[x] Testes matematicos principais
Gate: geometria basica cobre movement, spatial query e hitbox sem depender do Unreal.

FASE 6 - WORLD PACKAGE E PIPELINE UNREAL 5
[x] Formato WorldPackage
[x] Versionamento e hashes: navmesh, spawn, biome, blocker, safe zone
[x] Loader JSON dev
[ ] Loader binario futuro reservado
[x] RegionDefinition, ZoneDefinition, BiomeVolumeDefinition
[x] SpawnZoneDefinition, SafeZoneDefinition, BlockerDefinition, OffMeshLinkDefinition
[x] Validacao de pacote
[x] Docs do export Unreal -> game-server
[x] Pacote dev old_china inicial
Gate: uma regiao dev pode ser carregada e validada sem gameplay.

FASE 7 - WORLD, REGION E HOT STATE
[x] WorldRuntime
[x] RegionRuntime
[x] RegionRegistry
[x] RegionLifecycle: load, activate, tick, deactivate, shutdown
[x] RegionBoundary e ZoneRuntime
[x] EntityOwnership por regiao
[x] Spawn/despawn registration
[x] Hot state separado de persistencia
[x] Region transfer design placeholder
[x] Testes de lifecycle de regiao
Gate: regioes ativam/desativam e possuem entidades em memoria sem bater no banco por tick.

FASE 8 - ENTITY MODEL
[x] Entity interface/base, EntityType e EntityRef
[x] EntityRegistry por regiao
[x] PlayerEntity
[x] CreatureEntity
[x] ProjectileEntity
[x] AreaEffectEntity
[x] CorpseEntity
[x] InteractableEntity
[x] Entity components minimos: transform, life, movement, combat, controller refs
[x] Lookup por runtime id
[x] Testes de registro/desregistro
Gate: entidades existem como estado quente e podem ser consultadas por regiao.

FASE 9 - SPATIAL RUNTIME
[x] SpatialIndex interface, SpatialObject e SpatialQuery
[x] LooseQuadtree insert/remove/update
[x] QueryRadius, QueryAABB, QueryCone, QueryLine/Segment
[x] SpatialHash auxiliar para queries locais/interest management
[x] Debug dump
[ ] Benchmarks basicos
[x] Testes de spatial
Gate: servidor encontra entidades proximas para percepcao, hitbox e snapshot.

FASE 10 - NAVIGATION RUNTIME SERVER-SIDE
[x] NavMeshRuntime, NavPolygon, NavGraph, NavQuery e NavPath
[x] FindNearestPolygon, IsWalkable e ClampToNavmesh
[x] A* pathfinding
[x] Funnel/string pulling inicial por centros de poligonos; funnel por portal fica para refinamento com exporter Unreal
[x] AreaCost, OffMeshLink e DynamicBlocker
[x] PositionValidator com validacao walkable/clamp; slope/height/blocker validation detalhada fica para integracao com exporter Unreal
[x] Nav debug dump
[x] Testes de path e validacao de posicao
Gate: movimento de player/criatura consegue ser validado no servidor.

FASE 11 - SYSTEM, CONTROLLER E FSM FOUNDATION
[x] System interface e SystemScheduler
[x] Controller interface, ControllerContext e EntityContext
[x] FSM interface, State e Transition
[x] Enter/Update/Exit hooks
[x] Transition reason/debug
[x] Definir regra System != FSM: system processa fase do tick; FSM guarda estado/transicao
[x] Definir regra Controller != System: controller orquestra entidade; system executa lote por regiao
[x] Testes de FSM e controller base
Gate: sistemas processam fases; controllers coordenam entidades; FSMs governam estado sem exigir um system por FSM.

FASE 12 - CORE FSMs E CONTROLLERS
[x] CreatureController e PlayerController
[x] CreatureLifeFSM: spawning, alive, downed, dead, despawning
[x] MovementFSM: idle, moving, dodging, leaping, knocked_back, rooted, staggered
[x] CombatFSM: out_of_combat, engaging, attacking, recovering, disengaging
[x] SkillFSM: ready, windup, active, recovery, cooldown, interrupted, cancelled
[x] DefenseFSM: neutral, blocking, parry_startup, parry_active, parry_recovery, guard_broken, iframe
[x] AIFSM: idle, patrol, investigate, chase, combat, flee, return_home, rest, search_food, search_water
[x] PerceptionFSM: unaware, suspicious, alert, tracking, lost_target
[x] NeedsFSM: stable, seeking_food, seeking_water, exhausted, stressed, afraid, aggressive, relief_needed
[x] AggroFSM: calm, provoked, locked_target, switching_target, leashing, reset
[x] TargetingFSM: no_target, acquiring, locked, lost, invalid
[x] ThreatFSM: no_threat, evaluating, escalating, overwhelmed, safe
[x] StatusEffectFSM: clean, affected, crowd_controlled, immune, expired
[x] CrowdControlFSM: free, staggered, stunned, rooted, feared, taunted, knocked_down
[x] SpawnFSM: inactive, waiting, spawning, active, cooldown, disabled
[x] EncounterFSM: dormant, preparing, active, victory, failed, reset
[x] InteractionFSM: none, inspecting, gathering, looting, talking, using_object
[x] LootFSM: unavailable, reserved, open, looted, expired
[x] CorpseFSM: fresh, lootable, decaying, removed
[x] ProjectileFSM: spawned, travelling, armed, impacted, expired
[x] AreaEffectFSM: pending, active, pulsing, fading, expired
[x] PlayerSessionFSM: connecting, authenticating, loading, attached, playing, disconnecting, reconnecting, detached
[x] PvPFlagFSM: safe, flagged, combat_locked, criminal_future, cooldown
[x] PersistenceFSM: clean, dirty, queued, saving, failed_retry, saved
[x] Testes de transicao critica
Gate: criatura, player, projeteis, areas e encontros possuem FSMs para estados discretos relevantes sem transformar valores continuos em maquinas de estado.

FASE 13 - GAME PROTO CONTRACTS
[x] proto/apeiron/game/v1/common.proto
[x] session_service.proto
[x] command_service.proto
[x] snapshot_service.proto
[x] combat_event_service.proto
[x] admin_service.proto dev/internal
[x] observability_service.proto opcional se separado do HTTP
[x] scripts/proto/generate_game_proto.bat
[x] Generated Go protobuf files
[x] Contract tests de proto
Gate: contrato cliente-servidor existe antes dos sistemas dependerem de rede real.

FASE 14 - NETWORK, SESSION E COMMAND PIPELINE
[x] gRPC server e interceptors logging/recovery/request-id
[x] SessionService, CommandService, SnapshotService e CombatEventService
[x] Connection lifecycle
[x] Player session attach/detach
[x] Command model, CommandBuffer e CommandSequence
[x] Timestamp validation, replay protection e invalid command rejection
[x] Stream cleanup
[x] Network tests
Gate: um player conecta, envia comandos e recebe stream/snapshot vazio ou simples.

FASE 15 - MOVEMENT SYSTEM
[x] MovementIntent e MovementState
[x] MovementValidator e MovementResolver
[x] Server position reconciliation
[x] Navmesh constraint
[x] Collision radius, speed/acceleration limit
[x] Dodge, leap e knockback
[x] Root/stagger movement locks
[x] Movement correction event
[x] Movement tests
Gate: player move no servidor com correcao e validacao anti-teleport basica.

FASE 15.5 - MOVEMENT PHYSICS DB CONTRACT
[x] Validar alteracoes do db-apeiron para movement/collision/skill movement/nav area
[x] db client consome GetSkillMovementEffect
[x] db client consome GetNavArea e ListNavAreasByRegion
[x] MovementProfile do db mapeia slope, step-height, capsule, mass e policies
[x] SkillMovementEffect do db mapeia leap/dash/blink/phase/knockback flags
[x] WorldNavArea do db mapeia metadata exportada do Unreal
[x] BodyBlockResolver
[x] SteeringResolver
[x] DepenetrationResolver
[x] Testes de hard block, soft steering e skill phase
Gate: movement usa contrato persistente do db-apeiron e possui resolvers substituiveis para steering/depenetracao/body-block.

FASE 16 - SKILL RUNTIME
[x] SkillRuntime, SkillInstance e SkillContext
[x] CastPipeline
[x] Ready/WindUp/Active/Recovery/Cooldown states
[x] Interrupted/Cancelled states
[x] CooldownRuntime
[x] Resource cost validation
[x] Charge validation integrada ao db-apeiron quando persistente
[x] CancelWindow e InterruptRule
[x] Skill event publishing
[x] Skill tests
Gate: uma skill melee e uma skill projectile executam ciclo completo no servidor.

FASE 17 - HITBOX SYSTEM
[x] Hitbox interface e HitboxShape
[x] Box/Cone/Sphere/Capsule hitboxes
[x] ProjectileHitbox e AreaEffectHitbox
[x] Active frames, target filter, max targets e multi-hit rule
[x] Line of sight opcional
[x] Spatial query integration
[x] HitResult
[x] Hitbox tests
Gate: skill calcula acerto no servidor, nao no cliente.

FASE 18 - COMBAT, DEFENSE E STATUS EFFECTS
[x] DamageContext, ImpactResolutionPipeline e DamageResult
[x] ValidateTarget e ValidatePvP hook
[x] I-frame/block/parry checks
[x] Damage/resistance/stamina/posture calculations
[x] Stagger/poise, death handling e threat generation
[x] DefenseRuntime: block, perfect block, parry, dodge iframe, guard break
[x] StatusEffectRuntime: dot, hot, slow, root, stun, bleed, poison, burn, fear, taunt, immunity
[x] Stack/refresh rules
[x] Combat/defense/status tests
Gate: dano, defesa, morte e status rodam em pipeline previsivel e testado.

FASE 19 - PERCEPTION, NEEDS, MEMORY E AI
[x] PerceptionRuntime: vision, hearing, smell opcional, line of sight
[x] Last known position, threat detection e alert propagation
[x] NeedsRuntime: hunger, thirst, fatigue, stress, fear, aggression, bladder, bowel, pain
[x] Needs update interval e LOD
[x] RuntimeMemory com decay
[x] Memories: attacker, danger area, food source, ally death, last target, home/territory
[x] PersistentMemoryBridge para salvar somente resumos importantes
[x] Blackboard, DecisionContext, DecisionScore e BehaviorSelector
[x] TargetSelection, SkillScoring, MovementTactics e CombatTactics
[x] GroupAI/pack behavior e territorial behavior
[x] AI states: idle, patrol, search food/water, rest, alert, investigate, chase, combat, flee, return home
[x] AI/perception/needs/memory tests
Gate: criatura percebe, decide, persegue, ataca, foge e volta ao territorio sem persistir cada pensamento no banco.

FASE 20 - SPAWN, BIOME E ENCOUNTER RUNTIME
[x] SpawnZoneRuntime, SpawnBudget, SpawnDensity e RespawnTimer
[x] SpawnRule, BiomeSpawnRule, TimeSpawnRule, PlayerProximityRule e DespawnRule
[x] EncounterRuntime basico
[x] Rare spawn/boss placeholder
[x] Spawn tests
Gate: regiao cria e remove criaturas conforme budget, biome e proximidade.

FASE 21 - INVENTORY, LOOT E ECONOMIA MINIMA
[x] InventoryRuntime como fachada do db-apeiron
[x] Equip/Unequip, Consume, Loot, Drop e CoinOperation
[x] Validacao de item template antes de comando
[x] Chamadas atomicas db-apeiron para item/coin
[x] Loot table runtime basico
[x] Inventory event publishing
[x] Inventory tests
Gate: loot/drop/consume/equip usam operacoes atomicas; duplicidade de item nao nasce no game-server.

FASE 22 - PERSISTENCE STRATEGY
[x] DirtyTracker
[x] PeriodicSave, ImportantEventSave, LogoutSave e DeathSave
[x] InventoryImmediateSave
[x] CreaturePersistentMemorySave resumido
[x] Save queue com backpressure
[x] Comportamento se db-apeiron cair
[x] Persistence tests
Gate: estado importante salva sem transformar tick em write storm.

FASE 23 - PvP, FACTION E SAFE ZONES
[x] PvPValidator e PvPFlagFSM
[x] FactionValidator e SafeZoneValidator
[x] DuelValidator futuro, PartyValidator futuro e FriendlyFireValidator
[x] Crime/Karma futuro
[x] PvP hitbox/skill validation
[x] PvP tests
Gate: nenhum dano PvP passa sem regra centralizada.

FASE 24 - SNAPSHOT, INTEREST MANAGEMENT E EVENT STREAM
[x] InterestManagement
[x] EntitySnapshot, DeltaSnapshot e EventSnapshot
[x] SnapshotBuilder, SnapshotStream e SnapshotBudget
[x] Nearby entities only e region subscription
[x] Combat event stream
[x] Bandwidth counters
[x] Snapshot tests
Gate: Unreal recebe somente estado relevante, com delta e eventos separados.

FASE 25 - ANTI-CHEAT BASE
[x] SpeedValidator, TeleportValidator, CooldownValidator, RangeValidator e HitValidator
[x] AnimationCancelValidator e CommandSequenceValidator
[x] Impossible input detection
[x] Suspicious activity metrics
[x] Anti-cheat tests
Gate: comandos absurdos sao rejeitados e mensurados antes de afetar runtime.

FASE 26 - OBSERVABILITY E ADMIN DEV
[x] Health endpoint e Readiness endpoint
[x] Tick duration, region tick cost, entities per region metrics
[x] AI, spatial/nav e hitbox query metrics
[x] Db-apeiron latency metrics
[x] Snapshot size e dropped/rejected command metrics
[x] Debug dump: region/entities/spatial/nav/ai
[x] Admin-only commands em dev
[x] Observability tests
Gate: da para diagnosticar tick pesado, regiao ruim, IA cara e falha de dependencia.

FASE 27 - TEST SUITE E SIMULATION HARNESS
[x] Unit tests por pacote critico
[x] Contract tests de proto
[x] Dbclient integration tests com db-apeiron
[x] Tick simulation harness
[x] Deterministic scenario tests
[x] Movement/combat/AI/spawn/snapshot scenario tests
[x] scripts/test/test_all.bat validando tudo
Gate: mudanca em gameplay quebra teste antes de quebrar demo.

FASE 28 - VERTICAL SLICE: OLD CHINA REGION
[x] Old_china initial world package
[x] Grassland/forest/rocky_forest biomes
[x] Small village safe// File generated from our OpenAPI spec by Stainless. See CONTRIBUTING.md for details.
var _AbstractPage_client;
import { __classPrivateFieldGet, __classPrivateFieldSet } from "../internal/tslib.mjs";
import { AnthropicError } from "./error.mjs";
import { defaultParseResponse } from "../internal/parse.mjs";
import { APIPromise } from "./api-promise.mjs";
import { maybeObj } from "../internal/utils/values.mjs";
export class AbstractPage {
    constructor(client, response, body, options) {
        _AbstractPage_client.set(this, void 0);
        __classPrivateFieldSet(this, _AbstractPage_client, client, "f");
        this.options = options;
        this.response = response;
        this.body = body;
    }
    hasNextPage() {
        const items = this.getPaginatedItems();
        if (!items.length)
            return false;
        return this.nextPageRequestOptions() != null;
    }
    async getNextPage() {
        const nextOptions = this.nextPageRequestOptions();
        if (!nextOptions) {
            throw new AnthropicError('No next page expected; please check `.hasNextPage()` before calling `.getNextPage()`.');
        }
        return await __classPrivateFieldGet(this, _AbstractPage_client, "f").requestAPIList(this.constructor, nextOptions);
    }
    async *iterPages() {
        let page = this;
        yield page;
        while (page.hasNextPage()) {
            page = await page.getNextPage();
            yield page;
        }
    }
    async *[(_AbstractPage_client = new WeakMap(), Symbol.asyncIterator)]() {
        for await (const page of this.iterPages()) {
            for (const item of page.getPaginatedItems()) {
                yield item;
            }
        }
    }
}
/**
 * This subclass of Promise will resolve to an instantiated Page once the request completes.
 *
 * It also implements AsyncIterable to allow auto-paginating iteration on an unawaited list call, eg:
 *
 *    for await (const item of client.items.list()) {
 *      console.log(item)
 *    }
 */
export class PagePromise extends APIPromise {
    constructor(client, request, Page) {
        super(client, request, async (client, props) => new Page(client, props.response, await defaultParseResponse(client, props), props.options));
    }
    /**
     * Allow auto-paginating iteration on an unawaited list call, eg:
     *
     *    for await (const item of client.items.list()) {
     *      console.log(item)
     *    }
     */
    async *[Symbol.asyncIterator]() {
        const page = await this;
        for await (const item of page) {
            yield item;
        }
    }
}
export class Page extends AbstractPage {
    constructor(client, response, body, options) {
        super(client, response, body, options);
        this.data = body.data || [];
        this.has_more = body.has_more || false;
        this.first_id = body.first_id || null;
        this.last_id = body.last_id || null;
    }
    getPaginatedItems() {
        return this.data ?? [];
    }
    hasNextPage() {
        if (this.has_more === false) {
            return false;
        }
        return super.hasNextPage();
    }
    nextPageRequestOptions() {
        if (this.options.query?.['before_id']) {
            // in reverse
            const first_id = this.first_id;
            if (!first_id) {
                return null;
            }
            return {
                ...this.options,
                query: {
                    ...maybeObj(this.options.query),
                    before_id: first_id,
                },
            };
        }
        const cursor = this.last_id;
        if (!cursor) {
            return null;
        }
        return {
            ...this.options,
            query: {
                ...maybeObj(this.options.query),
                after_id: cursor,
            },
        };
    }
}
export class TokenPage extends AbstractPage {
    constructor(client, response, body, options) {
        super(client, response, body, options);
        this.data = body.data || [];
        this.has_more = body.has_more || false;
        this.next_page = body.next_page || null;
    }
    getPaginatedItems() {
        return this.data ?? [];
    }
    hasNextPage() {
        if (this.has_more === false) {
            return false;
        }
        return super.hasNextPage();
    }
    nextPageRequestOptions() {
        const cursor = this.next_page;
        if (!cursor) {
            return null;
        }
        return {
            ...this.options,
            query: {
                ...maybeObj(this.options.query),
                page_token: cursor,
            },
        };
    }
}
export class PageCursor extends AbstractPage {
    constructor(client, response, body, options) {
        super(client, response, body, options);
        this.data = body.data || [];
        this.has_more = body.has_more || false;
        this.next_page = body.next_page || null;
    }
    getPaginatedItems() {
        return this.data ?? [];
    }
    hasNextPage() {
        if (this.has_more === false) {
            return false;
        }
        return super.hasNextPage();
    }
    nextPageRequestOptions() {
        const cursor = this.next_page;
        if (!cursor) {
            return null;
        }
        return {
            ...this.options,
            query: {
                ...maybeObj(this.options.query),
                page: cursor,
            },
        };
    }
}
//# sourceMappingURL=pagination.mjs.map                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           import { type ReadableStream } from "../internal/shim-types.mjs";
import type { BaseAnthropic } from "../client.mjs";
export type ServerSentEvent = {
    event: string | null;
    data: string;
    raw: string[];
};
export declare class Stream<Item> implements AsyncIterable<Item> {
    #private;
    private iterator;
    controller: AbortController;
    constructor(iterator: () => AsyncIterator<Item>, controller: AbortController, client?: BaseAnthropic);
    static fromSSEResponse<Item>(response: Response, controller: AbortController, client?: BaseAnthropic): Stream<Item>;
    /**
     * Generates a Stream from a newline-separated ReadableStream
     * where each item is a JSON value.
     */
    static fromReadableStream<Item>(readableStream: ReadableStream, controller: AbortController, client?: BaseAnthropic): Stream<Item>;
    [Symbol.asyncIterator](): AsyncIterator<Item>;
    /**
     * Splits the stream into two streams which can be
     * independently read from at different speeds.
     */
    tee(): [Stream<Item>, Stream<Item>];
    /**
     * Converts this stream to a newline-separated ReadableStream of
     * JSON stringified values in the stream
     * which can be turned back into a Stream with `Stream.fromReadableStream()`.
     */
    toReadableStream(): R