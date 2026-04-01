package models

import "github.com/google/uuid"

type Product struct {
	Base
	Name     string  `db:"name" json:"name"`
	Price    float64 `db:"price" json:"price"`
	Category string  `db:"category" json:"category"`
	IsActive bool    `db:"is_active" json:"is_active"`
	ImageURL string  `db:"image_url" json:"image_url"`
}

type ProductMaterial struct {
	ProductID      uuid.UUID `db:"product_id" json:"product_id"`
	MaterialID     uuid.UUID `db:"material_id" json:"material_id"`
	QuantityNeeded float64   `db:"quantity_needed" json:"quantity_needed"`
}
