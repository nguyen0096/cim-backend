package auth

import (
	"context"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"google.golang.org/api/option"
)

// FirebaseAuthService handles Firebase authentication
type FirebaseAuthService struct {
	client *auth.Client
}

// NewFirebaseAuthService creates a new Firebase Auth service
func NewFirebaseAuthService(serviceAccountPath string) (*FirebaseAuthService, error) {
	opt := option.WithCredentialsFile(serviceAccountPath)
	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		return nil, fmt.Errorf("error initializing Firebase app: %v", err)
	}

	client, err := app.Auth(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error getting Firebase Auth client: %v", err)
	}

	return &FirebaseAuthService{
		client: client,
	}, nil
}

// VerifyToken verifies a Firebase ID token and returns user information
func (f *FirebaseAuthService) VerifyToken(ctx context.Context, idToken string) (*auth.Token, error) {
	token, err := f.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("error verifying ID token: %v", err)
	}
	return token, nil
}

// GetUser retrieves user information by UID
func (f *FirebaseAuthService) GetUser(ctx context.Context, uid string) (*auth.UserRecord, error) {
	user, err := f.client.GetUser(ctx, uid)
	if err != nil {
		return nil, fmt.Errorf("error getting user: %v", err)
	}
	return user, nil
}

// SetCustomClaims sets custom claims for a user
func (f *FirebaseAuthService) SetCustomClaims(ctx context.Context, uid string, claims map[string]interface{}) error {
	err := f.client.SetCustomUserClaims(ctx, uid, claims)
	if err != nil {
		return fmt.Errorf("error setting custom claims: %v", err)
	}
	return nil
}
  