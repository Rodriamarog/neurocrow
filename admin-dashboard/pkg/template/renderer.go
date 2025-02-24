package template

import (
	"html/template"
	"log"
	"net/http"
)

type Renderer struct {
	Templates *template.Template
	layouts   map[string]*template.Template
}

func NewRenderer() *Renderer {
	r := &Renderer{
		layouts: make(map[string]*template.Template),
	}

	// Load all templates
	r.Templates = template.Must(template.ParseGlob("templates/**/*.html"))

	// Pre-compile layouts with their components
	r.layouts["base"] = template.Must(template.ParseFiles(
		"templates/layouts/base.html",
		"templates/components/nav.html",
		"templates/components/footer.html",
	))

	return r
}

func (r *Renderer) RenderPage(w http.ResponseWriter, name string, data interface{}) error {
	layout := r.layouts["base"]
	return layout.ExecuteTemplate(w, name, data)
}

func (r *Renderer) RenderError(w http.ResponseWriter, err error) {
	log.Printf("❌ Error: %v", err)
	data := map[string]interface{}{
		"Error": err.Error(),
	}
	if renderErr := r.RenderPage(w, "error", data); renderErr != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
