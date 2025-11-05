package helpers

import (
	"context"
	"fmt"

	"cim-backend/internal/auth"

	firebaseAuth "firebase.google.com/go/v4/auth"
)

// MockFirebaseAuthService is a mock implementation of Firebase Auth for testing
type MockFirebaseAuthService struct {
	allowedTokens map[string]*MockToken
}

// Ensure MockFirebaseAuthService implements FirebaseAuthInterface
var _ auth.FirebaseAuthInterface = (*MockFirebaseAuthService)(nil)

// MockToken represents a mock Firebase token
type MockToken struct {
	UID    string
	Claims map[string]interface{}
}

// NewMockFirebaseAuthService creates a new mock Firebase Auth service
func NewMockFirebaseAuthService() *MockFirebaseAuthService {
	return &MockFirebaseAuthService{
		allowedTokens: make(map[string]*MockToken),
	}
}

// VerifyToken verifies a mock token and returns a Firebase-compatible token
func (m *MockFirebaseAuthService) VerifyToken(ctx context.Context, idToken string) (*firebaseAuth.Token, error) {
	// Check if token is registered
	mockToken, exists := m.allowedTokens[idToken]
	if !exists {
		return nil, fmt.Errorf("invalid token")
	}

	// Create a Firebase-compatible token
	token := &firebaseAuth.Token{
		UID:    mockToken.UID,
		Claims: mockToken.Claims,
	}

	return token, nil
}

// RegisterToken registers a token that will be accepted by VerifyToken
func (m *MockFirebaseAuthService) RegisterToken(tokenString string, uid string, email string, name string) {
	m.allowedTokens[tokenString] = &MockToken{
		UID: uid,
		Claims: map[string]interface{}{
			"email": email,
			"name":  name,
		},
	}
}

// GetUser is a stub implementation (not used in tests but required by interface)
func (m *MockFirebaseAuthService) GetUser(ctx context.Context, uid string) (*firebaseAuth.UserRecord, error) {
	return nil, fmt.Errorf("not implemented in mock")
}

// SetCustomClaims is a stub implementation (not used in tests but required by interface)
func (m *MockFirebaseAuthService) SetCustomClaims(ctx context.Context, uid string, claims map[string]interface{}) error {
	// For tests, we can optionally store claims if needed
	return nil
}
