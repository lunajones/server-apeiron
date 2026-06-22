package navigation

type AreaCost map[string]float64

func (c AreaCost) Cost(areaType string) float64 {
	if c == nil {
		return 1
	}
	if value, ok := c[areaType]; ok && value > 0 {
		return value
	}
	return 1
}
