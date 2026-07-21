package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ubibot/ubibot-platform-open/internal/auth"
	"github.com/ubibot/ubibot-platform-open/internal/model"
)

type roleDTO struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Permissions []string `json:"permissions"`
}

func toRoleDTO(r *model.Role) roleDTO {
	var perms []string
	if r.Permissions != "" {
		perms = strings.Fields(r.Permissions)
	}
	return roleDTO{ID: r.ID, Name: r.Name, Code: r.Code, Permissions: perms}
}

// ListRoles handles GET /api/admin/roles.
func (s *Server) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := s.Store.ListRoles()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]roleDTO, 0, len(roles))
	for i := range roles {
		list = append(list, toRoleDTO(&roles[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type roleRequest struct {
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	Permissions []string `json:"permissions"`
}

// CreateRole handles POST /api/admin/roles.
func (s *Server) CreateRole(w http.ResponseWriter, r *http.Request) {
	var req roleRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" || req.Code == "" {
		adminErr(w, 400, "name and code are required")
		return
	}
	role, err := s.Store.CreateRole(req.Name, req.Code, req.Permissions)
	if err != nil {
		adminErr(w, 400, "role already exists or invalid input")
		return
	}
	s.audit(r, "role.create", "role", role.ID, req.Code)
	writeJSON(w, 200, toRoleDTO(role))
}

// UpdateRole handles PATCH /api/admin/roles/{id}. Code is immutable —
// only Name/Permissions are editable — since RoleSuper's special
// treatment in store.HasPermission is keyed off Code, not the row id.
func (s *Server) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	var req roleRequest
	if err := decodeJSON(r, &req); err != nil || req.Name == "" {
		adminErr(w, 400, "name is required")
		return
	}
	if err := s.Store.UpdateRole(uint(id), req.Name, req.Permissions); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "role.update", "role", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

// DeleteRole handles DELETE /api/admin/roles/{id}.
func (s *Server) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if err := s.Store.DeleteRole(uint(id)); err != nil {
		adminErr(w, 400, err.Error())
		return
	}
	s.audit(r, "role.delete", "role", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

type adminUserDTO struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	RoleID    uint   `json:"role_id"`
	RoleName  string `json:"role_name,omitempty"`
	CreatedAt int64  `json:"created_at"`
}

func (s *Server) toAdminUserDTO(a *model.AdminUser) adminUserDTO {
	dto := adminUserDTO{ID: a.ID, Username: a.Username, RoleID: a.RoleID, CreatedAt: a.CreatedAt.Unix()}
	if role, err := s.Store.RoleByID(a.RoleID); err == nil {
		dto.RoleName = role.Name
	}
	return dto
}

// ListAdminUsers handles GET /api/admin/users — the "系统管理/管理员" page.
func (s *Server) ListAdminUsers(w http.ResponseWriter, r *http.Request) {
	admins, err := s.Store.ListAdminUsers()
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]adminUserDTO, 0, len(admins))
	for i := range admins {
		list = append(list, s.toAdminUserDTO(&admins[i]))
	}
	writeJSON(w, 200, map[string]any{"list": list})
}

type createAdminUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	RoleID   uint   `json:"role_id"`
}

// CreateAdminUser handles POST /api/admin/users.
func (s *Server) CreateAdminUser(w http.ResponseWriter, r *http.Request) {
	var req createAdminUserRequest
	if err := decodeJSON(r, &req); err != nil || req.Username == "" || req.Password == "" || req.RoleID == 0 {
		adminErr(w, 400, "username, password and role_id are required")
		return
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	admin, err := s.Store.CreateAdmin(req.Username, hash, req.RoleID)
	if err != nil {
		adminErr(w, 400, "username already exists or invalid input")
		return
	}
	s.audit(r, "admin.create", "admin_user", admin.ID, req.Username)
	writeJSON(w, 200, s.toAdminUserDTO(admin))
}

type updateAdminUserRequest struct {
	RoleID   uint   `json:"role_id"`
	Password string `json:"password"`
}

// UpdateAdminUser handles PATCH /api/admin/users/{id} — reassigns role
// and/or resets the password; both are optional so callers can do either
// independently.
func (s *Server) UpdateAdminUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	var req updateAdminUserRequest
	if err := decodeJSON(r, &req); err != nil {
		adminErr(w, 400, "invalid request body")
		return
	}
	if req.RoleID != 0 {
		if err := s.Store.UpdateAdminRole(uint(id), req.RoleID); err != nil {
			adminErr(w, 500, "internal error")
			return
		}
	}
	if req.Password != "" {
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			adminErr(w, 500, "internal error")
			return
		}
		if err := s.Store.UpdateAdminPassword(uint(id), hash); err != nil {
			adminErr(w, 500, "internal error")
			return
		}
	}
	s.audit(r, "admin.update", "admin_user", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

// DeleteAdminUser handles DELETE /api/admin/users/{id}. An admin cannot
// delete their own account — that would leave the caller instantly
// unauthenticated with no obvious recovery step short of another admin
// stepping in.
func (s *Server) DeleteAdminUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		adminErr(w, 400, "invalid id")
		return
	}
	if current := currentAdmin(r); current != nil && current.ID == uint(id) {
		adminErr(w, 400, "cannot delete your own account")
		return
	}
	if err := s.Store.DeleteAdmin(uint(id)); err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	s.audit(r, "admin.delete", "admin_user", uint(id), "")
	writeJSON(w, 200, map[string]any{"message": "ok"})
}

type auditLogDTO struct {
	ID         uint   `json:"id"`
	Username   string `json:"username"`
	Action     string `json:"action"`
	TargetType string `json:"target_type"`
	TargetID   uint   `json:"target_id"`
	Detail     string `json:"detail"`
	IP         string `json:"ip"`
	CreatedAt  int64  `json:"created_at"`
}

// ListAuditLogs handles GET /api/admin/audit-logs — the "操作日志" page.
func (s *Server) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := paginationParams(r)
	logs, total, err := s.Store.ListAuditLogs(page, pageSize)
	if err != nil {
		adminErr(w, 500, "internal error")
		return
	}
	list := make([]auditLogDTO, 0, len(logs))
	for _, l := range logs {
		list = append(list, auditLogDTO{
			ID: l.ID, Username: l.Username, Action: l.Action, TargetType: l.TargetType,
			TargetID: l.TargetID, Detail: l.Detail, IP: l.IP, CreatedAt: l.CreatedAt.Unix(),
		})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total})
}
