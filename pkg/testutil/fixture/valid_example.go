package fixture

import (
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func ValidInventory() models.Inventory {
	return models.Inventory{
		Base: models.Base{ID: 1},
		Name: fmt.Sprintf("Test Inventory %s", uuid.New().String()),
	}
}

func ValidSupplier() models.Supplier {
	return models.Supplier{
		Base: models.Base{ID: 1},
		Name: fmt.Sprintf("Test Supplier %s", uuid.New().String()),
	}
}

// ValidBaseUnit returns a valid base unit for testing.
func ValidBaseUnit() models.Unit {
	return models.Unit{
		Base:             models.Base{ID: 1},
		Name:             fmt.Sprintf("Test Base Unit %s", uuid.New().String()),
		Symbol:           fmt.Sprintf("TU-%s", uuid.New().String()[:8]), // Unique symbol
		UnitType:         "general",
		ConversionFactor: 1,
		Level:            1,
		DecimalPlaces:    0,
		BaseUnitID:       nil, // This is a base unit
	}
}

func ValidProduct(unitID uint) models.Product {
	return models.Product{
		Base:   models.Base{ID: 1},
		Name:   fmt.Sprintf("Test Product %s", uuid.New().String()),
		UnitID: unitID,
	}
}

func ValidPurchaseOrder(
	inventoryID uint,
	productID uint,
	supplierID uint,
	unitID uint,
) models.PurchaseOrder {
	return models.PurchaseOrder{
		Base:        models.Base{ID: 1},
		Status:      models.PurchaseOrderStatusOrderPlaced,
		Notes:       fmt.Sprintf("Test purchase order %s", uuid.New().String()[:8]),
		InventoryID: pkg.Ptr(inventoryID),
		Items: []*models.PurchaseOrderItem{
			{
				ProductID:  pkg.Ptr(productID),
				SupplierID: pkg.Ptr(supplierID),
				UnitID:     pkg.Ptr(unitID),
				Quantity:   decimal.NewFromInt(1),
			},
		},
	}
}
