package gamefsm

import "server-apeiron/internal/fsm"

const (
	LifeSpawning   fsm.StateID = "spawning"
	LifeAlive      fsm.StateID = "alive"
	LifeDowned     fsm.StateID = "downed"
	LifeDead       fsm.StateID = "dead"
	LifeDespawning fsm.StateID = "despawning"

	MovementIdle        fsm.StateID = "idle"
	MovementMoving      fsm.StateID = "moving"
	MovementDodging     fsm.StateID = "dodging"
	MovementLeaping     fsm.StateID = "leaping"
	MovementKnockedBack fsm.StateID = "knocked_back"
	MovementRooted      fsm.StateID = "rooted"
	MovementStaggered   fsm.StateID = "staggered"

	CombatOutOfCombat fsm.StateID = "out_of_combat"
	CombatEngaging    fsm.StateID = "engaging"
	CombatAttacking   fsm.StateID = "attacking"
	CombatRecovering  fsm.StateID = "recovering"
	CombatDisengaging fsm.StateID = "disengaging"

	SkillReady       fsm.StateID = "ready"
	SkillWindup      fsm.StateID = "windup"
	SkillActive      fsm.StateID = "active"
	SkillRecovery    fsm.StateID = "recovery"
	SkillCooldown    fsm.StateID = "cooldown"
	SkillInterrupted fsm.StateID = "interrupted"
	SkillCancelled   fsm.StateID = "cancelled"

	DefenseNeutral       fsm.StateID = "neutral"
	DefenseBlocking      fsm.StateID = "blocking"
	DefenseParryStartup  fsm.StateID = "parry_startup"
	DefenseParryActive   fsm.StateID = "parry_active"
	DefenseParryRecovery fsm.StateID = "parry_recovery"
	DefenseGuardBroken   fsm.StateID = "guard_broken"
	DefenseIFrame        fsm.StateID = "iframe"

	AIIdle        fsm.StateID = "idle"
	AIPatrol      fsm.StateID = "patrol"
	AIInvestigate fsm.StateID = "investigate"
	AIChase       fsm.StateID = "chase"
	AICombat      fsm.StateID = "combat"
	AIFlee        fsm.StateID = "flee"
	AIReturnHome  fsm.StateID = "return_home"
	AIRest        fsm.StateID = "rest"
	AISearchFood  fsm.StateID = "search_food"
	AISearchWater fsm.StateID = "search_water"

	PerceptionUnaware    fsm.StateID = "unaware"
	PerceptionSuspicious fsm.StateID = "suspicious"
	PerceptionAlert      fsm.StateID = "alert"
	PerceptionTracking   fsm.StateID = "tracking"
	PerceptionLostTarget fsm.StateID = "lost_target"

	NeedsStable       fsm.StateID = "stable"
	NeedsSeekingFood  fsm.StateID = "seeking_food"
	NeedsSeekingWater fsm.StateID = "seeking_water"
	NeedsExhausted    fsm.StateID = "exhausted"
	NeedsStressed     fsm.StateID = "stressed"
	NeedsAfraid       fsm.StateID = "afraid"
	NeedsAggressive   fsm.StateID = "aggressive"
	NeedsReliefNeeded fsm.StateID = "relief_needed"

	AggroCalm            fsm.StateID = "calm"
	AggroProvoked        fsm.StateID = "provoked"
	AggroLockedTarget    fsm.StateID = "locked_target"
	AggroSwitchingTarget fsm.StateID = "switching_target"
	AggroLeashing        fsm.StateID = "leashing"
	AggroReset           fsm.StateID = "reset"

	TargetingNoTarget  fsm.StateID = "no_target"
	TargetingAcquiring fsm.StateID = "acquiring"
	TargetingLocked    fsm.StateID = "locked"
	TargetingLost      fsm.StateID = "lost"
	TargetingInvalid   fsm.StateID = "invalid"

	ThreatNone        fsm.StateID = "no_threat"
	ThreatEvaluating  fsm.StateID = "evaluating"
	ThreatEscalating  fsm.StateID = "escalating"
	ThreatOverwhelmed fsm.StateID = "overwhelmed"
	ThreatSafe        fsm.StateID = "safe"

	StatusClean           fsm.StateID = "clean"
	StatusAffected        fsm.StateID = "affected"
	StatusCrowdControlled fsm.StateID = "crowd_controlled"
	StatusImmune          fsm.StateID = "immune"
	StatusExpired         fsm.StateID = "expired"

	CrowdControlFree        fsm.StateID = "free"
	CrowdControlStaggered   fsm.StateID = "staggered"
	CrowdControlStunned     fsm.StateID = "stunned"
	CrowdControlRooted      fsm.StateID = "rooted"
	CrowdControlFeared      fsm.StateID = "feared"
	CrowdControlTaunted     fsm.StateID = "taunted"
	CrowdControlKnockedDown fsm.StateID = "knocked_down"

	SpawnInactive fsm.StateID = "inactive"
	SpawnWaiting  fsm.StateID = "waiting"
	SpawnSpawning fsm.StateID = "spawning"
	SpawnActive   fsm.StateID = "active"
	SpawnCooldown fsm.StateID = "cooldown"
	SpawnDisabled fsm.StateID = "disabled"

	EncounterDormant   fsm.StateID = "dormant"
	EncounterPreparing fsm.StateID = "preparing"
	EncounterActive    fsm.StateID = "active"
	EncounterVictory   fsm.StateID = "victory"
	EncounterFailed    fsm.StateID = "failed"
	EncounterReset     fsm.StateID = "reset"

	InteractionNone        fsm.StateID = "none"
	InteractionInspecting  fsm.StateID = "inspecting"
	InteractionGathering   fsm.StateID = "gathering"
	InteractionLooting     fsm.StateID = "looting"
	InteractionTalking     fsm.StateID = "talking"
	InteractionUsingObject fsm.StateID = "using_object"

	LootUnavailable fsm.StateID = "unavailable"
	LootReserved    fsm.StateID = "reserved"
	LootOpen        fsm.StateID = "open"
	LootLooted      fsm.StateID = "looted"
	LootExpired     fsm.StateID = "expired"

	CorpseFresh    fsm.StateID = "fresh"
	CorpseLootable fsm.StateID = "lootable"
	CorpseDecaying fsm.StateID = "decaying"
	CorpseRemoved  fsm.StateID = "removed"

	ProjectileSpawned    fsm.StateID = "spawned"
	ProjectileTravelling fsm.StateID = "travelling"
	ProjectileArmed      fsm.StateID = "armed"
	ProjectileImpacted   fsm.StateID = "impacted"
	ProjectileExpired    fsm.StateID = "expired"

	AreaEffectPending fsm.StateID = "pending"
	AreaEffectActive  fsm.StateID = "active"
	AreaEffectPulsing fsm.StateID = "pulsing"
	AreaEffectFading  fsm.StateID = "fading"
	AreaEffectExpired fsm.StateID = "expired"

	SessionConnecting     fsm.StateID = "connecting"
	SessionAuthenticating fsm.StateID = "authenticating"
	SessionLoading        fsm.StateID = "loading"
	SessionAttached       fsm.StateID = "attached"
	SessionPlaying        fsm.StateID = "playing"
	SessionDisconnecting  fsm.StateID = "disconnecting"
	SessionReconnecting   fsm.StateID = "reconnecting"
	SessionDetached       fsm.StateID = "detached"

	PvPSafe         fsm.StateID = "safe"
	PvPFlagged      fsm.StateID = "flagged"
	PvPCombatLocked fsm.StateID = "combat_locked"
	PvPCriminal     fsm.StateID = "criminal_future"
	PvPCooldown     fsm.StateID = "cooldown"

	PersistenceClean       fsm.StateID = "clean"
	PersistenceDirty       fsm.StateID = "dirty"
	PersistenceQueued      fsm.StateID = "queued"
	PersistenceSaving      fsm.StateID = "saving"
	PersistenceFailedRetry fsm.StateID = "failed_retry"
	PersistenceSaved       fsm.StateID = "saved"
)

func NewCreatureLifeFSM() *Machine {
	m := Must(NewMachine("creature_life", LifeSpawning, LifeSpawning, LifeAlive, LifeDowned, LifeDead, LifeDespawning))
	addBoolTransition(m, LifeSpawning, "spawn.complete", LifeAlive, "spawn completed")
	addBoolTransition(m, LifeAlive, "life.downed", LifeDowned, "health reached downed threshold")
	addBoolTransition(m, LifeAlive, "life.dead", LifeDead, "health depleted")
	addBoolTransition(m, LifeDowned, "life.revived", LifeAlive, "revived")
	addBoolTransition(m, LifeDowned, "life.dead", LifeDead, "downed entity died")
	addBoolTransition(m, LifeDead, "despawn.requested", LifeDespawning, "despawn requested")
	return m
}

func NewMovementFSM() *Machine {
	m := Must(NewMachine("movement", MovementIdle, MovementIdle, MovementMoving, MovementDodging, MovementLeaping, MovementKnockedBack, MovementRooted, MovementStaggered))
	addBoolTransition(m, MovementIdle, "movement.has_intent", MovementMoving, "movement intent received")
	addBoolTransition(m, MovementMoving, "movement.no_intent", MovementIdle, "movement intent cleared")
	addBoolTransition(m, MovementIdle, "movement.dodge", MovementDodging, "dodge requested")
	addBoolTransition(m, MovementMoving, "movement.dodge", MovementDodging, "dodge requested")
	addBoolTransition(m, MovementDodging, "movement.recovered", MovementIdle, "dodge recovered")
	addBoolTransition(m, MovementMoving, "cc.rooted", MovementRooted, "root applied")
	addBoolTransition(m, MovementRooted, "cc.cleared", MovementIdle, "root cleared")
	return m
}

func NewCombatFSM() *Machine {
	m := Must(NewMachine("combat", CombatOutOfCombat, CombatOutOfCombat, CombatEngaging, CombatAttacking, CombatRecovering, CombatDisengaging))
	addBoolTransition(m, CombatOutOfCombat, "combat.target_acquired", CombatEngaging, "target acquired")
	addBoolTransition(m, CombatEngaging, "combat.attack_started", CombatAttacking, "attack started")
	addBoolTransition(m, CombatAttacking, "combat.attack_resolved", CombatRecovering, "attack resolved")
	addBoolTransition(m, CombatRecovering, "combat.recovered", CombatEngaging, "attack recovered")
	addBoolTransition(m, CombatEngaging, "combat.disengage", CombatDisengaging, "disengage requested")
	addBoolTransition(m, CombatDisengaging, "combat.left", CombatOutOfCombat, "combat left")
	return m
}

func NewSkillFSM() *Machine {
	m := Must(NewMachine("skill", SkillReady, SkillReady, SkillWindup, SkillActive, SkillRecovery, SkillCooldown, SkillInterrupted, SkillCancelled))
	addBoolTransition(m, SkillReady, "skill.cast_requested", SkillWindup, "cast requested")
	addBoolTransition(m, SkillWindup, "skill.windup_complete", SkillActive, "windup completed")
	addBoolTransition(m, SkillActive, "skill.active_complete", SkillRecovery, "active window completed")
	addBoolTransition(m, SkillRecovery, "skill.recovery_complete", SkillCooldown, "recovery completed")
	addBoolTransition(m, SkillCooldown, "skill.cooldown_complete", SkillReady, "cooldown completed")
	addBoolTransition(m, SkillWindup, "skill.interrupted", SkillInterrupted, "skill interrupted")
	addBoolTransition(m, SkillWindup, "skill.cancelled", SkillCancelled, "skill cancelled")
	addBoolTransition(m, SkillInterrupted, "skill.reset", SkillReady, "skill reset")
	addBoolTransition(m, SkillCancelled, "skill.reset", SkillReady, "skill reset")
	return m
}

func NewDefenseFSM() *Machine {
	return Must(NewMachine("defense", DefenseNeutral, DefenseNeutral, DefenseBlocking, DefenseParryStartup, DefenseParryActive, DefenseParryRecovery, DefenseGuardBroken, DefenseIFrame))
}

func NewAIFSM() *Machine {
	return Must(NewMachine("ai", AIIdle, AIIdle, AIPatrol, AIInvestigate, AIChase, AICombat, AIFlee, AIReturnHome, AIRest, AISearchFood, AISearchWater))
}

func NewPerceptionFSM() *Machine {
	return Must(NewMachine("perception", PerceptionUnaware, PerceptionUnaware, PerceptionSuspicious, PerceptionAlert, PerceptionTracking, PerceptionLostTarget))
}

func NewNeedsFSM() *Machine {
	return Must(NewMachine("needs", NeedsStable, NeedsStable, NeedsSeekingFood, NeedsSeekingWater, NeedsExhausted, NeedsStressed, NeedsAfraid, NeedsAggressive, NeedsReliefNeeded))
}

func NewAggroFSM() *Machine {
	return Must(NewMachine("aggro", AggroCalm, AggroCalm, AggroProvoked, AggroLockedTarget, AggroSwitchingTarget, AggroLeashing, AggroReset))
}

func NewTargetingFSM() *Machine {
	return Must(NewMachine("targeting", TargetingNoTarget, TargetingNoTarget, TargetingAcquiring, TargetingLocked, TargetingLost, TargetingInvalid))
}

func NewThreatFSM() *Machine {
	return Must(NewMachine("threat", ThreatNone, ThreatNone, ThreatEvaluating, ThreatEscalating, ThreatOverwhelmed, ThreatSafe))
}

func NewStatusEffectFSM() *Machine {
	return Must(NewMachine("status_effect", StatusClean, StatusClean, StatusAffected, StatusCrowdControlled, StatusImmune, StatusExpired))
}

func NewCrowdControlFSM() *Machine {
	return Must(NewMachine("crowd_control", CrowdControlFree, CrowdControlFree, CrowdControlStaggered, CrowdControlStunned, CrowdControlRooted, CrowdControlFeared, CrowdControlTaunted, CrowdControlKnockedDown))
}

func NewSpawnFSM() *Machine {
	return Must(NewMachine("spawn", SpawnInactive, SpawnInactive, SpawnWaiting, SpawnSpawning, SpawnActive, SpawnCooldown, SpawnDisabled))
}

func NewEncounterFSM() *Machine {
	return Must(NewMachine("encounter", EncounterDormant, EncounterDormant, EncounterPreparing, EncounterActive, EncounterVictory, EncounterFailed, EncounterReset))
}

func NewInteractionFSM() *Machine {
	return Must(NewMachine("interaction", InteractionNone, InteractionNone, InteractionInspecting, InteractionGathering, InteractionLooting, InteractionTalking, InteractionUsingObject))
}

func NewLootFSM() *Machine {
	return Must(NewMachine("loot", LootUnavailable, LootUnavailable, LootReserved, LootOpen, LootLooted, LootExpired))
}

func NewCorpseFSM() *Machine {
	return Must(NewMachine("corpse", CorpseFresh, CorpseFresh, CorpseLootable, CorpseDecaying, CorpseRemoved))
}

func NewProjectileFSM() *Machine {
	return Must(NewMachine("projectile", ProjectileSpawned, ProjectileSpawned, ProjectileTravelling, ProjectileArmed, ProjectileImpacted, ProjectileExpired))
}

func NewAreaEffectFSM() *Machine {
	return Must(NewMachine("area_effect", AreaEffectPending, AreaEffectPending, AreaEffectActive, AreaEffectPulsing, AreaEffectFading, AreaEffectExpired))
}

func NewPlayerSessionFSM() *Machine {
	return Must(NewMachine("player_session", SessionConnecting, SessionConnecting, SessionAuthenticating, SessionLoading, SessionAttached, SessionPlaying, SessionDisconnecting, SessionReconnecting, SessionDetached))
}

func NewPvPFlagFSM() *Machine {
	return Must(NewMachine("pvp_flag", PvPSafe, PvPSafe, PvPFlagged, PvPCombatLocked, PvPCriminal, PvPCooldown))
}

func NewPersistenceFSM() *Machine {
	return Must(NewMachine("persistence", PersistenceClean, PersistenceClean, PersistenceDirty, PersistenceQueued, PersistenceSaving, PersistenceFailedRetry, PersistenceSaved))
}
