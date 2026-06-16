package handler

import "testing"

// TestNewRouterRegisters builds the router with typed-nil dependencies to assert
// route registration itself succeeds. Gin panics on a route conflict (e.g. a
// duplicated method+path or an inconsistent wildcard name) during registration,
// before any handler runs — so this catches such mistakes without a database.
// Handlers are never invoked here; method values on nil pointers are fine.
func TestNewRouterRegisters(t *testing.T) {
	r, err := NewRouter(
		(*AuthMiddleware)(nil), (*AuthHandler)(nil),
		(*FileHandler)(nil), (*DuplicateHandler)(nil), (*TagHandler)(nil), (*CategoryHandler)(nil), (*PoolHandler)(nil),
		(*UserHandler)(nil), (*ACLHandler)(nil), (*AuditHandler)(nil),
		"", nil,
	)
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if r == nil {
		t.Fatal("NewRouter returned nil engine")
	}
}
