package main

import (
	"testing"
	"time"
)

func loadTbilisi(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Tbilisi")
	if err != nil {
		t.Fatalf("failed to load timezone: %v", err)
	}
	return loc
}

func TestParseDue_DateOnly(t *testing.T) {
	loc := loadTbilisi(t)
	result, err := parseDue("2026-03-15", loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Midnight in Tbilisi = 2026-03-14T20:00:00Z
	expected := "2026-03-15T00:00:00+04:00"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestParseDue_DateOnly_UTC(t *testing.T) {
	result, err := parseDue("2026-03-15", time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "2026-03-15T00:00:00Z"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestParseDue_DateTime(t *testing.T) {
	loc := loadTbilisi(t)
	result, err := parseDue("2026-03-15T14:30", loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "2026-03-15T14:30:00+04:00"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestParseDue_Invalid(t *testing.T) {
	_, err := parseDue("not-a-date", time.UTC)
	if err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestParseDue_NilLocation(t *testing.T) {
	result, err := parseDue("2026-03-15", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "2026-03-15T00:00:00Z"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatDue_DateOnly(t *testing.T) {
	result := formatDue("2026-03-15T00:00:00Z", time.UTC)
	if result != "2026-03-15" {
		t.Errorf("expected 2026-03-15, got %q", result)
	}
}

func TestFormatDue_WithTime(t *testing.T) {
	loc := loadTbilisi(t)
	result := formatDue("2026-03-15T10:30:00+04:00", loc)
	if result != "2026-03-15 10:30" {
		t.Errorf("expected '2026-03-15 10:30', got %q", result)
	}
}

func TestFormatDue_FractionalSeconds(t *testing.T) {
	result := formatDue("2026-03-15T00:00:00.000Z", time.UTC)
	if result != "2026-03-15" {
		t.Errorf("expected 2026-03-15, got %q", result)
	}
}

func TestFormatDue_MidnightInTimezone(t *testing.T) {
	loc := loadTbilisi(t)
	// Midnight UTC is 04:00 in Tbilisi — should show time
	result := formatDue("2026-03-15T00:00:00Z", loc)
	if result != "2026-03-15 04:00" {
		t.Errorf("expected '2026-03-15 04:00', got %q", result)
	}
}

func TestFormatDue_NilLocation(t *testing.T) {
	result := formatDue("2026-03-15T00:00:00Z", nil)
	if result != "2026-03-15" {
		t.Errorf("expected 2026-03-15, got %q", result)
	}
}

func TestFormatDue_InvalidFallback(t *testing.T) {
	result := formatDue("garbage", time.UTC)
	if result != "garbage" {
		t.Errorf("expected fallback to raw string, got %q", result)
	}
}
