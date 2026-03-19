package database

import (
	"log/slog"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/payment-service/internal/models"
	"gorm.io/gorm"
)

func SeedSifrePlacanja(db *gorm.DB) error {
	sifre := []models.SifraPlacanja{
		{Sifra: "221", Naziv: "Promet robe"},
		{Sifra: "222", Naziv: "Promet usluga"},
		{Sifra: "223", Naziv: "Zakup"},
		{Sifra: "240", Naziv: "Komunalne usluge"},
		{Sifra: "241", Naziv: "Struja"},
		{Sifra: "242", Naziv: "Gas"},
		{Sifra: "243", Naziv: "Voda"},
		{Sifra: "244", Naziv: "Telefon/Internet"},
		{Sifra: "253", Naziv: "Premije osiguranja"},
		{Sifra: "254", Naziv: "Prenos sredstava"},
		{Sifra: "265", Naziv: "Otplata kredita"},
		{Sifra: "270", Naziv: "Donacija"},
		{Sifra: "289", Naziv: "Ostale transakcije"},
		{Sifra: "290", Naziv: "Interni prenos"},
	}

	for _, s := range sifre {
		var existing models.SifraPlacanja
		result := db.Where("sifra = ?", s.Sifra).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			if err := db.Create(&s).Error; err != nil {
				return err
			}
			slog.Info("Seeded sifra placanja", "sifra", s.Sifra, "naziv", s.Naziv)
		}
	}

	slog.Info("Sifre placanja seed complete")
	return nil
}
