package handlers

import (
	"cim-backend/internal/auth"
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/internal/services/dto"
	"cim-backend/pkg"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	userService  *services.UserService
	firebaseAuth auth.FirebaseAuthInterface
}

func NewUserHandler(userService *services.UserService, firebaseAuth auth.FirebaseAuthInterface) *UserHandler {
	return &UserHandler{
		userService:  userService,
		firebaseAuth: firebaseAuth,
	}
}

// GetProfile retrieves the current user's profile
func (h *UserHandler) GetProfile(c echo.Context) error {
	userID, _ := c.Get(pkg.AuthContextKeyUserID.String()).(string)
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	user, err := h.userService.GetUserByUID(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get user"})
	}
	if user == nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":    user.ID,
		"uid":   user.UID,
		"email": user.Email,
		"name":  user.Name,
		"role":  user.Role,
	})
}

// ListUsers retrieves all users (admin only)
func (h *UserHandler) ListUsers(c echo.Context) error {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get current user ID to exclude from results
	currentUserID, _ := c.Get(pkg.AuthContextKeyUserID.String()).(string)

	users, total, err := h.userService.ListUsers(c.Request().Context(), limit, offset, currentUserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list users"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       users,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
	}

	return c.JSON(http.StatusOK, response)
}

// UpdateUser updates all properties of a user (admin only)
// @Summary Update user
// @Description Update all properties of a user including email, name, role, and status
// @Tags users
// @Accept json
// @Produce json
// @Param uid path string true "User UID"
// @Param user body dto.UpdateUserRequest true "User update data"
// @Success 200 {object} map[string]string "User updated successfully"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /users/{uid} [put]
func (h *UserHandler) UpdateUser(c echo.Context) error {
	userID := c.Param("id")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "User ID is required"})
	}

	var req dto.UpdateUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request", "details": err.Error()})
	}

	// Validate request
	if err := pkg.Validator.Struct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	// Get current user (admin performing the action)
	err := h.userService.UpdateUser(c.Request().Context(), userID, "", req.Name, req.Role, req.Status, false)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user"})
	}

	// Update Firebase custom claims
	claims := map[string]interface{}{
		"role": req.Role,
	}
	if err := h.firebaseAuth.SetCustomClaims(c.Request().Context(), userID, claims); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update Firebase claims"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User updated successfully"})
}

// GetUsersByRole retrieves users by role (admin only)
func (h *UserHandler) GetUsersByRole(c echo.Context) error {
	role := c.Param("role")
	if role == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Role is required"})
	}

	// Validate role
	if !models.UserRole(role).IsValidRole() {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid role"})
	}

	users, err := h.userService.GetUsersByRole(c.Request().Context(), role)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get users by role"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"users": users,
		"role":  role,
	})
}

// GetUserPermissions retrieves all permissions for the current user based on their roles
// @Summary Get current user permissions
// @Description Get all permissions for the current authenticated user based on their roles in Casbin
// @Tags users
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "User permissions"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 404 {object} map[string]string "User not found"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /users/permissions [get]
func (h *UserHandler) GetUserPermissions(c echo.Context) error {
	userEmail, _ := c.Get(pkg.AuthContextKeyUserEmail.String()).(string)
	if userEmail == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user email"})
	}

	permissions, err := h.userService.GetUserPermissions(c.Request().Context(), userEmail)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get user permissions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"permissions": permissions,
	})
}

// CreateUser creates a new user (admin only)
// @Summary Create a new user
// @Description Create a new user with specified UID, email, name, and role
// @Tags users
// @Accept json
// @Produce json
// @Param user body dto.CreateUserRequest true "User creation data"
// @Success 201 {object} dto.CreateUserResponse "User created successfully"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 409 {object} map[string]string "User already exists"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /users [post]
func (h *UserHandler) CreateUser(c echo.Context) error {
	var req dto.CreateUserRequest

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request", "details": err.Error()})
	}

	// Validate request
	if err := pkg.Validator.Struct(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Validation failed", "details": err.Error()})
	}

	user, err := h.userService.CreateUser(c.Request().Context(), req.UID, req.Email, req.Name, req.Role, req.Status)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	// Convert to response DTO
	response := dto.CreateUserResponse{
		ID:        user.ID.String(),
		UID:       user.UID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      string(user.Role),
		Status:    user.Status,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}

	return c.JSON(http.StatusCreated, response)
}

// DeleteUser soft deletes a user (admin only)
func (h *UserHandler) DeleteUser(c echo.Context) error {
	userID := c.Param("id")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "User ID is required"})
	}

	err := h.userService.DeleteUser(c.Request().Context(), userID)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User deleted successfully"})
}

// SearchUsers searches users by name or email with pagination (admin only)
// @Summary Search users
// @Description Search users by name or email with pagination support
// @Tags users
// @Accept json
// @Produce json
// @Param query query string false "Search query for name or email"
// @Param limit query int false "Number of results per page (default: 20)"
// @Param page query int false "Page number (default: 1)"
// @Success 200 {object} map[string]interface{} "Search results with pagination"
// @Failure 400 {object} map[string]string "Bad request"
// @Failure 500 {object} map[string]string "Internal server error"
// @Security BearerAuth
// @Router /users/search [get]
func (h *UserHandler) SearchUsers(c echo.Context) error {
	// Parse query parameters
	query := c.QueryParam("q")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	page, _ := strconv.Atoi(c.QueryParam("page"))

	// Set defaults
	if limit == 0 {
		limit = 20
	}
	if page == 0 {
		page = 1
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Get current user ID to exclude from results
	currentUserID, _ := c.Get(pkg.AuthContextKeyUserID.String()).(string)

	users, total, err := h.userService.SearchUsers(c.Request().Context(), query, limit, offset, currentUserID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to search users"})
	}

	// Calculate total pages
	totalPages := int((total + int64(limit) - 1) / int64(limit))

	// Create response
	response := map[string]interface{}{
		"data":       users,
		"total":      total,
		"page":       page,
		"limit":      limit,
		"totalPages": totalPages,
		"query":      query,
	}

	return c.JSON(http.StatusOK, response)
}
