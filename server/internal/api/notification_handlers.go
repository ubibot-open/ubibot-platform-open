package api

import (
	"net/http"
	"strconv"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

type notificationDTO struct {
	ID        uint   `json:"id"`
	Type      string `json:"type"`
	Level     string `json:"level"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"created_at"`
}

func toNotificationDTO(n *model.Notification) notificationDTO {
	return notificationDTO{
		ID: n.ID, Type: n.Type, Level: n.Level, Title: n.Title,
		Content: n.Content, Status: n.Status, CreatedAt: n.CreatedAt.Unix(),
	}
}

// ListNotifications handles GET /api/admin/notifications — the header
// bell's backing list, plus the unread count it badges itself with.
func (s *Server) ListNotifications(w http.ResponseWriter, r *http.Request) {
	page, pageSize := paginationParams(r)
	rows, total, err := s.Store.ListNotifications(page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	unread, err := s.Store.CountUnreadNotifications()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]notificationDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toNotificationDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total, "unread": unread})
}

// MarkNotificationRead handles POST /api/admin/notifications/{id}/read.
func (s *Server) MarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.MarkNotificationRead(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

// MarkAllNotificationsRead handles POST /api/admin/notifications/read-all.
func (s *Server) MarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.MarkAllNotificationsRead(); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	writeJSON(w, 200, map[string]any{"message": "ok"})
}
