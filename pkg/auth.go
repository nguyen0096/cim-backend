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
	// AuthContextKeyEnforcementSubject carries the Casbin subject the request was
	// actually enforced on. Anything reporting what the caller may do must read this
	// rather than re-deriving, which can differ from what was enforced.
	AuthContextKeyEnforcementSubject
)

const (
	// RBAC resources
	RBACResourceInventorySubmissions = "inventory-submissions"

	// RBAC actions
	RBACActionApprove = "approve"
	// RBACActionInitiateReconciliation gates the reconcile-initiate endpoint;
	// admin/accountant only.
	RBACActionInitiateReconciliation = "initiate_reconciliation"

	// Staff reconciliation child-item actions; granted to staff (own rows, while
	// the submission is open) and admin/accountant.
	RBACActionReconItemCreate = "recon_item_create"
	RBACActionReconItemUpdate = "recon_item_update"
	RBACActionReconItemDelete = "recon_item_delete"
	// RBACActionReconItemView gates the list-rows endpoint; staff see their own
	// rows, admin/accountant see all.
	RBACActionReconItemView = "recon_item_view"

	// RBACActionReconManage gates the admin/accountant reconciliation management
	// endpoints (close/reopen/start-processing) and flags a caller allowed to edit
	// a closed submission's rows. Admin/accountant only.
	RBACActionReconManage = "recon_manage"
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

// GetEnforcementSubjectFromContext gets the Casbin subject the request was enforced on
func GetEnforcementSubjectFromContext(ctx context.Context) (string, error) {
	subject, ok := ctx.Value(AuthContextKeyEnforcementSubject).(string)
	if !ok {
		return "", fmt.Errorf("enforcement subject not found in context")
	}
	return subject, nil
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

	userEmail, err := GetUserEmailFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user email from context: %w", err)
	}

	updates["updated_at"] = time.Now()
	updates["updated_by"] = userEmail

	return updates, nil
}
