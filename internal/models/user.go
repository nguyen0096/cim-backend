package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UID       string         `json:"uid" gorm:"uniqueIndex;not null"` // Firebase UID
	Email     string         `json:"email" gorm:"uniqueIndex;not null"`
	Name      string         `json:"name"`
	Role      string         `json:"role" gorm:"default:'staff'"` // admin, accountant, staff
	Active    bool           `json:"active" gorm:"default:true"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// UserRole represents the available roles in the system
type UserRole string

const (
	RoleAdmin      UserRole = "admin"
	RoleAccountant UserRole = "accountant"
	RoleStaff      UserRole = "staff"
)

// IsValidRole checks if the role is valid
func (r UserRole) IsValidRole() bool {
	switch r {
	case RoleAdmin, RoleAccountant, RoleStaff:
		return true
	default:
		return false
	}
}
