package models

type SifraDelatnosti struct {
	ID    uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Sifra string `gorm:"uniqueIndex;not null" json:"sifra"`
	Naziv string `gorm:"not null" json:"naziv"`
}
