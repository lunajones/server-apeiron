package controllers

type AIController struct {
	BaseController
}

func NewAIController(base BaseController) *AIController {
	return &AIController{BaseController: base}
}

func (c *AIController) Update(ctx EntityContext) error {
	return c.Validate()
}
