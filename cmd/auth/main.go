package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	"github.com/joho/godotenv"
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

var (
	apiKey          string
	defaultUsername string
	defaultPassword string
)

var rootCmd = &cobra.Command{
	Use:   "auth [email] [password]",
	Short: "Get Firebase authentication token",
	Long:  "Get Firebase authentication token using email and password credentials. If no arguments provided, uses default credentials from .env file.",
	Args:  cobra.RangeArgs(0, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate Firebase environment variables
		if apiKey == "" {
			return fmt.Errorf("FIREBASE_WEB_API_KEY environment variable is not set")
		}

		var email, password string

		if len(args) == 0 {
			// Use default credentials
			if defaultUsername == "" {
				return fmt.Errorf("FIREBASE_TEST_USER environment variable is not set")
			}
			if defaultPassword == "" {
				return fmt.Errorf("FIREBASE_TEST_PASSWORD environment variable is not set")
			}
			email = defaultUsername
			password = defaultPassword
			fmt.Printf("Using default credentials: %s\n", email)
		} else if len(args) == 2 {
			// Use provided credentials
			email = args[0]
			password = args[1]
		} else {
			return fmt.Errorf("please provide either no arguments (for default credentials) or both email and password")
		}

		token, err := getFirebaseToken(email, password)
		if err != nil {
			return fmt.Errorf("failed to get Firebase token: %w", err)
		}

		fmt.Println(token)

		// Copy to clipboard using pbcopy (macOS/Linux)
		if err := copyToClipboard(token); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not copy to clipboard: %v\n", err)
		} else {
			fmt.Println("✓ Token copied to clipboard")
		}

		return nil
	},
}

func init() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not load .env file: %v\n", err)
	}

	// Load Firebase environment variables
	apiKey = os.Getenv("FIREBASE_WEB_API_KEY")
	defaultUsername = os.Getenv("FIREBASE_TEST_USER")
	defaultPassword = os.Getenv("FIREBASE_TEST_PASSWORD")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
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
		return "", fmt.Errorf("authentication failed (status %d): %s", resp.StatusCode, string(data))
	}

	var res SignInResponse
	if err := json.Unmarshal(data, &res); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return res.IDToken, nil
}

// copyToClipboard copies text to the clipboard using pbcopy (macOS) or xclip (Linux)
func copyToClipboard(text string) error {
	// Try pbcopy (macOS)
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewBufferString(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try xclip (Linux)
	cmd = exec.Command("xclip", "-selection", "clipboard")
	cmd.Stdin = bytes.NewBufferString(text)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Try xsel (Linux alternative)
	cmd = exec.Command("xsel", "--clipboard", "--input")
	cmd.Stdin = bytes.NewBufferString(text)
	return cmd.Run()
}
