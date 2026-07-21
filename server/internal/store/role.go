package store

import (
	"errors"
	"strings"

	"gorm.io/gorm"

	"github.com/ubibot/ubibot-platform-open/internal/model"
)

// HasPermission reports whether role grants code. The built-in super-admin
// role (model.RoleSuper) and a stored "*" both mean "everything" — the
// former so a botched permissions edit can never lock every admin out.
func HasPermission(role *model.Role, code string) bool {
	if role.Code == model.RoleSuper {
		return true
	}
	for _, p := range strings.Fields(role.Permissions) {
		if p == "*" || p == code {
			return true
		}
	}
	return false
}

func (s *Store) CreateRole(name, code string, permissions []string) (*model.Role, error) {
	role := &model.Role{Name: name, Code: code, Permissions: strings.Join(permissions, " ")}
	if err := s.db.Create(role).Error; err != nil {
		return nil, err
	}
	return role, nil
}

func (s *Store) ListRoles() ([]model.Role, error) {
	var rows []model.Role
	err := s.db.Order("id asc").Find(&rows).Error
	return rows, err
}

func (s *Store) RoleByID(id uint) (*model.Role, error) {
	var role model.Role
	if err := s.db.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (s *Store) RoleByCode(code string) (*model.Role, error) {
	var role model.Role
	if err := s.db.Where("code = ?", code).First(&role).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &role, nil
}

func (s *Store) UpdateRole(id uint, name string, permissions []string) error {
	return s.db.Model(&model.Role{}).Where("id = ?", id).Updates(map[string]any{
		"name":        name,
		"permissions": strings.Join(permissions, " "),
	}).Error
}

// DeleteRole refuses to delete a role that's still assigned to an admin
// account — otherwise that account would be left with a dangling RoleID
// and every permission check for it would silently fail closed.
func (s *Store) DeleteRole(id uint) error {
	var count int64
	if err := s.db.Model(&model.AdminUser{}).Where("role_id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("role is still assigned to one or more admin accounts")
	}
	return s.db.Delete(&model.Role{}, id).Error
}
