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

// GetUserByEmail retrieves a user by email
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}

// UpdateUser updates user properties except email (admin only)
func (s *UserService) UpdateUser(ctx context.Context, id, name, role, status string, forceCasbinUpdate bool) error {
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

	// Check if role is being changed for Casbin update
	roleChanged := user.Role != models.UserRole(role)

	// Update user properties (email cannot be changed)
	user.Name = name
	user.Role = models.UserRole(role)
	user.Status = status

	// Update user in database
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user in database: %w", err)
	}

	// Update role in Casbin if role changed (using email as identifier)
	if roleChanged || forceCasbinUpdate {
		// Remove old role from Casbin
		oldRoles, err := s.casbinService.GetRolesForUser(user.Email)
		if err != nil {
			return fmt.Errorf("failed to get user roles from Casbin: %w", err)
		}
		for _, oldRole := range oldRoles {
			if err := s.casbinService.DeleteRoleForUser(user.Email, oldRole); err != nil {
				return fmt.Errorf("failed to remove old role from Casbin: %w", err)
			}
		}

		// Add new role to Casbin
		if err := s.casbinService.AddRoleForUser(user.Email, role); err != nil {
			return fmt.Errorf("failed to add new role to Casbin: %w", err)
		}
	}

	return nil
}

// ListUsers retrieves all users with pagination (admin only)
func (s *UserService) ListUsers(ctx context.Context, limit, offset int, excludeUserEmail string) ([]*models.User, int64, error) {
	users, total, err := s.userRepo.List(ctx, limit, offset, excludeUserEmail)
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
func (s *UserService) SearchUsers(ctx context.Context, query string, limit, offset int, excludeUserEmail string) ([]*models.User, int64, error) {
	users, total, err := s.userRepo.Search(ctx, query, limit, offset, excludeUserEmail)
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

	// Get permissions from Casbin (using email as identifier)
	permissions, err := s.casbinService.GetUserPermissions(user.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}

	return permissions, nil
}

// CreateUser creates a new user (admin only)
func (s *UserService) CreateUser(ctx context.Context, email, name, role, status string) (*models.User, error) {
	// Validate role
	if !models.UserRole(role).IsValidRole() {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid role", nil)
	}

	// Create new user
	newUser := &models.User{
		Email:  email,
		Name:   name,
		Role:   models.UserRole(role),
		Status: status,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Assign role in Casbin (using email as identifier)
	if err := s.casbinService.AddRoleForUser(email, role); err != nil {
		return nil, fmt.Errorf("failed to assign role: %w", err)
	}

	return newUser, nil
}

// DeleteUser soft deletes a user (admin only)
func (s *UserService) DeleteUser(ctx context.Context, userID string) error {
	// Remove all roles from Casbin first
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "User not found", nil)
	}

	// Remove roles from Casbin (using email as identifier)
	roles, err := s.casbinService.GetRolesForUser(user.Email)
	if err != nil {
		return fmt.Errorf("failed to get user roles: %w", err)
	}
	for _, role := range roles {
		if err := s.casbinService.DeleteRoleForUser(user.Email, role); err != nil {
			return fmt.Errorf("failed to remove role from Casbin: %w", err)
		}
	}

	// Delete user from database
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
