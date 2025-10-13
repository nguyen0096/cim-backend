package handlers

import (
	"cim-backend/internal/auth"
	"cim-backend/internal/models"
	"cim-backend/internal/services"
	"cim-backend/pkg"
	"context"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	userService  *services.UserService
	firebaseAuth *auth.FirebaseAuthService
}

func NewUserHandler(userService *services.UserService, firebaseAuth *auth.FirebaseAuthService) *UserHandler {
	return &UserHandler{
		userService:  userService,
		firebaseAuth: firebaseAuth,
	}
}

// VerifyToken verifies a Firebase token and creates/updates user
func (h *UserHandler) VerifyToken(c echo.Context) error {
	var req struct {
		Token string `json:"token" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Verify Firebase token
	ctx := context.Background()
	firebaseToken, err := h.firebaseAuth.VerifyToken(ctx, req.Token)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token"})
	}

	// Extract user information
	uid := firebaseToken.UID
	email, _ := firebaseToken.Claims["email"].(string)
	name, _ := firebaseToken.Claims["name"].(string)
	role, _ := firebaseToken.Claims["role"].(string)

	// Create or update user
	user, err := h.userService.CreateOrUpdateUser(ctx, uid, email, name)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create/update user"})
	}

	// Set custom claims if role is provided and different from database
	if role != "" && role != user.Role {
		claims := map[string]interface{}{
			"role": user.Role,
		}
		if err := h.firebaseAuth.SetCustomClaims(ctx, uid, claims); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to set user role"})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
		"user": map[string]interface{}{
			"id":    user.ID,
			"uid":   user.UID,
			"email": user.Email,
			"name":  user.Name,
			"role":  user.Role,
		},
	})
}

// GetProfile retrieves the current user's profile
func (h *UserHandler) GetProfile(c echo.Context) error {
	userID, _ := c.Get(pkg.AuthContextKeyUserID).(string)
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	ctx := context.Background()
	user, err := h.userService.GetUserByUID(ctx, userID)
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
	// Get pagination parameters
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 10 // default
	offset := 0 // default

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	ctx := context.Background()
	users, total, err := h.userService.ListUsers(ctx, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list users"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"users":  users,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// UpdateUserRole updates a user's role (admin only)
func (h *UserHandler) UpdateUserRole(c echo.Context) error {
	userUID := c.Param("uid")
	if userUID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "User UID is required"})
	}

	var req struct {
		Role string `json:"role" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request"})
	}

	// Get current user (admin performing the action)
	currentUserID, _ := c.Get(pkg.AuthContextKeyUserID).(string)

	ctx := context.Background()
	err := h.userService.UpdateUserRole(ctx, userUID, req.Role, currentUserID)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update user role"})
	}

	// Update Firebase custom claims
	claims := map[string]interface{}{
		"role": req.Role,
	}
	if err := h.firebaseAuth.SetCustomClaims(ctx, userUID, claims); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update Firebase claims"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User role updated successfully"})
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

	ctx := context.Background()
	users, err := h.userService.GetUsersByRole(ctx, role)
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
	userID, _ := c.Get(pkg.AuthContextKeyUserID).(string)
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}

	ctx := context.Background()
	permissions, err := h.userService.GetUserPermissions(ctx, userID)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to get user permissions"})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user_id":     userID,
		"permissions": permissions,
	})
}

// DeleteUser soft deletes a user (admin only)
func (h *UserHandler) DeleteUser(c echo.Context) error {
	userID := c.Param("id")
	if userID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "User ID is required"})
	}

	ctx := context.Background()
	err := h.userService.DeleteUser(ctx, userID)
	if err != nil {
		if appErr, ok := err.(*pkg.AppError); ok {
			return c.JSON(appErr.HTTPStatus(), map[string]string{"error": appErr.Message})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete user"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "User deleted successfully"})
}
