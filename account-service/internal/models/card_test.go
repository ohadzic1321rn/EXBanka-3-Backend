package models_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/account-service/internal/models"
)

func TestCard_HasRequiredFields(t *testing.T) {
	rt := reflect.TypeOf(models.Card{})
	required := []string{
		"ID", "BrojKartice", "CVV", "VrstaKartice", "NazivKartice",
		"AccountID", "ClientID", "Status",
		"DatumKreiranja", "DatumIsteka", "CreatedAt", "UpdatedAt",
	}
	for _, name := range required {
		if _, ok := rt.FieldByName(name); !ok {
			t.Errorf("Card missing field: %s", name)
		}
	}
}

func TestCard_BrojKarticeHasUniqueIndex(t *testing.T) {
	rt := reflect.TypeOf(models.Card{})
	f, ok := rt.FieldByName("BrojKartice")
	if !ok {
		t.Fatal("BrojKartice field not found")
	}
	tag := f.Tag.Get("gorm")
	if !strings.Contains(tag, "uniqueIndex") {
		t.Errorf("BrojKartice: expected gorm tag to contain uniqueIndex, got: %s", tag)
	}
	if !strings.Contains(tag, "size:16") {
		t.Errorf("BrojKartice: expected gorm tag to contain size:16, got: %s", tag)
	}
}

func TestCard_CVV_NotInJSON(t *testing.T) {
	rt := reflect.TypeOf(models.Card{})
	f, ok := rt.FieldByName("CVV")
	if !ok {
		t.Fatal("CVV field not found")
	}
	jsonTag := f.Tag.Get("json")
	if jsonTag != "-" {
		t.Errorf("CVV: expected json:\"-\" to hide from API responses, got: %q", jsonTag)
	}
}

func TestCard_StatusDefaultIsAktivna(t *testing.T) {
	rt := reflect.TypeOf(models.Card{})
	f, ok := rt.FieldByName("Status")
	if !ok {
		t.Fatal("Status field not found")
	}
	tag := f.Tag.Get("gorm")
	if !strings.Contains(tag, "default:'aktivna'") {
		t.Errorf("Status: expected gorm default='aktivna', got: %s", tag)
	}
}

func TestCard_ValidCardTypes(t *testing.T) {
	types := models.ValidCardTypes()
	expected := []string{"visa", "mastercard", "dinacard", "amex"}
	if len(types) != len(expected) {
		t.Errorf("expected %d card types, got %d", len(expected), len(types))
	}
	for _, e := range expected {
		found := false
		for _, g := range types {
			if g == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing card type: %s", e)
		}
	}
}

func TestCard_ValidCardStatuses(t *testing.T) {
	statuses := models.ValidCardStatuses()
	expected := []string{"aktivna", "blokirana", "deaktivirana"}
	if len(statuses) != len(expected) {
		t.Errorf("expected %d card statuses, got %d", len(expected), len(statuses))
	}
	for _, e := range expected {
		found := false
		for _, g := range statuses {
			if g == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing card status: %s", e)
		}
	}
}

func TestCard_CVV_GormSize3(t *testing.T) {
	rt := reflect.TypeOf(models.Card{})
	f, ok := rt.FieldByName("CVV")
	if !ok {
		t.Fatal("CVV field not found")
	}
	tag := f.Tag.Get("gorm")
	if !strings.Contains(tag, "size:3") {
		t.Errorf("CVV: expected gorm tag to contain size:3, got: %s", tag)
	}
}
