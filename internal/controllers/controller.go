package controllers

type BaseController struct {
	Name string
}

func NewBaseController(name string) BaseController {
	return BaseController{Name: name}
}

func (c BaseController) Validate() error {
	return nil
}
