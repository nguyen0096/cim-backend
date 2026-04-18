package dto

import "github.com/shopspring/decimal"

type CreateSellingPriceRequest struct {
	ProductID     uint            `json:"product_id" validate:"required"`
	InventoryID   *uint           `json:"inventory_id"`
	Price         decimal.Decimal `json:"price" validate:"required"`
	EffectiveFrom string          `json:"effective_from" validate:"required"` // "2026-04-11" format
	Notes         string          `json:"notes"`
}

type UpdateSellingPriceRequest struct {
	Price         decimal.Decimal `json:"price" validate:"required"`
	EffectiveFrom string          `json:"effective_from" validate:"required"`
	Notes         string          `json:"notes"`
}

type UpdatePOItemSellingPriceRequest struct {
	SellingPrice decimal.Decimal `json:"selling_price" validate:"required"`
}
