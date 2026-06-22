package world

import domainmath "server-apeiron/internal/domain/math"

type PlayerSpawnDefinition struct {
	ID       string          `json:"id"`
	Name     string          `json:"name,omitempty"`
	Default  bool            `json:"default,omitempty"`
	Position domainmath.Vec3 `json:"position"`
	Rotation domainmath.Vec3 `json:"rotation,omitempty"`
}
