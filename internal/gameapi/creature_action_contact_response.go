package gameapi

import (
	"math"

	dbv1 "db-apeiron/gen/apeiron/v1"
	"server-apeiron/internal/combat/contactpolicy"
)

type creatureActionContactRuntime struct {
	Policy             string
	AllowsPassthrough  bool
	StopsAtContact     bool
	AppliesPush        bool
	CarriesTarget      bool
	StopDistanceCM     float64
	RequiredTargetKind string
}

type actionMotionContactResponse struct {
	Applied    bool
	Stopped    bool
	Position   vector
	Velocity   vector
	DistanceCM float64
}

func creatureActionContactRuntimeFromContract(contract SkillRuntimeContract) creatureActionContactRuntime {
	classification := contactpolicy.Classify(contract.ContactPolicy, contract.MovementAction.ContactPolicy)
	return creatureActionContactRuntime{
		Policy:            classification.Canonical,
		AllowsPassthrough: classification.AllowsPassthrough,
		StopsAtContact:    classification.StopsAtContact,
		AppliesPush:       classification.AppliesPush,
		CarriesTarget:     classification.CarriesTarget,
		StopDistanceCM:    creatureSkillContactStopDistanceCM(contract),
	}
}

func creatureSkillContactStopDistanceCM(contract SkillRuntimeContract) float64 {
	stopDistance := 0.0
	for _, profile := range contract.Hitboxes {
		stopDistance = maxFloat64(stopDistance, hitboxProfileContactRadiusCM(profile))
		if profile == nil || profile.GetMotionProfile() == nil {
			continue
		}
		for _, sample := range profile.GetMotionProfile().GetSamples() {
			stopDistance = maxFloat64(stopDistance, hitboxMotionSampleContactRadiusCM(sample))
		}
	}
	return stopDistance
}

func hitboxProfileContactRadiusCM(profile *dbv1.SkillHitboxProfile) float64 {
	if profile == nil {
		return 0
	}
	return maxFloat64(
		positiveFloat64(profile.GetRadius()),
		positiveFloat64(profile.GetSizeY()/2),
		positiveFloat64(profile.GetSizeX()/2),
	)
}

func hitboxMotionSampleContactRadiusCM(sample *dbv1.SkillHitboxMotionSample) float64 {
	if sample == nil {
		return 0
	}
	return maxFloat64(
		positiveFloat64(sample.GetRadius()),
		positiveFloat64(sample.GetSizeY()/2),
		positiveFloat64(sample.GetSizeX()/2),
	)
}

func (r *Runtime) resolveActionMotionContactResponseLocked(entity *entityState, motion *actionMotionState, projected vector, velocity vector, distanceCM float64) actionMotionContactResponse {
	out := actionMotionContactResponse{Position: projected, Velocity: velocity, DistanceCM: distanceCM}
	if entity == nil || motion == nil {
		return out
	}
	if motion.AllowsPassthrough || !motion.StopsAtContact || motion.ContactTargetID == 0 || motion.ContactStopCM <= 0 {
		return out
	}
	target := r.entities[motion.ContactTargetID]
	if target == nil {
		return out
	}
	dir := normalize(vector{x: motion.Direction.x, y: motion.Direction.y})
	if dir == (vector{}) {
		return out
	}
	toTarget := vector{x: target.position.x - motion.StartPosition.x, y: target.position.y - motion.StartPosition.y}
	targetAlongPath := dot2D(toTarget, dir)
	if targetAlongPath <= 0 {
		return out
	}
	stopTravel := math.Max(0, targetAlongPath-motion.ContactStopCM)
	currentTravel := dot2D(vector{x: projected.x - motion.StartPosition.x, y: projected.y - motion.StartPosition.y}, dir)
	if currentTravel < stopTravel {
		return out
	}
	stoppedPosition := add(motion.StartPosition, scale(dir, stopTravel))
	stoppedPosition.z = motion.StartPosition.z
	return actionMotionContactResponse{
		Applied:    true,
		Stopped:    true,
		Position:   stoppedPosition,
		Velocity:   vector{},
		DistanceCM: stopTravel,
	}
}

func dot2D(a, b vector) float64 {
	return a.x*b.x + a.y*b.y
}

func positiveFloat64(value float64) float64 {
	if value > 0 && !math.IsNaN(value) {
		return value
	}
	return 0
}
