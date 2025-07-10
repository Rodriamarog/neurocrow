package template

import (
	"html/template"
	"net/http"
)

type Renderer struct {
	templates *template.Template
}

func (r *Renderer) RenderPage(w http.ResponseWriter, name string, data interface{}) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func (r *Renderer) RenderPartial(w http.ResponseWriter, name string, data interface{}) error {
	return r.templates.ExecuteTemplate(w, name, data)
}

func (r *Renderer) RenderError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (r *Renderer) GetTemplates() *template.Template {
	return r.templates
}

func NewRenderer() *Renderer {
	r := &Renderer{
		templates: template.Must(template.ParseGlob("templates/**/*.html")),
	}
	return r
}
