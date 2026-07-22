package api

import (
	"net/http"
	"strings"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

type iconDTO struct {
	Key       string `json:"key"`
	Name      string `json:"name"`
	SVG       string `json:"svg"`
	CreatedAt int64  `json:"created_at"`
}

func toIconDTO(i *model.IconAsset) iconDTO {
	return iconDTO{Key: i.Key, Name: i.Name, SVG: i.SVG, CreatedAt: i.CreatedAt.Unix()}
}

// ListIcons handles GET /api/admin/icons -- gated on device:read (not
// system:manage) since this only affects how the 数据仓库 page renders
// sensor values, the same read scope as the data it's decorating, not a
// system-configuration change.
func (s *Server) ListIcons(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListIcons()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]iconDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toIconDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type iconUploadRequest struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	SVG  string `json:"svg"`
}

// maxIconSVGBytes is generous for a hand-authored icon and just guards
// against pasting something that clearly isn't one.
const maxIconSVGBytes = 64 * 1024

// UploadIcon handles POST /api/admin/icons. The SVG travels as raw markup
// in the JSON body rather than a multipart file upload -- the frontend
// reads the chosen .svg file as text with FileReader and sends its
// contents directly, which is simpler than a multipart round trip for
// something this small. Uploading again for a key already in the library
// replaces its icon (see store.UpsertIcon).
func (s *Server) UploadIcon(w http.ResponseWriter, r *http.Request) {
	var req iconUploadRequest
	if err := decodeJSON(r, &req); err != nil || req.Key == "" || req.Name == "" || req.SVG == "" {
		adminErr(w, 400, "key, name and svg are required")
		return
	}
	if len(req.SVG) > maxIconSVGBytes {
		adminErr(w, 400, "svg too large")
		return
	}
	if !strings.Contains(req.SVG, "<svg") {
		adminErr(w, 400, "not a valid svg")
		return
	}

	icon, err := s.Store.UpsertIcon(req.Key, req.Name, req.SVG)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "icon.upload", "icon_asset", icon.ID, req.Key)
	writeJSON(w, 200, toIconDTO(icon))
}

// DeleteIcon handles DELETE /api/admin/icons/{key} -- reverts that field
// back to the frontend's built-in default icon.
func (s *Server) DeleteIcon(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		adminErr(w, 400, "invalid key")
		return
	}
	if err := s.Store.DeleteIcon(key); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "icon.delete", "icon_asset", 0, key)
	writeJSON(w, 200, map[string]any{"message": "ok"})
}
