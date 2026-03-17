package models

import "time"

// Client is a read-only reference type used by Account for foreign key resolution.
type Client struct {
	ID        uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Ime       string    `json:"ime"`
	Prezime   string    `json:"prezime"`
	Email     string    `gorm:"uniqueIndex;not null" json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
