package controllers

type DefenseController struct {
	BaseController
}

func NewDefenseController(base BaseController) *DefenseController {
	return &DefenseController{BaseController: base}
}

func (c *DefenseController) Update(ctx EntityContext) error {
	return c.Validate()
}
