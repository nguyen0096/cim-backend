package dto

// CreateUserRequest represents the request for creating a new user
type CreateUserRequest struct {
	UID    string `json:"uid" example:"firebase-uid-123"`
	Email  string `json:"email" validate:"required,email" example:"user@example.com"`
	Name   string `json:"name" example:"John Doe"`
	Status string `json:"status" validate:"required,oneof=active pending inactive" example:"active"`
	Role   string `json:"role" validate:"required,oneof=admin accountant staff" example:"staff"`
}

// CreateUserResponse represents the response for creating a new user
type CreateUserResponse struct {
	ID        string `json:"id" example:"550e8400-e29b-41d4-a716-446655440000"`
	UID       string `json:"uid" example:"firebase-uid-123"`
	Email     string `json:"email" example:"user@example.com"`
	Name      string `json:"name" example:"John Doe"`
	Role      string `json:"role" example:"staff"`
	Status    string `json:"status" example:"active"`
	CreatedAt string `json:"created_at" example:"2023-01-01T00:00:00Z"`
}

// UpdateUserRequest represents the request for updating a user
type UpdateUserRequest struct {
	Name   string `json:"name" example:"John Doe"`
	Role   string `json:"role" validate:"required,oneof=admin accountant staff" example:"staff"`
	Status string `json:"status" validate:"required,oneof=active pending inactive" example:"active"`
}
