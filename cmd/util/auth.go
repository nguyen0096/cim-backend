package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

type SignInRequest struct {
	Email             string `json:"email"`
	Password          string `json:"password"`
	ReturnSecureToken bool   `json:"returnSecureToken"`
}

type SignInResponse struct {
	IDToken      string `json:"idToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    string `json:"expiresIn"`
	LocalID      string `json:"localId"`
}

var authCmd = &cobra.Command{
	Use:   "auth [email] [password]",
	Short: "Get Firebase authentication token",
	Long:  "Get Firebase authentication token using email and password credentials. If no arguments provided, uses default credentials.",
	Args:  cobra.RangeArgs(0, 2),
	Run: func(cmd *cobra.Command, args []string) {
		// Validate Firebase environment variables
		if apiKey == "" {
			fmt.Fprintf(os.Stderr, "Error: FIREBASE_WEB_API_KEY environment variable is not set\n")
			os.Exit(1)
		}

		var email, password string

		if len(args) == 0 {
			// Use default credentials
			if defaultUsername == "" {
				fmt.Fprintf(os.Stderr, "Error: FIREBASE_TEST_USER environment variable is not set\n")
				os.Exit(1)
			}
			if defaultPassword == "" {
				fmt.Fprintf(os.Stderr, "Error: FIREBASE_TEST_PASSWORD environment variable is not set\n")
				os.Exit(1)
			}
			email = defaultUsername
			password = defaultPassword
			fmt.Printf("Using default credentials: %s\n", email)
		} else if len(args) == 2 {
			// Use provided credentials
			email = args[0]
			password = args[1]
		} else {
			fmt.Fprintf(os.Stderr, "Error: Please provide either no arguments (for default credentials) or both email and password\n")
			os.Exit(1)
		}

		token, err := getFirebaseToken(email, password)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting Firebase token: %v\n", err)
			os.Exit(1)
		}

		bearerToken := "Bearer " + token
		fmt.Println(bearerToken)

		// Copy to clipboard using pbcopy
		if err := copyToClipboard(bearerToken); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not copy to clipboard: %v\n", err)
		} else {
			fmt.Println("✓ Token copied to clipboard")
		}
	},
}

// getFirebaseToken authenticates with Firebase and returns the ID token
func getFirebaseToken(email, password string) (string, error) {
	reqBody := SignInRequest{
		Email:             email,
		Password:          password,
		ReturnSecureToken: true,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, err := http.Post(
		"https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key="+apiKey,
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return "", fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("authentication failed: %s", string(data))
	}

	var res SignInResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return res.IDToken, nil
}

// copyToClipboard copies text to the clipboard using pbcopy
func copyToClipboard(text string) error {
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}
