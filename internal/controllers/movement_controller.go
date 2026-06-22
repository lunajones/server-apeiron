package controllers

type MovementController struct {
	BaseController
}

func NewMovementController(base BaseController) *MovementController {
	return &MovementController{BaseController: base}
}

func (c *MovementController) Update(ctx EntityContext) error {
	return c.Validate()
}
