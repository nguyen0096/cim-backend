package pkg

import (
	"context"
	"fmt"
	"time"
)

//go:generate stringer -type=AuthContextKey
type AuthContextKey int

const (
	AuthContextKeyUserID AuthContextKey = iota
	AuthContextKeyUserEmail
	AuthContextKeyUserName
	AuthContextKeyUserRole
	AuthContextKeyUserPermissions
)

const (
	// RBAC resources
	RBACResourceInventorySubmissions = "inventory-submissions"

	// RBAC actions
	RBACActionApprove = "approve"
	// RBACActionInitiateReconciliation gates the reconcile-initiate endpoint
	// (epic #38, Part 2): only admin/accountant may start a reconciliation and
	// capture the per-item snapshot baseline.
	RBACActionInitiateReconciliation = "initiate_reconciliation"
)

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
	permissions, ok := ctx.Value(AuthContextKeyUserPermissions).(map[UserPermission]struct{})
	if !ok {
		return false
	}

	expectedPermission := UserPermission{
		Resource: resource,
		Action:   action,
	}
	_, exists := permissions[expectedPermission]
	return exists
}

// WithUpdateFields wraps a map update to include UpdatedAt and UpdatedBy fields
// This replicates the behavior of the BeforeUpdate hook when using Updates(map[string]interface{})
func WithUpdateFields(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error) {
	if updates == nil {
		updates = make(map[string]interface{})
	}

	// Get user email from context
	userEmail, err := GetUserEmailFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user email from context: %w", err)
	}

	// Add update fields
	updates["updated_at"] = time.Now()
	updates["updated_by"] = userEmail

	return updates, nil
}
