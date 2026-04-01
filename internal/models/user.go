package models

type User struct {
	Base
	FullName     string  `db:"full_name" json:"full_name"`
	Email        string  `db:"email" json:"email"`
	PasswordHash string  `db:"password_hash" json:"-"`
	Role         string  `db:"role" json:"role"`
	Balance      float64 `db:"balance" json:"balance"`
	IsApproved   bool    `db:"is_approved" json:"is_approved"`
}
