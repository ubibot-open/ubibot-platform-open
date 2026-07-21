// Mirrors the handful of backend constants (server/internal/model/model.go)
// the frontend needs to know about by value — kept in one place so they're
// easy to spot if the backend ever renames them.
export const model = {
  // The built-in super-admin role code. Always passes every permission
  // check server-side (see store.HasPermission) and can't be deleted —
  // the role picker/role-management UI treats it as protected too.
  RoleSuper: 'super_admin',
}
