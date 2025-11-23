package handlers

import (
	"bytes"
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// assertSuccessResponse asserts that the response is successful and returns the purchase order
func assertSuccessResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int) models.PurchaseOrder {
	assert.Equal(t, expectedStatus, rec.Code)

	var responseOrder models.PurchaseOrder
	err := json.Unmarshal(rec.Body.Bytes(), &responseOrder)
	require.NoError(t, err)

	return responseOrder
}

// assertErrorResponse asserts that the response contains the expected error
func assertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, expectedStatus int, expectedError string) {
	assert.Equal(t, expectedStatus, rec.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &responseBody)
	require.NoError(t, err)
	errorMsg, ok := responseBody["error"].(string)
	require.True(t, ok, "error field should be a string")
	assert.Contains(t, errorMsg, expectedError)
}

// assertValidationErrorResponse asserts that the response contains a validation error
func assertValidationErrorResponse(t *testing.T, rec *httptest.ResponseRecorder) {
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var responseBody map[string]interface{}
	err := json.Unmarshal(rec.Body.Bytes(), &responseBody)
	require.NoError(t, err)

	// Check that it's a validation error
	errorMsg, ok := responseBody["error"].(string)
	require.True(t, ok, "error field should be a string")
	assert.Contains(t, errorMsg, pkg.MsgErrValidation)
	assert.Equal(t, "validation", responseBody["code"])
}

// createRequest creates an HTTP request with JSON body
func createRequest(method, url string, body interface{}) (*http.Request, error) {
	var requestBody []byte
	var err error

	if str, ok := body.(string); ok {
		// Handle string body (for invalid JSON tests)
		requestBody = []byte(str)
	} else if body != nil {
		requestBody, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
	}

	req := httptest.NewRequest(method, url, bytes.NewBuffer(requestBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req, nil
}
