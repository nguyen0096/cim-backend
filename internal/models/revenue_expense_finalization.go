package models

import "time"

type RevenueExpenseFinalizationStatus string

const (
	RevenueExpenseFinalizationStatusSuccess RevenueExpenseFinalizationStatus = "success"
	RevenueExpenseFinalizationStatusFailed  RevenueExpenseFinalizationStatus = "failed"
)

// RevenueExpenseFinalization represents a historical record of revenue/expense finalization
type RevenueExpenseFinalization struct {
	Base
	FinalizedDate time.Time                         `json:"finalized_date" gorm:"not null"`
	Status        *RevenueExpenseFinalizationStatus `json:"status,omitempty"`
	Reason        *string                           `json:"reason,omitempty"`
}
