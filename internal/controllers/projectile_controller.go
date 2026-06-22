package controllers

type ProjectileController struct {
	BaseController
}

func NewProjectileController(base BaseController) *ProjectileController {
	return &ProjectileController{BaseController: base}
}

func (c *ProjectileController) Update(ctx EntityContext) error {
	return c.Validate()
}
