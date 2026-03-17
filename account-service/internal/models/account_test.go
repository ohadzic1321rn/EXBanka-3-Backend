package models_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/account-service/internal/models"
)

func TestAccount_GormTags(t *testing.T) {
	rt := reflect.TypeOf(models.Account{})

	f, ok := rt.FieldByName("BrojRacuna")
	if !ok {
		t.Fatal("BrojRacuna field not found on Account")
	}
	tag := f.Tag.Get("gorm")
	if !strings.Contains(tag, "uniqueIndex") {
		t.Errorf("BrojRacuna: expected gorm tag to contain uniqueIndex, got: %s", tag)
	}
	if !strings.Contains(tag, "size:18") {
		t.Errorf("BrojRacuna: expected gorm tag to contain size:18, got: %s", tag)
	}
	if !strings.Contains(tag, "not null") {
		t.Errorf("BrojRacuna: expected gorm tag to contain not null, got: %s", tag)
	}

	s, ok := rt.FieldByName("Status")
	if !ok {
		t.Fatal("Status field not found on Account")
	}
	stag := s.Tag.Get("gorm")
	if !strings.Contains(stag, "aktivan") {
		t.Errorf("Status: expected gorm default:'aktivan', got: %s", stag)
	}
}

func TestAccount_TipValues(t *testing.T) {
	tekuci := models.Account{Tip: "tekuci"}
	devizni := models.Account{Tip: "devizni"}
	if tekuci.Tip != "tekuci" {
		t.Errorf("expected tekuci, got %s", tekuci.Tip)
	}
	if devizni.Tip != "devizni" {
		t.Errorf("expected devizni, got %s", devizni.Tip)
	}
}

func TestAccount_VrstaValues(t *testing.T) {
	licni := models.Account{Vrsta: "licni"}
	poslovni := models.Account{Vrsta: "poslovni"}
	if licni.Vrsta != "licni" {
		t.Errorf("expected licni, got %s", licni.Vrsta)
	}
	if poslovni.Vrsta != "poslovni" {
		t.Errorf("expected poslovni, got %s", poslovni.Vrsta)
	}
}
