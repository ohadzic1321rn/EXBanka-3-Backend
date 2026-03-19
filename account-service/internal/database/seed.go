package database

import (
	"log/slog"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/account-service/internal/models"
	"gorm.io/gorm"
)

func SeedCurrencies(db *gorm.DB) error {
	currencies := []models.Currency{
		{Kod: "RSD", Naziv: "Srpski dinar", Simbol: "RSD", Drzava: "Srbija", Aktivan: true},
		{Kod: "EUR", Naziv: "Evro", Simbol: "€", Drzava: "Evropska unija", Aktivan: true},
		{Kod: "USD", Naziv: "Američki dolar", Simbol: "$", Drzava: "SAD", Aktivan: true},
		{Kod: "GBP", Naziv: "Britanska funta", Simbol: "£", Drzava: "Velika Britanija", Aktivan: true},
		{Kod: "CHF", Naziv: "Švajcarski franak", Simbol: "CHF", Drzava: "Švajcarska", Aktivan: true},
		{Kod: "JPY", Naziv: "Japanski jen", Simbol: "¥", Drzava: "Japan", Aktivan: true},
		{Kod: "CAD", Naziv: "Kanadski dolar", Simbol: "C$", Drzava: "Kanada", Aktivan: true},
		{Kod: "AUD", Naziv: "Australijski dolar", Simbol: "A$", Drzava: "Australija", Aktivan: true},
	}

	for _, c := range currencies {
		var existing models.Currency
		result := db.Where("kod = ?", c.Kod).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			if err := db.Create(&c).Error; err != nil {
				return err
			}
			slog.Info("Seeded currency", "kod", c.Kod)
		}
	}

	slog.Info("Currency seed complete")
	return nil
}
