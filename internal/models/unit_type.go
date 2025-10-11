package models

type UnitType struct {
	Base
	Name string `json:"name" gorm:"not null"`
}
