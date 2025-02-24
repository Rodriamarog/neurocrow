package template

import (
	"html/template"
	"net/http"
)

type Renderer struct {
	templates *template.Template
}

func NewRenderer() *Renderer {
	return &Renderer{
		templates: Templates,
	}
}

func (r *Renderer) Render(w http.ResponseWriter, name string, data interface{}) error {
	return RenderTemplate(w, name, data)
}
