package domain

import "time"

type Language string

const (
	LangRU Language = "ru"
	LangEN Language = "en"
)

type User struct {
	ID           int64     `db:"id" json:"id"`
	TelegramID   int64     `db:"telegram_id" json:"telegram_id"`
	DisplayName  string    `db:"display_name" json:"display_name"`
	Username     *string   `db:"username" json:"username,omitempty"`
	IsFamilyHead bool      `db:"is_family_head" json:"is_family_head"`
	Language     Language  `db:"language" json:"language"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}
