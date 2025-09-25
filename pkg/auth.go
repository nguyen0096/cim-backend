package pkg

import (
	"context"
	"fmt"
)

const AuthContextKeyUserID = "user_id"
const AuthContextKeyUserEmail = "user_email"

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

// WithUserEmail adds user email to the context
func WithUserEmail(ctx context.Context, email string) context.Context {
	return context.WithValue(ctx, AuthContextKeyUserEmail, email)
}
