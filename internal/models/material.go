package models

type Material struct {
	Base
	Name          string  `db:"name" json:"name"`
	StockQuantity float64 `db:"stock_quantity" json:"stock_quantity"`
	Unit          string  `db:"unit" json:"unit"`
}
