package controllers

type SkillController struct {
	BaseController
}

func NewSkillController(base BaseController) *SkillController {
	return &SkillController{BaseController: base}
}

func (c *SkillController) Update(ctx EntityContext) error {
	return c.Validate()
}
