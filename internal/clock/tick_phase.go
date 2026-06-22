package clock

type TickPhase string

const (
	PhaseInput       TickPhase = "input"
	PhaseAI          TickPhase = "ai"
	PhaseMovement    TickPhase = "movement"
	PhaseCombat      TickPhase = "combat"
	PhaseSpawn       TickPhase = "spawn"
	PhasePersistence TickPhase = "persistence"
	PhaseSnapshot    TickPhase = "snapshot"
)

type PhaseFunc func(TickContext) error
