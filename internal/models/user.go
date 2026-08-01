package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user in the system
type User struct {
	ID        uuid.UUID      `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UID       string         `json:"uid" gorm:"index"` // Firebase UID
	Email     string         `json:"email" gorm:"uniqueIndex;not null"`
	Name      string         `json:"name"`
	Role      UserRole       `json:"role" gorm:"default:'staff'"`                                                    // admin, accountant, staff, bot_form, chef, waiter, cashier, developer
	Type      UserType       `json:"type" gorm:"default:'user'"`                                                     // developer, user
	Status    string         `json:"status" gorm:"default:active;check:status IN ('active', 'pending', 'inactive')"` // active, pending, inactive
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

// UserType represents the type of user in the system
type UserType string

const (
	UserTypeDeveloper UserType = "developer"
	UserTypeUser      UserType = "user"
)

// UserRole represents the available roles in the system
type UserRole string

const (
	RoleAdmin      UserRole = "admin"
	RoleAccountant UserRole = "accountant"
	RoleStaff      UserRole = "staff"
	RoleBotForm    UserRole = "bot_form"
	RoleChef       UserRole = "chef"
	RoleWaiter     UserRole = "waiter"
	RoleCashier    UserRole = "cashier"
	// RoleDeveloper is standalone: it holds only the developer-tools screen and the
	// tool endpoints, and is not derived from nor does it imply admin.
	RoleDeveloper UserRole = "developer"
)

// IsValidRole checks if the role is valid
func (r UserRole) IsValidRole() bool {
	switch r {
	case RoleAdmin, RoleAccountant, RoleStaff, RoleBotForm, RoleChef, RoleWaiter, RoleCashier, RoleDeveloper:
		return true
	default:
		return false
	}
}
