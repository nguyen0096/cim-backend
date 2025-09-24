package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims
type Claims struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

func main() {
	// JWT secret (should match the one in your config)
	secret := "your-secret-key-here"
	
	// Create claims
	claims := Claims{
		UserID: "test-user-uuid",
		Email:  "test@example.com",
		Role:   "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	
	// Sign token
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		fmt.Printf("Error generating token: %v\n", err)
		return
	}

	fmt.Println("Generated JWT Token:")
	fmt.Println("===================")
	fmt.Println(tokenString)
	fmt.Println()
	fmt.Println("Use this token in your requests:")
	fmt.Println("Authorization: Bearer", tokenString)
	fmt.Println()
	fmt.Println("Token Details:")
	fmt.Printf("- User ID: %s\n", claims.UserID)
	fmt.Printf("- Email: %s\n", claims.Email)
	fmt.Printf("- Role: %s\n", claims.Role)
	fmt.Printf("- Expires: %s\n", claims.ExpiresAt.Time.Format(time.RFC3339))
}
