package services

import (
	"cim-backend/internal/auth"
	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"
	"context"
	"fmt"
)

// UserService handles user business logic
type UserService struct {
	userRepo      *repository.UserRepository
	casbinService *auth.CasbinService
}

// NewUserService creates a new user service
func NewUserService(userRepo *repository.UserRepository, casbinService *auth.CasbinService) *UserService {
	return &UserService{
		userRepo:      userRepo,
		casbinService: casbinService,
	}
}

// GetUserByUID retrieves a user by Firebase UID
func (s *UserService) GetUserByUID(ctx context.Context, uid string) (*models.User, error) {
	user, err := s.userRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

// UpdateUser updates user properties except email (admin only)
func (s *UserService) UpdateUser(ctx context.Context, id, uid, name, role, status string) error {
	// Validate role
	if !models.UserRole(role).IsValidRole() {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid role", nil)
	}

	// Get current user
	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "User not found", nil)
	}

	// Email updates are not allowed - keep existing email

	// Update user properties (email cannot be changed)
	if uid != "" {
		user.UID = uid
	}
	user.Name = name
	user.Role = models.UserRole(role)
	user.Status = status

	// The row is the only thing to write: authorization derives the subject from
	// users.role on each request, so this commit is the whole role change.
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user in database: %w", err)
	}

	return nil
}

// ListUsers retrieves all users with pagination (admin only)
func (s *UserService) ListUsers(ctx context.Context, limit, offset int, excludeUserID string) ([]*models.User, int64, error) {
	users, total, err := s.userRepo.List(ctx, limit, offset, excludeUserID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}
	return users, total, nil
}

// GetUsersByRole retrieves users by role
func (s *UserService) GetUsersByRole(ctx context.Context, role string) ([]*models.User, error) {
	users, err := s.userRepo.GetByRole(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("failed to get users by role: %w", err)
	}
	return users, nil
}

// SearchUsers searches users by name or email with pagination
func (s *UserService) SearchUsers(ctx context.Context, query string, limit, offset int, excludeUserID string) ([]*models.User, int64, error) {
	users, total, err := s.userRepo.Search(ctx, query, limit, offset, excludeUserID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search users: %w", err)
	}
	return users, total, nil
}

// GetUserPermissions retrieves all permissions for a user based on their roles
func (s *UserService) GetUserPermissions(ctx context.Context, userEmail string) ([]string, error) {
	// Verify user exists
	user, err := s.userRepo.GetByEmail(ctx, userEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "User not found", nil)
	}

	if user.Status == "inactive" {
		return nil, pkg.NewAppError(pkg.ErrorCodeForbidden, "User is inactive", nil)
	}

	// The subject the middleware enforced on, not a fresh derivation from this row:
	// the row may carry a UID the middleware failed to repair, which would report a
	// different account's permissions from the one the endpoints answer for.
	subject, err := pkg.GetEnforcementSubjectFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot report permissions for an unauthorized request: %w", err)
	}

	permissions, err := s.casbinService.GetUserPermissions(subject)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	return permissions, nil
}

// CreateUser creates a new user (admin only)
func (s *UserService) CreateUser(ctx context.Context, uid, email, name, role, status string) (*models.User, error) {
	// Validate role
	if !models.UserRole(role).IsValidRole() {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid role", nil)
	}

	// Create new user
	newUser := &models.User{
		Email:  email,
		UID:    uid,
		Name:   name,
		Role:   models.UserRole(role),
		Status: status,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return newUser, nil
}

// DeleteUser soft deletes a user (admin only). The row is the account: once it is gone
// the email lookup in AuthorizationMiddleware fails and the caller is refused, so there
// is no separate grant to revoke.
func (s *UserService) DeleteUser(ctx context.Context, userID string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "User not found", nil)
	}

	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// UpdateUserUID repoints a user row at the Firebase UID its token presents.
func (s *UserService) UpdateUserUID(ctx context.Context, user *models.User, newUID string) error {
	// If UID is already the same, no need to update
	if user.UID == newUID {
		return nil
	}

	user.UID = newUID

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user UID in database: %w", err)
	}

	return nil
}
