package handlers

import (
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template"
	"admin-dashboard/pkg/views"
	"admin-dashboard/services"
	"net/http"
)

type MessageHandler struct {
	messageService *services.MessageService
	renderer       *template.Renderer
}

func NewMessageHandler(ms *services.MessageService, r *template.Renderer) *MessageHandler {
	return &MessageHandler{
		messageService: ms,
		renderer:       r,
	}
}

func (h *MessageHandler) GetMessageList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := auth.UserFromContext(ctx)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	messages, err := h.messageService.GetMessages(ctx, user.ClientID)
	if err != nil {
		h.renderer.RenderError(w, err)
		return
	}

	view := &views.MessageListView{
		Messages: views.ToMessageViews(messages),
		User:     views.ToUserView(user),
		Filters: views.FilterOptions{
			StartDate: "",
			EndDate:   "",
			Platform:  "",
			Status:    "",
		},
	}

	h.renderer.RenderPage(w, "message-list", view)
}
