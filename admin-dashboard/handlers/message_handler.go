package handlers

import (
	"admin-dashboard/pkg/auth"
	"admin-dashboard/pkg/template"
	"admin-dashboard/pkg/views"
	"admin-dashboard/services"
	"net/http"
	"strconv"
	"time"
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

	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	params := views.MessageListParams{
		ClientID:  user.ClientID,
		ThreadID:  r.URL.Query().Get("thread_id"),
		Cursor:    r.URL.Query().Get("cursor"),
		Page:      page,
		PageSize:  h.getPageSize(r),
		StartDate: h.parseDate(r.URL.Query().Get("start_date")),
		EndDate:   h.parseDate(r.URL.Query().Get("end_date")),
		Platform:  r.URL.Query().Get("platform"),
		Status:    r.URL.Query().Get("status"),
	}

	response, err := h.messageService.GetMessages(ctx, params)
	if err != nil {
		h.renderer.RenderError(w, err)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		h.renderer.RenderPartial(w, "message-list-items", response)
		return
	}

	h.renderer.RenderPage(w, "message-list", response)
}

func (h *MessageHandler) getPageSize(r *http.Request) int {
	if sizeStr := r.URL.Query().Get("page_size"); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil {
			if size > 0 && size <= h.messageService.Config.MaxPageSize {
				return size
			}
		}
	}
	return h.messageService.Config.DefaultPageSize
}

func (h *MessageHandler) parseDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return time.Time{}
	}
	return date
}
