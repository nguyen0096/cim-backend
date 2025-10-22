package dto

import "cim-backend/internal/models"

// ReconcileInventoryRequest represents the request for confirming inventory
type ReconcileInventoryRequest struct {
	InventoryID uint            `json:"inventory_id" validate:"required" param:"id"`
	Items       []ReconcileItem `json:"items" validate:"required,min=1,dive"`
}

// InventoryItemQuantity represents a single inventory item disposal
type ReconcileItem struct {
	InventoryItemID uint                  `json:"inventory_item_id" validate:"required"`
	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	ActualQuantity  *int                  `json:"actual_quantity" validate:"required,min=0"`
	PrevQuantity    int                   `json:"prev_quantity,omitempty" validate:"required,min=1"`
}

// DisposeInventoryRequest represents the request for disposing inventory items
type DisposeInventoryRequest struct {
	InventoryID uint          `json:"inventory_id" validate:"required" param:"id"`
	Items       []DisposeItem `json:"items" validate:"required,min=1,dive"`
}

// InventoryItemQuantity represents a single inventory item disposal
type DisposeItem struct {
	InventoryItemID uint                  `json:"inventory_item_id" validate:"required"`
	InventoryItem   *models.InventoryItem `json:"inventory_item,omitempty" validate:"-"`
	Quantity        *int                  `json:"quantity" validate:"required,min=1"`
	PrevQuantity    int                   `json:"prev_quantity,omitempty" validate:"required,min=1"`
}

// ReconcileSubmissionPayload represents the payload for a reconcile submission
type ReconcileSubmissionPayload struct {
	Items []ReconcileItem `json:"items"`
}

// DisposeSubmissionPayload represents the payload for a dispose submission
type DisposeSubmissionPayload struct {
	Items []DisposeItem `json:"items"`
}
