package models

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestGetEffectiveSellingPrice_NilReceiver(t *testing.T) {
	var p *POItemSellingPrice
	result := p.GetEffectiveSellingPrice()
	assert.Nil(t, result)
}

func TestGetEffectiveSellingPrice_OverrideSet(t *testing.T) {
	override := decimal.NewFromFloat(25.50)
	ledgerPrice := decimal.NewFromFloat(20.00)

	p := &POItemSellingPrice{
		SellingPrice: &override,
		SellingPriceRef: &SellingPrice{
			Price: ledgerPrice,
		},
	}

	result := p.GetEffectiveSellingPrice()
	assert.NotNil(t, result)
	assert.True(t, result.Equal(override), "should return override price when set")
}

func TestGetEffectiveSellingPrice_FallbackToLedger(t *testing.T) {
	ledgerPrice := decimal.NewFromFloat(20.00)

	p := &POItemSellingPrice{
		SellingPrice: nil, // not overridden
		SellingPriceRef: &SellingPrice{
			Price: ledgerPrice,
		},
	}

	result := p.GetEffectiveSellingPrice()
	assert.NotNil(t, result)
	assert.True(t, result.Equal(ledgerPrice), "should fall back to ledger price when override is nil")
}

func TestGetEffectiveSellingPrice_BothNil(t *testing.T) {
	p := &POItemSellingPrice{
		SellingPrice:    nil,
		SellingPriceRef: nil,
	}

	result := p.GetEffectiveSellingPrice()
	assert.Nil(t, result, "should return nil when both override and ledger are nil")
}

func TestGetEffectiveSellingPrice_OverrideZero(t *testing.T) {
	zero := decimal.NewFromFloat(0)
	ledgerPrice := decimal.NewFromFloat(20.00)

	p := &POItemSellingPrice{
		SellingPrice: &zero,
		SellingPriceRef: &SellingPrice{
			Price: ledgerPrice,
		},
	}

	result := p.GetEffectiveSellingPrice()
	assert.NotNil(t, result)
	assert.True(t, result.Equal(zero), "should return override even if zero (explicit override)")
}
