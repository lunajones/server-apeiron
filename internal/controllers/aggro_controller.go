package controllers

type AggroController struct {
	BaseController
}

func NewAggroController(base BaseController) *AggroController {
	return &AggroController{BaseController: base}
}

func (c *AggroController) Update(ctx EntityContext) error {
	return c.Validate()
}
