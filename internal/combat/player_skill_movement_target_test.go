package combat

import (
	"math"
	"testing"

	domainentity "server-apeiron/internal/domain/entity"
	"server-apeiron/internal/domain/ids"
	domainmath "server-apeiron/internal/domain/math"
	"server-apeiron/internal/hitbox"
	"server-apeiron/internal/movement"
	"server-apeiron/internal/skill"
)

func TestCommitPendingPlayerSkillForwardDistanceIgnoresTargetAndMouseAnchors(t *testing.T) {
	source := combatEntity(1000, "region-test", 0, 0)
	source.SetPosition(domainmath.V3(0, 0, 0))
	source.components.Movement.Locomotion.AuthoritativeYaw = 0
	source.components.Transform.RotationY = 0

	target := combatEntity(2000, "region-test", 90, 220)
	target.entityType = domainentity.EntityTypeCreature
	target.SetPosition(domainmath.V3(90, 220, 0))

	pending := pendingPlayerSkillAction{
		Intent: skill.Intent{
			SkillID:        "player_shield_rush",
			HasAim:         true,
			AimDirection:   domainmath.V3(1, 0, 0),
			HasTarget:      true,
			Target:         skill.TargetRef{RuntimeID: target.RuntimeID()},
			HasPosition:    true,
			TargetPosition: domainmath.V3(30, -400, 0),
		},
		HasMovementContract: true,
		MovementContract: movement.MovementActionContract{
			ID:                   "shield_rush_front_contact_v1",
			MovementAction:       "grounded_skill",
			MovementType:         "grounded_skill",
			HorizontalDistanceCM: 470,
			BaseSpeedCMPerSec:    620,
		},
	}
	resolver := hitbox.EntityResolverFunc(func(id ids.RuntimeEntityID) (domainentity.Entity, bool) {
		if id == target.RuntimeID() {
			return target, true
		}
		return nil, false
	})

	got := commitPendingPlayerSkillMovementTarget(source, resolver, pending, skillMovementConfig{})

	if !got.MovementTargetCommitted {
		t.Fatal("movement target was not committed")
	}
	if math.Abs(got.CommittedTargetPosition.X-470) > 0.0001 || math.Abs(got.CommittedTargetPosition.Y) > 0.0001 || math.Abs(got.CommittedTargetPosition.Z) > 0.0001 {
		t.Fatalf("committed target = %+v, want forward distance (470,0,0)", got.CommittedTargetPosition)
	}
	if math.Abs(got.CommittedMoveDirection.X-1) > 0.0001 || math.Abs(got.CommittedMoveDirection.Y) > 0.0001 {
		t.Fatalf("committed direction = %+v, want player forward", got.CommittedMoveDirection)
	}
}
