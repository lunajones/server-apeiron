package navigation

import domainmath "server-apeiron/internal/domain/math"

type PositionValidator struct {
	NavMesh *NavMeshRuntime
}

func NewPositionValidator(navmesh *NavMeshRuntime) *PositionValidator {
	return &PositionValidator{NavMesh: navmesh}
}

func (v *PositionValidator) Validate(point domainmath.Vec3) PositionValidation {
	if v == nil || v.NavMesh == nil {
		return PositionValidation{
			Walkable:      false,
			Clamped:       point,
			FailureReason: "navmesh is not configured",
		}
	}

	return v.NavMesh.ValidatePosition(point)
}
