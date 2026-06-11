package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"tanabata/backend/internal/domain"
	"tanabata/backend/internal/port"
)

// UserService handles user CRUD and profile management.
type UserService struct {
	users    port.UserRepo
	sessions port.SessionRepo
	audit    *AuditService
}

// NewUserService creates a UserService.
func NewUserService(users port.UserRepo, sessions port.SessionRepo, audit *AuditService) *UserService {
	return &UserService{users: users, sessions: sessions, audit: audit}
}

// EnsureAdmin creates the initial administrator account if it does not already
// exist. It is idempotent and never overwrites an existing user's password, so
// an operator who has changed the admin password keeps it across restarts.
func (s *UserService) EnsureAdmin(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("EnsureAdmin: username and password must be set")
	}

	if _, err := s.users.GetByName(ctx, username); err == nil {
		return nil // already exists
	} else if !errors.Is(err, domain.ErrNotFound) {
		return fmt.Errorf("EnsureAdmin: lookup: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("EnsureAdmin: hash: %w", err)
	}
	_, err = s.users.Create(ctx, &domain.User{
		Name:      username,
		Password:  string(hash),
		IsAdmin:   true,
		CanCreate: true,
	})
	if err != nil {
		return fmt.Errorf("EnsureAdmin: create: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Self-service
// ---------------------------------------------------------------------------

// GetMe returns the profile of the currently authenticated user.
func (s *UserService) GetMe(ctx context.Context) (*domain.User, error) {
	userID, _, _ := domain.UserFromContext(ctx)
	return s.users.GetByID(ctx, userID)
}

// UpdateMeParams holds fields a user may change on their own profile.
type UpdateMeParams struct {
	Name     string  // empty = no change
	Password *string // nil = no change
}

// UpdateMe allows a user to change their own name and/or password.
func (s *UserService) UpdateMe(ctx context.Context, p UpdateMeParams) (*domain.User, error) {
	userID, _, _ := domain.UserFromContext(ctx)

	current, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	patch := *current
	if p.Name != "" {
		patch.Name = p.Name
	}
	if p.Password != nil {
		hash, err := bcrypt.GenerateFromPassword([]byte(*p.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("UserService.UpdateMe hash: %w", err)
		}
		patch.Password = string(hash)
	}

	return s.users.Update(ctx, userID, &patch)
}

// ---------------------------------------------------------------------------
// Admin CRUD
// ---------------------------------------------------------------------------

// List returns a paginated list of users (admin only — caller must enforce).
func (s *UserService) List(ctx context.Context, params port.OffsetParams) (*domain.UserPage, error) {
	return s.users.List(ctx, params)
}

// Get returns a user by ID (admin only).
func (s *UserService) Get(ctx context.Context, id int16) (*domain.User, error) {
	return s.users.GetByID(ctx, id)
}

// CreateParams holds fields for creating a new user.
type CreateUserParams struct {
	Name      string
	Password  string
	IsAdmin   bool
	CanCreate bool
}

// Create inserts a new user with a bcrypt-hashed password (admin only).
func (s *UserService) Create(ctx context.Context, p CreateUserParams) (*domain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("UserService.Create hash: %w", err)
	}

	u := &domain.User{
		Name:      p.Name,
		Password:  string(hash),
		IsAdmin:   p.IsAdmin,
		CanCreate: p.CanCreate,
	}
	created, err := s.users.Create(ctx, u)
	if err != nil {
		return nil, err
	}

	_ = s.audit.Log(ctx, "user_create", nil, nil, map[string]any{"target_user_id": created.ID})
	return created, nil
}

// UpdateAdminParams holds fields an admin may change on any user.
type UpdateAdminParams struct {
	IsAdmin   *bool
	CanCreate *bool
	IsBlocked *bool
}

// UpdateAdmin applies an admin-level patch to a user.
func (s *UserService) UpdateAdmin(ctx context.Context, id int16, p UpdateAdminParams) (*domain.User, error) {
	current, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	patch := *current
	if p.IsAdmin != nil {
		patch.IsAdmin = *p.IsAdmin
	}
	if p.CanCreate != nil {
		patch.CanCreate = *p.CanCreate
	}
	if p.IsBlocked != nil {
		patch.IsBlocked = *p.IsBlocked
	}

	updated, err := s.users.Update(ctx, id, &patch)
	if err != nil {
		return nil, err
	}

	// Log block/unblock specifically, and revoke all sessions on block so the
	// user's outstanding access tokens stop working immediately.
	if p.IsBlocked != nil {
		action := "user_unblock"
		if *p.IsBlocked {
			action = "user_block"
			if err := s.sessions.DeleteByUserID(ctx, id); err != nil {
				return nil, fmt.Errorf("UserService.UpdateAdmin revoke sessions: %w", err)
			}
		}
		_ = s.audit.Log(ctx, action, nil, nil, map[string]any{"target_user_id": id})
	}
	return updated, nil
}

// Delete removes a user by ID (admin only).
func (s *UserService) Delete(ctx context.Context, id int16) error {
	if err := s.users.Delete(ctx, id); err != nil {
		return err
	}
	_ = s.audit.Log(ctx, "user_delete", nil, nil, map[string]any{"target_user_id": id})
	return nil
}
