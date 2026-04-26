package service

import (
	"testing"
	"time"

	"github.com/RAF-SI-2025/EXBanka-3-Backend/exchange-service/internal/models"
)

func TestParseInt(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"0", 0},
		{"7", 7},
		{"42", 42},
		{"09", 9},
		{"abc", 0},
		{"1a2", 12},
	}
	for _, c := range cases {
		if got := parseInt(c.in); got != c.want {
			t.Errorf("parseInt(%q)=%d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseWorkingHours_Valid(t *testing.T) {
	ref := time.Date(2026, 4, 26, 10, 0, 0, 0, time.UTC)
	open, close, err := parseWorkingHours("09:30-16:00", ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if open.Hour() != 9 || open.Minute() != 30 {
		t.Errorf("open=%v, want 09:30", open)
	}
	if close.Hour() != 16 || close.Minute() != 0 {
		t.Errorf("close=%v, want 16:00", close)
	}
}

func TestParseWorkingHours_InvalidFormat(t *testing.T) {
	ref := time.Now()
	if _, _, err := parseWorkingHours("0930-1600", ref); err == nil {
		t.Error("expected error for missing colon")
	}
	if _, _, err := parseWorkingHours("09:30", ref); err == nil {
		t.Error("expected error for missing dash")
	}
	if _, _, err := parseWorkingHours("09-30-16-00", ref); err == nil {
		t.Error("expected error for too many dashes")
	}
}

func TestGetExchangeTimeStatus_ManualOpen(t *testing.T) {
	ex := models.Exchange{UseManualTime: true, ManualTimeOpen: true, WorkingHours: "09:00-17:00"}
	status := GetExchangeTimeStatus(ex)
	if !status.IsOpen || !status.ManualMode {
		t.Errorf("expected manually-open, got %+v", status)
	}
}

func TestGetExchangeTimeStatus_ManualClosed(t *testing.T) {
	ex := models.Exchange{UseManualTime: true, ManualTimeOpen: false}
	status := GetExchangeTimeStatus(ex)
	if status.IsOpen {
		t.Errorf("expected closed, got open: %+v", status)
	}
}

func TestGetExchangeTimeStatus_BadTimezone(t *testing.T) {
	ex := models.Exchange{Timezone: "Mars/Olympus"}
	status := GetExchangeTimeStatus(ex)
	if status.IsOpen {
		t.Errorf("expected closed for invalid timezone, got open")
	}
}

func TestGetExchangeTimeStatus_BadWorkingHours(t *testing.T) {
	ex := models.Exchange{Timezone: "UTC", WorkingHours: "garbage"}
	status := GetExchangeTimeStatus(ex)
	if status.IsOpen {
		t.Errorf("expected closed for bad hours, got open")
	}
}

func TestGetExchangeTimeStatus_AlwaysOpenAllDay(t *testing.T) {
	// 00:00-23:59 — at any time during a weekday, should be OPEN.
	ex := models.Exchange{Timezone: "UTC", WorkingHours: "00:00-23:59"}
	status := GetExchangeTimeStatus(ex)

	now := time.Now().UTC()
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		// On weekends the status will be CLOSED — that's expected.
		if status.IsOpen {
			t.Errorf("expected weekend closed, got open")
		}
		return
	}
	if !status.IsOpen {
		t.Errorf("expected open during all-day hours on weekday, got %+v", status)
	}
}
