package models

import "time"

type Card struct {
	ID             uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	BrojKartice    string    `gorm:"uniqueIndex;size:16;not null" json:"broj_kartice"`
	CVV            string    `gorm:"size:3;not null" json:"-"`
	VrstaKartice   string    `gorm:"not null" json:"vrsta_kartice"` // visa, mastercard, dinacard, amex
	NazivKartice   string    `json:"naziv_kartice"`
	AccountID      uint      `gorm:"not null" json:"account_id"`
	ClientID       uint      `gorm:"not null" json:"client_id"`
	Status         string    `gorm:"default:'aktivna';not null" json:"status"` // aktivna, blokirana, deaktivirana
	DatumKreiranja time.Time `json:"datum_kreiranja"`
	DatumIsteka    time.Time `json:"datum_isteka"` // +5 years from creation
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ValidCardTypes returns the supported card network types.
func ValidCardTypes() []string {
	return []string{"visa", "mastercard", "dinacard", "amex"}
}

// ValidCardStatuses returns all possible card status values.
func ValidCardStatuses() []string {
	return []string{"aktivna", "blokirana", "deaktivirana"}
}
