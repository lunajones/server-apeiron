package controllers

type StatusEffectController struct {
	BaseController
}

func NewStatusEffectController(base BaseController) *StatusEffectController {
	return &StatusEffectController{BaseController: base}
}

func (c *StatusEffectController) Update(ctx EntityContext) error {
	return c.Validate()
}
