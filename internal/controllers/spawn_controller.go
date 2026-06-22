package controllers

type SpawnController struct {
	BaseController
}

func NewSpawnController(base BaseController) *SpawnController {
	return &SpawnController{BaseController: base}
}

func (c *SpawnController) Update(ctx EntityContext) error {
	return c.Validate()
}
