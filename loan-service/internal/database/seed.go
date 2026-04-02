package database

import (
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/loan-service/internal/models"
	"gorm.io/gorm"
)

// SeedClientLoans creates two active loans (with installments) for each default test
// client. Idempotent — skips clients/accounts that aren't found yet and skips clients
// that already have loans.
func SeedClientLoans(db *gorm.DB) error {
	clientEmails := []string{
		"klijent@bank.com",
		"jelena.nikolic@bank.com",
		"nikola.djordjevic@bank.com",
	}

	// Resolve RSD currency ID
	var rsdCurrency struct {
		ID uint
	}
	if err := db.Table("currencies").Where("kod = ?", "RSD").First(&rsdCurrency).Error; err != nil {
		slog.Warn("RSD currency not found, skipping loan seed")
		return nil
	}

	for _, email := range clientEmails {
		// Find client
		var client struct {
			ID uint
		}
		if err := db.Table("clients").Where("email = ?", email).First(&client).Error; err != nil {
			slog.Warn("Client not found, skipping loans", "email", email)
			continue
		}

		// Skip if client already has loans
		var count int64
		db.Model(&models.Loan{}).Where("client_id = ?", client.ID).Count(&count)
		if count > 0 {
			slog.Info("Client already has loans, skipping", "email", email)
			continue
		}

		// Find tekuci RSD account
		var account models.Account
		if err := db.Table("accounts").
			Where("client_id = ? AND tip = ? AND currency_id = ?", client.ID, "tekuci", rsdCurrency.ID).
			First(&account).Error; err != nil {
			slog.Warn("Tekuci RSD account not found, skipping loans", "email", email)
			continue
		}

		type loanSpec struct {
			vrsta, tipKamate        string
			iznos                   float64
			period                  int
			kamatnaStopa, iznosRate float64
		}
		specs := []loanSpec{
			{
				vrsta: "gotovinski", tipKamate: "fiksna",
				iznos: 50_000, period: 24,
				kamatnaStopa: 10.0,
				iznosRate:    loanAnnuity(50_000, 10.0, 24),
			},
			{
				vrsta: "stambeni", tipKamate: "varijabilna",
				iznos: 200_000, period: 120,
				kamatnaStopa: 7.0,
				iznosRate:    loanAnnuity(200_000, 7.0, 120),
			},
		}

		for _, s := range specs {
			now := time.Now()
			loan := models.Loan{
				Vrsta:             s.vrsta,
				BrojRacuna:        account.BrojRacuna,
				BrojKredita:       fmt.Sprintf("KRED-%d-%06d", now.UnixMilli(), rand.Intn(1_000_000)),
				Iznos:             s.iznos,
				Period:            s.period,
				KamatnaStopa:      s.kamatnaStopa,
				TipKamate:         s.tipKamate,
				DatumKreiranja:    now,
				DatumDospeca:      now.AddDate(0, s.period, 0),
				IznosRate:         s.iznosRate,
				Status:            "aktivan",
				ClientID:          client.ID,
				CurrencyID:        rsdCurrency.ID,
				SvrhaKredita:      "Seed data",
				IznosMesecnePlate: 80_000,
				StatusZaposlenja:  "stalno",
				PeriodZaposlenja:  "5 godina",
				KontaktTelefon:    "0601234567",
			}

			if err := db.Transaction(func(tx *gorm.DB) error {
				if err := tx.Create(&loan).Error; err != nil {
					return fmt.Errorf("failed to create loan: %w", err)
				}

				// Credit loan amount to client's account
				if err := tx.Table("accounts").Where("id = ?", account.ID).Updates(map[string]interface{}{
					"stanje":             account.Stanje + s.iznos,
					"raspolozivo_stanje": account.RaspolozivoStanje + s.iznos,
				}).Error; err != nil {
					return fmt.Errorf("failed to credit account: %w", err)
				}
				// Keep local copy in sync for subsequent loans
				account.Stanje += s.iznos
				account.RaspolozivoStanje += s.iznos

				installments := makeSeedInstallments(&loan)
				if err := tx.Create(&installments).Error; err != nil {
					return fmt.Errorf("failed to create installments: %w", err)
				}
				return nil
			}); err != nil {
				return fmt.Errorf("loan seed transaction failed for %s: %w", email, err)
			}

			slog.Info("Seeded loan", "client", email, "vrsta", s.vrsta, "iznos", s.iznos)
		}
	}

	slog.Info("Client loans seed complete")
	return nil
}

func loanAnnuity(principal, annualRatePercent float64, periodMonths int) float64 {
	r := annualRatePercent / 100.0 / 12.0
	n := float64(periodMonths)
	return math.Round(principal*r*math.Pow(1+r, n)/(math.Pow(1+r, n)-1)*100) / 100
}

func makeSeedInstallments(loan *models.Loan) []models.LoanInstallment {
	installments := make([]models.LoanInstallment, loan.Period)
	for i := range installments {
		installments[i] = models.LoanInstallment{
			LoanID:              loan.ID,
			RedniBroj:           i + 1,
			Iznos:               loan.IznosRate,
			KamataStopaSnapshot: loan.KamatnaStopa,
			DatumDospeca:        time.Now().AddDate(0, i+1, 0),
			Status:              "ocekuje",
		}
	}
	return installments
}
