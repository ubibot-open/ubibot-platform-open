package api

import (
	"net/http"
	"strconv"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

type dictEntryDTO struct {
	ID   uint   `json:"id"`
	Type string `json:"type"`
	Key  string `json:"key"`
	Label string `json:"label"`
	Sort int    `json:"sort"`
}

func toDictEntryDTO(e *model.DictEntry) dictEntryDTO {
	return dictEntryDTO{ID: e.ID, Type: e.Type, Key: e.Key, Label: e.Label, Sort: e.Sort}
}

// ListDictEntries handles GET /api/admin/dict?type=command_type — type is
// optional, omit it to list every entry across every dictionary.
func (s *Server) ListDictEntries(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListDictEntries(r.URL.Query().Get("type"))
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]dictEntryDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toDictEntryDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type dictEntryRequest struct {
	Type  string `json:"type"`
	Key   string `json:"key"`
	Label string `json:"label"`
	Sort  int    `json:"sort"`
}

// CreateDictEntry handles POST /api/admin/dict.
func (s *Server) CreateDictEntry(w http.ResponseWriter, r *http.Request) {
	var req dictEntryRequest
	if err := decodeJSON(r, &req); err != nil || req.Type == "" || req.Key == "" || req.Label == "" {
		adminErr(w, 400, "type, key and label are required")
		return
	}
	e, err := s.Store.CreateDictEntry(req.Type, req.Key, req.Label, req.Sort)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "dict.create", "dict_entry", e.ID, req.Type+":"+req.Key)
	writeJSON(w, 200, toDictEntryDTO(e))
}

// UpdateDictEntry handles PATCH /api/admin/dict/{id}.
func (s *Server) UpdateDictEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	var req dictEntryRequest
	if err := decodeJSON(r, &req); err != nil || req.Label == "" {
		adminErr(w, 400, "label is required")
		return
	}
	if err := s.Store.UpdateDictEntry(uint(id), req.Label, req.Sort); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "dict.update", "dict_entry", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

// DeleteDictEntry handles DELETE /api/admin/dict/{id}.
func (s *Server) DeleteDictEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.DeleteDictEntry(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "dict.delete", "dict_entry", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}
