// Package rbac holds role constants shared across layers (model default, JWT
// claim, middleware role checks).
package rbac

const (
	// RoleAdmin can register and soft-delete users across all organizations
	// (super-admin / global scope).
	RoleAdmin = "admin"
	// RoleUser is the default role for every registered user.
	RoleUser = "user"
)

// IsValidRole reports whether r is a known role.
func IsValidRole(r string) bool {
	return r == RoleAdmin || r == RoleUser
}
