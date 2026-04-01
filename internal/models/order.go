package models

import "github.com/google/uuid"

type Order struct {
	Base
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	TotalPrice  float64   `db:"total_price" json:"total_price"`
	Status      string    `db:"status" json:"status"`
	Note        string    `db:"note" json:"note"`
	QRCodeToken string    `db:"qr_code_token" json:"qr_code_token"`
}

type OrderItem struct {
	Base
	OrderID   uuid.UUID `db:"order_id" json:"order_id"`
	ProductID uuid.UUID `db:"product_id" json:"product_id"`
	Quantity  int       `db:"quantity" json:"quantity"`
	UnitPrice float64   `db:"unit_price" json:"unit_price"`
}
