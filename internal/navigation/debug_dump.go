package navigation

type DebugDump struct {
	ID               string
	Version          int
	CoordinateSystem string
	PolygonCount     int
	DynamicBlockers  int
}

func (n *NavMeshRuntime) DebugDump() DebugDump {
	n.mu.RLock()
	defer n.mu.RUnlock()

	return DebugDump{
		ID:               n.ID,
		Version:          n.Version,
		CoordinateSystem: n.CoordinateSystem,
		PolygonCount:     len(n.Polygons),
		DynamicBlockers:  len(n.DynamicBlockers),
	}
}
