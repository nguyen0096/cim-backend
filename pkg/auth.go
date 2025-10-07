package pkg

import (
	"context"
	"fmt"
)

const AuthContextKeyUserID = "user_id"
const AuthContextKeyUserEmail = "user_email"
const AuthContextKeyUserName = "user_name"
const AuthContextKeyUserRole = "user_role"
const AuthContextKeyUserPermissions = "user_permissions"

// GetUserEmailFromContext gets the user email from the context
func GetUserEmailFromContext(ctx context.Context) (string, error) {
	userEmailIntf := ctx.Value(AuthContextKeyUserEmail)
	if userEmailIntf == nil {
		return "", fmt.Errorf("user not authenticated")
	}

	userEmail, ok := userEmailIntf.(string)
	if !ok {
		return "", fmt.Errorf("invalid user email format")
	}

	return userEmail, nil
}

// GetUserIDFromContext gets the user ID from the context
func GetUserIDFromContext(ctx context.Context) (string, error) {
	userIDIntf := ctx.Value(AuthContextKeyUserID)
	if userIDIntf == nil {
		return "", fmt.Errorf("user not authenticated")
	}

	userID, ok := userIDIntf.(string)
	if !ok {
		return "", fmt.Errorf("invalid user ID format")
	}

	return userID, nil
}

// WithUserEmail adds user email to the context
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, AuthContextKeyUserEmail, email)
}

// WithUserID adds user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, AuthContextKeyUserID, userID)
}

// UserPermission represents a single permission for a user
type UserPermission struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// HasPermission checks if user has a specific permission (uses context permissions)
func HasPermission(ctx context.Context, resource, action string) bool {
	permissions, ok := ctx.Value(AuthContextKeyUserPermissions).([]UserPermission)
	if !ok {
		return false
	}

	for _, perm := range permissions {
		if perm.Resource == resource && perm.Action == action {
			return true
		}
	}
	return false
}

// GetUserPermissions returns all user permissions from context
func GetUserPermissions(ctx context.Context) []UserPermission {
	permissions, ok := ctx.Value(AuthContextKeyUserPermissions).([]UserPermission)
	if !ok {
		return []UserPermission{}
	}
	return permissions
}
