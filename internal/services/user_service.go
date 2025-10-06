package services

import (
	"context"
	"fmt"
	"import-export-backend/internal/auth"
	"import-export-backend/internal/models"
	"import-export-backend/internal/repository"
	"import-export-backend/pkg"
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

// CreateOrUpdateUser creates a new user or updates existing one from Firebase token
func (s *UserService) CreateOrUpdateUser(ctx context.Context, uid, email, name string) (*models.User, error) {
	// Check if user already exists
	existingUser, err := s.userRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	if existingUser != nil {
		// Update existing user
		existingUser.Email = email
		existingUser.Name = name
		if err := s.userRepo.Update(ctx, existingUser); err != nil {
			return nil, fmt.Errorf("failed to update user: %w", err)
		}
		return existingUser, nil
	}

	// Create new user with default role
	newUser := &models.User{
		UID:    uid,
		Email:  email,
		Name:   name,
		Role:   string(models.RoleStaff), // Default role
		Active: true,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Assign default role in Casbin
	if err := s.casbinService.AddRoleForUser(uid, string(models.RoleStaff)); err != nil {
		return nil, fmt.Errorf("failed to assign default role: %w", err)
	}

	return newUser, nil
}

// GetUserByUID retrieves a user by Firebase UID
func (s *UserService) GetUserByUID(ctx context.Context, uid string) (*models.User, error) {
	user, err := s.userRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return user, nil
}

// UpdateUserRole updates a user's role (admin only)
func (s *UserService) UpdateUserRole(ctx context.Context, uid, newRole string, updatedBy string) error {
	// Validate role
	if !models.UserRole(newRole).IsValidRole() {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid role", nil)
	}

	// Get current user
	user, err := s.userRepo.GetByUID(ctx, uid)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "User not found", nil)
	}

	// Update role in database
	if err := s.userRepo.UpdateRole(ctx, uid, newRole); err != nil {
		return fmt.Errorf("failed to update user role in database: %w", err)
	}

	// Remove old role from Casbin
	oldRoles, err := s.casbinService.GetRolesForUser(uid)
	if err != nil {
		return fmt.Errorf("failed to get user roles from Casbin: %w", err)
	}
	for _, role := range oldRoles {
		if err := s.casbinService.DeleteRoleForUser(uid, role); err != nil {
			return fmt.Errorf("failed to remove old role from Casbin: %w", err)
		}
	}

	// Add new role to Casbin
	if err := s.casbinService.AddRoleForUser(uid, newRole); err != nil {
		return fmt.Errorf("failed to add new role to Casbin: %w", err)
	}

	return nil
}

// ListUsers retrieves all users with pagination (admin only)
func (s *UserService) ListUsers(ctx context.Context, limit, offset int) ([]*models.User, int64, error) {
	users, total, err := s.userRepo.List(ctx, limit, offset)
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

	// Remove roles from Casbin
	roles, err := s.casbinService.GetRolesForUser(user.UID)
	if err != nil {
		return fmt.Errorf("failed to get user roles: %w", err)
	}
	for _, role := range roles {
		if err := s.casbinService.DeleteRoleForUser(user.UID, role); err != nil {
			return fmt.Errorf("failed to remove role from Casbin: %w", err)
		}
	}

	// Delete user from database
	if err := s.userRepo.Delete(ctx, userID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}
