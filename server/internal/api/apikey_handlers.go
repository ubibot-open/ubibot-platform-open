package api

import (
	"net/http"
	"strconv"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

type apiKeyDTO struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Prefix     string `json:"prefix"`
	Revoked    bool   `json:"revoked"`
	LastUsedAt *int64 `json:"last_used_at"`
	CreatedAt  int64  `json:"created_at"`
}

func toApiKeyDTO(k *model.ApiKey) apiKeyDTO {
	dto := apiKeyDTO{ID: k.ID, Name: k.Name, Prefix: k.Prefix, Revoked: k.Revoked, CreatedAt: k.CreatedAt.Unix()}
	if k.LastUsedAt != nil {
		v := k.LastUsedAt.Unix()
		dto.LastUsedAt = &v
	}
	return dto
}

// ListApiKeys handles GET /api/admin/api-keys.
func (s *Server) ListApiKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := s.Store.ListApiKeys()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]apiKeyDTO, 0, len(rows))
	for i := range rows {
		list = append(list, toApiKeyDTO(&rows[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type createApiKeyRequest struct {
	Name string `json:"name"`
}

// CreateApiKey handles POST /api/admin/api-keys — the raw key is only
// ever shown in this response, the same "shown once" pattern as a device
// secret.
func (s *Server) CreateApiKey(w http.ResponseWriter, r *http.Request) {
	var req createApiKeyRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		adminErr(w, 400, "name is required")
		return
	}
	key, raw, err := s.Store.CreateApiKey(req.Name)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "apikey.create", "api_key", key.ID, req.Name)
	dto := toApiKeyDTO(key)
	writeJSON(w, 200, map[string]any{"key": dto, "raw_key": raw})
}

// RevokeApiKey handles POST /api/admin/api-keys/{id}/revoke.
func (s *Server) RevokeApiKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.RevokeApiKey(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "apikey.revoke", "api_key", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}
