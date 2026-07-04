package testutil

import (
	"bytes"
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	firebaseAuth "firebase.google.com/go/v4/auth"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"
)

// Client is a test client for the API.
type Client struct {
	AuthToken *string
	BaseURL   string
}

// NewClient creates a new test client for the API.
// A fake auth token is generated and registered with the mock auth service.
func NewClient(
	tenv *TestBox,
	userRole models.UserRole,
) *Client {
	client := &Client{
		BaseURL: tenv.BaseURL,
	}

	if userRole != "" {
		uid := uuid.New().String()
		email := fmt.Sprintf("test-%s-%s@cim.local", userRole, uid)
		user := &models.User{
			UID:    uid,
			Email:  email,
			Name:   email,
			Role:   userRole,
			Status: "active",
			Type:   models.UserTypeUser,
		}

		err := tenv.DB.Create(&user).Error
		Expect(err).NotTo(HaveOccurred())

		// Register token with mock auth
		client.AuthToken = pkg.Ptr(fmt.Sprintf("token-test-%s-%s", userRole, uid))
		tenv.AuthMock.On("VerifyToken", mock.Anything, *client.AuthToken).Return(&firebaseAuth.Token{
			UID: uid,
			Claims: map[string]interface{}{
				"email": email,
				"name":  user.Name,
			},
			Expires:  time.Now().Add(time.Hour * 24).Unix(),
			IssuedAt: time.Now().Unix(),
			Issuer:   "test",
			Audience: "test",
			Subject:  uid,
			Firebase: firebaseAuth.FirebaseInfo{
				SignInProvider: "test",
			},
		}, nil)
	}

	return client
}

// RequestOptions is a function that can be used to modify a request.
type RequestOptions func(client *Client, req *http.Request)

// WithAuth adds an authorization header to the request.
func WithAuth() RequestOptions {
	return func(client *Client, req *http.Request) {
		if client.AuthToken == nil {
			Fail("no auth token found")
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *client.AuthToken))
	}
}

// WithHeader sets a single request header.
func WithHeader(key, value string) RequestOptions {
	return func(client *Client, req *http.Request) {
		req.Header.Set(key, value)
	}
}

// WithParams adds query parameters to the request.
func WithParams(params map[string]string) RequestOptions {
	return func(client *Client, req *http.Request) {
		if len(params) > 0 {
			q := req.URL.Query()
			for key, value := range params {
				q.Set(key, value)
			}
			req.URL.RawQuery = q.Encode()
		}
	}
}

// WithContentType adds a custom content type header to the request.
func WithContentType(contentType string) RequestOptions {
	return func(client *Client, req *http.Request) {
		req.Header.Set("Content-Type", contentType)
	}
}

// MultipartFormData represents multipart form data for testing
type MultipartFormData struct {
	Fields map[string]string
	Files  map[string]struct {
		Filename string
		Content  io.Reader
	}
}

// WithMultipartFormData creates a RequestOptions that sets up multipart/form-data with fields and files
func WithMultipartFormData(data *MultipartFormData) RequestOptions {
	return func(client *Client, req *http.Request) {
		var bodyBuffer bytes.Buffer
		writer := multipart.NewWriter(&bodyBuffer)

		// Add form fields
		for key, value := range data.Fields {
			err := writer.WriteField(key, value)
			if err != nil {
				Fail(fmt.Sprintf("failed to write form field %s: %v", key, err))
			}
		}

		// Add file fields
		for fieldName, fileData := range data.Files {
			part, err := writer.CreateFormFile(fieldName, fileData.Filename)
			if err != nil {
				Fail(fmt.Sprintf("failed to create form file %s: %v", fieldName, err))
			}
			_, err = io.Copy(part, fileData.Content)
			if err != nil {
				Fail(fmt.Sprintf("failed to copy file content for %s: %v", fieldName, err))
			}
		}

		err := writer.Close()
		if err != nil {
			Fail(fmt.Sprintf("failed to close multipart writer: %v", err))
		}

		req.Body = io.NopCloser(&bodyBuffer)
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}
}

// MakeRequest makes an HTTP request to the test server with auth headers
func (c *Client) MakeRequest(
	method, path string,
	body interface{},
	opts ...RequestOptions,
) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		// Check if body is already an io.Reader (e.g., bytes.Buffer for multipart)
		if reader, ok := body.(io.Reader); ok {
			bodyReader = reader
		} else {
			// Marshal to JSON for regular requests
			jsonData, err := json.Marshal(body)
			if err != nil {
				Fail("failed to marshal request body: " + err.Error())
			}
			bodyReader = bytes.NewBuffer(jsonData)
		}
	}

	// Parse path to separate actual path from query string
	pathOnly := path
	var rawQuery string
	if idx := strings.Index(path, "?"); idx != -1 {
		pathOnly = path[:idx]
		rawQuery = path[idx+1:]
	}

	baseURL, err := url.Parse(c.BaseURL)
	if err != nil {
		Fail("failed to parse base URL: " + err.Error())
	}

	pathURL, err := url.Parse(pathOnly)
	if err != nil {
		Fail("failed to parse path: " + err.Error())
	}

	fullURL := baseURL.ResolveReference(pathURL)
	if rawQuery != "" {
		fullURL.RawQuery = rawQuery
	}

	req, err := http.NewRequest(method, fullURL.String(), bodyReader)
	if err != nil {
		Fail("failed to create request: " + err.Error())
	}

	// Set default Content-Type only if not already set by options
	// This allows multipart forms to set their own content type
	defaultContentType := true
	for _, opt := range opts {
		opt(c, req)
		// Check if Content-Type was set by the option
		if req.Header.Get("Content-Type") != "" && req.Header.Get("Content-Type") != "application/json" {
			defaultContentType = false
		}
	}
	if defaultContentType {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		Fail("failed to make request: " + err.Error())
	}
	DeferCleanup(resp.Body.Close)
	return resp, nil
}

// ParseResponse parses JSON response body into a map
func ParseResponse(resp *http.Response) map[string]interface{} {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		Fail("failed to read response body: " + err.Error())
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		Fail("failed to parse response: " + err.Error())
	}

	return result
}

// ParseResponseArray parses JSON response body into an array
func ParseResponseArray(resp *http.Response) ([]interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var result []interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result, nil
}

// MakeMultipartRequest makes an HTTP request with multipart/form-data for file uploads
func MakeMultipartRequest(method, url, token string, filePath, fieldName string) (*http.Response, error) {
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

func CreateImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	return img
}
