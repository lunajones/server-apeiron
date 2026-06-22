package controllers

type EncounterController struct {
	BaseController
}

func NewEncounterController(base BaseController) *EncounterController {
	return &EncounterController{BaseController: base}
}

func (c *EncounterController) Update(ctx EntityContext) error {
	return c.Validate()
}
