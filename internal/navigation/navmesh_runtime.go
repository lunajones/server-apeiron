package navigation

import "sync"

import domainmath "server-apeiron/internal/domain/math"

type NavPolygonID string

type NavMeshRuntime struct {
	mu               sync.RWMutex
	ID               string
	Version          int
	CoordinateSystem string
	Polygons         []NavPolygon
	DynamicBlockers  []DynamicBlocker
}

type NavPolygon struct {
	ID NavPolygonID
}

type PositionValidation struct {
	Valid         bool
	Walkable      bool
	Position      domainmath.Position
	Clamped       domainmath.Position
	Reason        string
	FailureReason string
}

func (n *NavMeshRuntime) ValidatePosition(position domainmath.Position) PositionValidation {
	return PositionValidation{Valid: true, Walkable: true, Position: position, Clamped: position}
}
