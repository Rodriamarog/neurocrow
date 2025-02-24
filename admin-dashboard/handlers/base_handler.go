package handlers

import (
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template"
	"net/http"
)

type BaseHandler struct {
	templateRenderer *template.Renderer
	auth             *auth.Authenticator
}

func (h *BaseHandler) requireAuth(w http.ResponseWriter, r *http.Request) (*auth.User, bool) {
	user := r.Context().Value("user").(*auth.User)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return nil, false
	}
	return user, true
}

func (h *BaseHandler) renderError(w http.ResponseWriter, err error, status int) {
	// Common error handling logic
}
