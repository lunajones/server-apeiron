package movement

import (
	"math"
	"time"

	domainmath "server-apeiron/internal/domain/math"
)

// Locomotion is the single canonical locomotion state the server publishes for an
// entity in a given tick.
//
// Per the skill-movement-authority architecture (recuperação 5 / chat final), the
// movement resolver is the ONLY producer of these fields. Combat must emit
// intent/timeline only and never assemble or overwrite locomotion directly, and the
// gameapi runtime must publish what the resolver produced — not its own parallel
// version. This field set is the anti-rubberband parity contract: normal movement
// (walk/run/turn/dodge/leap) and skill movement resolve through the SAME fields so the
// client reconciles a skill exactly like it reconciles a walk.
type Locomotion struct {
	MovementMode            string
	Action                  string
	AbilityKey              string
	Phase                   string
	ReconciliationMode      string
	PhaseElapsedMS          int32
	PhaseRemainingMS        int32
	DurationMS              int32
	StartupMS               int32
	ActiveMS                int32
	RecoveryMS              int32
	TargetSpeed             float64
	EffectiveSpeed          float64
	PhaseSpeedScale         float64
	ActionDistanceTraveled  float64
	ActionStartPosition     domainmath.Position
	ActionProjectedPosition domainmath.Position
	HasActionPositions      bool
	PhaseWindowPolicy       string
	PredictionErrorPolicy   string
	ActionContractID        string
	MovementType            string
}

// Resolver owns locomotion production. It is the single authority that turns a movement
// action contract plus the action's elapsed time into a published Locomotion. Both the
// normal-movement path and the skill-movement path must go through this resolver; that
// unification is what removes the end-of-skill (and mid-skill) rubberband.
type Resolver struct{}

// NewResolver builds a stateless locomotion resolver.
func NewResolver() *Resolver { return &Resolver{} }

// Default policy values, matching the historical locomotionFromContract behaviour so
// migrating the existing callers onto the resolver does not change normal movement.
const (
	defaultLocomotionDurationMS        = 180
	defaultLocomotionActiveMS          = 120
	defaultLocomotionRecoveryMS        = 60
	defaultReconciliationMode          = "grounded_move_reconciliation"
	defaultPhaseWindowPolicy           = "server_authoritative"
	defaultLocomotionPredictionPolicy  = "bounded_smooth_correction"
	defaultLocomotionMovementModeValue = "grounded"
)

// LocomotionInput is the resolver's domain input: the authoritative contract plus the
// action's current timing and positions. Every field published in Locomotion is derived
// here and nowhere else.
type LocomotionInput struct {
	MovementMode            string
	AbilityKey              string
	Contract                MovementActionContract
	Phase                   string // optional; computed from Elapsed when empty
	Elapsed                 time.Duration
	ActionStartPosition     domainmath.Position
	ActionProjectedPosition domainmath.Position
	HasActionPositions      bool
}

// Resolve produces the canonical Locomotion for an action. This is the single place
// reconciliation mode, phase windows, distance and speed are decided.
func (r *Resolver) Resolve(in LocomotionInput) Locomotion {
	c := in.Contract

	duration := orDefaultInt32(c.DurationMS, defaultLocomotionDurationMS)
	active := orDefaultInt32(c.ActiveMS, defaultLocomotionActiveMS)
	recovery := orDefaultInt32(c.RecoveryMS, defaultLocomotionRecoveryMS)

	phase := in.Phase
	var elapsedMS, remainingMS int32
	if phase == "" {
		phase, elapsedMS, remainingMS = r.ResolvePhase(in.Elapsed, c.StartupMS, active, recovery)
	}

	return Locomotion{
		MovementMode:            orDefaultString(in.MovementMode, orDefaultString(c.MovementMode, defaultLocomotionMovementModeValue)),
		Action:                  c.MovementAction,
		AbilityKey:              in.AbilityKey,
		Phase:                   phase,
		ReconciliationMode:      orDefaultString(c.ReconciliationMode, defaultReconciliationMode),
		PhaseElapsedMS:          elapsedMS,
		PhaseRemainingMS:        remainingMS,
		DurationMS:              duration,
		StartupMS:               c.StartupMS,
		ActiveMS:                active,
		RecoveryMS:              recovery,
		TargetSpeed:             c.BaseSpeedCMPerSec,
		EffectiveSpeed:          c.BaseSpeedCMPerSec,
		PhaseSpeedScale:         1,
		ActionDistanceTraveled:  c.HorizontalDistanceCM,
		ActionStartPosition:     in.ActionStartPosition,
		ActionProjectedPosition: in.ActionProjectedPosition,
		HasActionPositions:      in.HasActionPositions,
		PhaseWindowPolicy:       defaultPhaseWindowPolicy,
		PredictionErrorPolicy:   orDefaultString(c.PredictionErrorPolicy, defaultLocomotionPredictionPolicy),
		ActionContractID:        c.ID,
		MovementType:            orDefaultString(c.MovementType, c.MovementAction),
	}
}

// ResolvePhase is the single authority for action phase windows. Ported from the combat
// skill phase math (pendingPlayerSkillLocomotionPhase) so skill movement and normal
// movement compute phase/elapsed/remaining the same way — the divergence here was a
// direct cause of end-of-skill rubberband.
func (r *Resolver) ResolvePhase(elapsed time.Duration, startupMS, activeMS, recoveryMS int32) (phase string, elapsedMS int32, remainingMS int32) {
	startup := msToDuration(startupMS)
	active := msToDuration(activeMS)
	recovery := msToDuration(recoveryMS)

	if elapsed < startup {
		return "startup", roundMillis(elapsed), roundMillis(startup - elapsed)
	}
	activeElapsed := elapsed - startup
	if activeElapsed < active {
		return "active", roundMillis(activeElapsed), roundMillis(active - activeElapsed)
	}
	recoveryElapsed := elapsed - startup - active
	if recovery <= 0 || recoveryElapsed >= recovery {
		return "recovery", roundMillis(recovery), 0
	}
	return "recovery", roundMillis(recoveryElapsed), roundMillis(recovery - recoveryElapsed)
}

func msToDuration(ms int32) time.Duration {
	if ms <= 0 {
		return 0
	}
	return time.Duration(ms) * time.Millisecond
}

func roundMillis(d time.Duration) int32 {
	if d <= 0 {
		return 0
	}
	return int32(math.Round(d.Seconds() * 1000))
}

func orDefaultInt32(value, fallback int32) int32 {
	if value <= 0 {
		return fallback
	}
	return value
}

func orDefaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
