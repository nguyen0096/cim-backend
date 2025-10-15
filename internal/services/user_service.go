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

// CreateOrUpdateUser creates a new user or updates existing one from Firebase token
func (s *UserService) CreateOrUpdateUser(ctx context.Context, uid, email, name, status string) (*models.User, error) {
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
		Status: status,
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

// UpdateUser updates user properties except email (admin only)
func (s *UserService) UpdateUser(ctx context.Context, userID, name, role, status string, updatedBy string) error {
	// Validate role
	if !models.UserRole(role).IsValidRole() {
		return pkg.NewAppError(pkg.ErrorCodeValidation, "Invalid role", nil)
	}

	// Get current user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return pkg.NewAppError(pkg.ErrorCodeNotFound, "User not found", nil)
	}

	// Email updates are not allowed - keep existing email

	// Check if role is being changed for Casbin update
	roleChanged := user.Role != role

	// Update user properties (email cannot be changed)
	user.Name = name
	user.Role = role
	user.Status = status

	// Update user in database
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user in database: %w", err)
	}

	// Update role in Casbin if role changed
	if roleChanged {
		// Remove old role from Casbin
		oldRoles, err := s.casbinService.GetRolesForUser(user.UID)
		if err != nil {
			return fmt.Errorf("failed to get user roles from Casbin: %w", err)
		}
		for _, oldRole := range oldRoles {
			if err := s.casbinService.DeleteRoleForUser(user.UID, oldRole); err != nil {
				return fmt.Errorf("failed to remove old role from Casbin: %w", err)
			}
		}

		// Add new role to Casbin
		if err := s.casbinService.AddRoleForUser(user.UID, role); err != nil {
			return fmt.Errorf("failed to add new role to Casbin: %w", err)
		}
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

// GetUserPermissions retrieves all permissions for a user based on their roles
func (s *UserService) GetUserPermissions(ctx context.Context, userUID string) ([]string, error) {
	// Verify user exists
	user, err := s.userRepo.GetByUID(ctx, userUID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeNotFound, "User not found", nil)
	}

	// Get permissions from Casbin
	permissions, err := s.casbinService.GetUserPermissions(userUID)
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

	// Check if user already exists by UID
	existingUserByUID, err := s.userRepo.GetByUID(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user by UID: %w", err)
	}
	if existingUserByUID != nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "User with this UID already exists", nil)
	}

	// Check if user already exists by email
	existingUserByEmail, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing user by email: %w", err)
	}
	if existingUserByEmail != nil {
		return nil, pkg.NewAppError(pkg.ErrorCodeValidation, "User with this email already exists", nil)
	}

	// Create new user
	newUser := &models.User{
		Email:  email,
		UID:    uid,
		Name:   name,
		Role:   role,
		Status: status,
	}

	if err := s.userRepo.Create(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Assign role in Casbin
	if err := s.casbinService.AddRoleForUser(uid, role); err != nil {
		// If Casbin assignment fails, we should clean up the created user
		if deleteErr := s.userRepo.Delete(ctx, newUser.ID.String()); deleteErr != nil {
			return nil, fmt.Errorf("failed to create user and clean up after role assignment failure: %w", err)
		}
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
