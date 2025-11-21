package pkg

import (
	"testing"

	"github.com/shopspring/decimal"
)

type TestStruct struct {
	Price decimal.Decimal `validate:"required"`
}

func TestValidateDecimal(t *testing.T) {
	tests := []struct {
		name    string
		input   TestStruct
		wantErr bool
	}{
		{
			name:    "valid decimal",
			input:   TestStruct{Price: decimal.NewFromFloat(10.5)},
			wantErr: false,
		},
		{
			name:    "zero decimal",
			input:   TestStruct{Price: decimal.Zero},
			wantErr: false,
		},
		{
			name:    "negative decimal",
			input:   TestStruct{Price: decimal.NewFromFloat(-5.25)},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validator.Struct(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validator.Struct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
