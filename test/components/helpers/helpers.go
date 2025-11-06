package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"cim-backend/internal/models"
	"cim-backend/internal/repository"
	"cim-backend/pkg"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CreateTestUser creates a test user in the database with the specified role
func CreateTestUser(ctx context.Context, db *gorm.DB, email, name string, role models.UserRole) (*models.User, error) {
	userRepo := repository.NewUserRepository(db, "test")

	uid := uuid.New().String()
	user := &models.User{
		UID:    uid,
		Email:  email,
		Name:   name,
		Role:   role,
		Status: "active",
		Type:   models.UserTypeUser,
	}

	if err := userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create test user: %w", err)
	}

	return user, nil
}

func CreateTestFile(t *testing.T, filename string) (*os.File, error) {
	t.Helper()

	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open test file: %w", err)
	}

	return file, nil
}

// GetAuthToken generates a test auth token for API calls
func GetAuthToken(mockAuth *MockFirebaseAuthService, uid, email, name string) string {
	token := fmt.Sprintf("test-token-%s", uuid.New().String())
	mockAuth.RegisterToken(token, uid, email, name)
	return token
}

// MakeRequest makes an HTTP request to the test server with auth headers
func MakeRequest(t *testing.T, method, url, token string, body interface{}) (*http.Response, error) {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

// MakeMultipartRequest makes an HTTP request with multipart/form-data for file uploads
func MakeMultipartRequest(t *testing.T, method, url, token string, filePath, fieldName string) (*http.Response, error) {
	t.Helper()

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info for filename
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Create multipart writer
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Add file field
	part, err := writer.CreateFormFile(fieldName, fileInfo.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy file content to form
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file content: %w", err)
	}

	// Close the multipart writer
	err = writer.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create request
	req, err := http.NewRequest(method, url, &requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	if token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

// ParseResponse parses JSON response body into a map
func ParseResponse(t *testing.T, resp *http.Response) (map[string]interface{}, error) {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// ParseResponseArray parses JSON response body into an array
func ParseResponseArray(t *testing.T, resp *http.Response) ([]interface{}, error) {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}
	defer resp.Body.Close()

	var result []interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// CleanupTestData cleans up test data from the database
func CleanupTestData(ctx context.Context, db *gorm.DB, tableName string, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	// Use raw SQL for cleanup to avoid model-specific issues
	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (?)", tableName)
	if err := db.WithContext(ctx).Exec(query, ids).Error; err != nil {
		return fmt.Errorf("failed to cleanup test data: %w", err)
	}

	return nil
}

// CreateTestContext creates a context with user email for testing
func CreateTestContext(email string) context.Context {
	return pkg.WithUserEmail(context.Background(), email)
}

func ToString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		// JSON numbers are decoded as float64
		return strconv.FormatFloat(val, 'f', -1, 64)
	case int:
		return strconv.Itoa(val)
	case uint:
		return strconv.FormatUint(uint64(val), 10)
	default:
		return ""
	}
}
