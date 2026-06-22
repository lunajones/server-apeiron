package controllers

type PerceptionController struct {
	BaseController
}

func NewPerceptionController(base BaseController) *PerceptionController {
	return &PerceptionController{BaseController: base}
}

func (c *PerceptionController) Update(ctx EntityContext) error {
	return c.Validate()
}
