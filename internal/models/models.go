package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"cim-backend/pkg"
)

type Base struct {
	ID        uint           `json:"id" gorm:"primaryKey;autoIncrement"`
	CreatedBy string         `json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedBy string         `json:"updated_by"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index" swaggertype:"string"`
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	// set CreatedBy to the user email
	if tx.Statement.Context == nil {
		return fmt.Errorf("context with user email is required to set CreatedBy")
	}

	userEmail, err := pkg.GetUserEmailFromContext(tx.Statement.Context)
	if err != nil {
		return err
	}
	b.CreatedBy = userEmail
	return nil
}

func (b *Base) BeforeUpdate(tx *gorm.DB) error {
	// set UpdatedBy to the user email
	if tx.Statement.Context == nil {
		return fmt.Errorf("context with user email is required to set UpdatedBy")
	}

	userEmail, err := pkg.GetUserEmailFromContext(tx.Statement.Context)
	if err != nil {
		return err
	}
	b.UpdatedBy = userEmail
	return nil
}
