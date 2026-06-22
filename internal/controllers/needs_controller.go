package controllers

type NeedsController struct {
	BaseController
}

func NewNeedsController(base BaseController) *NeedsController {
	return &NeedsController{BaseController: base}
}

func (c *NeedsController) Update(ctx EntityContext) error {
	return c.Validate()
}
