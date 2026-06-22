package controllers

type CombatController struct {
	BaseController
}

func NewCombatController(base BaseController) *CombatController {
	return &CombatController{BaseController: base}
}

func (c *CombatController) Update(ctx EntityContext) error {
	return c.Validate()
}
