package database

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/auth-service/internal/models"
	"github.com/RAF-SI-2025/EXBanka-3-Backend/auth-service/internal/util"
	"gorm.io/gorm"
)

func SeedDefaultAdmin(db *gorm.DB) error {
	const defaultAdminPassword = "Admin123!"

	var existing models.Employee
	if result := db.Where("email = ?", "admin@bank.com").First(&existing); result.Error == nil {
		ok, err := util.VerifyPassword(defaultAdminPassword, existing.SaltPassword, existing.Password)
		if err != nil {
			return fmt.Errorf("failed to verify existing admin password: %w", err)
		}

		var adminPerm models.Permission
		if err := db.Where("name = ?", models.PermEmployeeAdmin).First(&adminPerm).Error; err != nil {
			return fmt.Errorf("failed to fetch employeeAdmin permission: %w", err)
		}

		if ok && existing.Aktivan {
			if err := db.Model(&existing).Association("Permissions").Replace([]models.Permission{adminPerm}); err != nil {
				return fmt.Errorf("failed to sync admin permissions: %w", err)
			}
			slog.Info("Admin already exists, skipping seed")
			return nil
		}

		salt, err := util.GenerateSalt()
		if err != nil {
			return fmt.Errorf("failed to generate salt: %w", err)
		}

		hashedPwd, err := util.HashPassword(defaultAdminPassword, salt)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		updates := map[string]interface{}{
			"password":      hashedPwd,
			"salt_password": salt,
			"aktivan":       true,
		}
		if err := db.Model(&existing).Updates(updates).Error; err != nil {
			return fmt.Errorf("failed to repair existing admin: %w", err)
		}
		if err := db.Model(&existing).Association("Permissions").Replace([]models.Permission{adminPerm}); err != nil {
			return fmt.Errorf("failed to sync admin permissions: %w", err)
		}

		slog.Info("Default admin user repaired successfully")
		return nil
	} else if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing admin: %w", result.Error)
	}

	salt, err := util.GenerateSalt()
	if err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}

	hashedPwd, err := util.HashPassword(defaultAdminPassword, salt)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	var adminPerm models.Permission
	if err := db.Where("name = ?", models.PermEmployeeAdmin).First(&adminPerm).Error; err != nil {
		return fmt.Errorf("failed to fetch employeeAdmin permission: %w", err)
	}

	admin := models.Employee{
		Ime:           "Admin",
		Prezime:       "User",
		DatumRodjenja: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Pol:           "M",
		Email:         "admin@bank.com",
		BrojTelefona:  "0601234567",
		Adresa:        "Admin Street 1",
		Username:      "admin",
		Password:      hashedPwd,
		SaltPassword:  salt,
		Pozicija:      "Administrator",
		Departman:     "IT",
		Aktivan:       true,
		Permissions:   []models.Permission{adminPerm},
	}

	if err := db.Create(&admin).Error; err != nil {
		return fmt.Errorf("failed to create admin: %w", err)
	}

	slog.Info("Default admin user created successfully")
	return nil
}

// SeedDefaultEmployees seeds two default bank employees if they don't already exist.
func SeedDefaultEmployees(db *gorm.DB) error {
	type empSpec struct {
		ime, prezime, email, username, pozicija, departman, perm string
	}
	specs := []empSpec{
		{"Marko", "Petrovic", "marko.petrovic@bank.com", "marko.petrovic", "Referent", "Retail", models.PermEmployeeBasic},
		{"Ana", "Jovic", "ana.jovic@bank.com", "ana.jovic", "Agent", "Retail", models.PermEmployeeAgent},
	}

	const defaultPassword = "Zaposleni123!"

	for _, s := range specs {
		var existing models.Employee
		if err := db.Where("email = ?", s.email).First(&existing).Error; err == nil {
			slog.Info("Employee already exists, skipping", "email", s.email)
			continue
		}

		salt, err := util.GenerateSalt()
		if err != nil {
			return fmt.Errorf("failed to generate salt for %s: %w", s.email, err)
		}
		hashedPwd, err := util.HashPassword(defaultPassword, salt)
		if err != nil {
			return fmt.Errorf("failed to hash password for %s: %w", s.email, err)
		}

		var perm models.Permission
		if err := db.Where("name = ?", s.perm).First(&perm).Error; err != nil {
			return fmt.Errorf("permission %s not found: %w", s.perm, err)
		}

		emp := models.Employee{
			Ime:           s.ime,
			Prezime:       s.prezime,
			DatumRodjenja: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
			Pol:           "M",
			Email:         s.email,
			BrojTelefona:  "0601234567",
			Adresa:        "Beograd",
			Username:      s.username,
			Password:      hashedPwd,
			SaltPassword:  salt,
			Pozicija:      s.pozicija,
			Departman:     s.departman,
			Aktivan:       true,
			Permissions:   []models.Permission{perm},
		}

		if err := db.Create(&emp).Error; err != nil {
			return fmt.Errorf("failed to create employee %s: %w", s.email, err)
		}
		slog.Info("Employee created", "email", s.email)
	}

	slog.Info("Default employees seed complete")
	return nil
}
