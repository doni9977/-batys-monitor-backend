package parser

import (
	"testing"
	"time"
)

func TestParseServiceDate_PreservesTime(t *testing.T) {
	t.Run("excel european datetime", func(t *testing.T) {
		parsed := parseServiceDate("07.09.23 17:02", false)
		if parsed.IsZero() {
			t.Fatal("expected parsed timestamp, got zero time")
		}
		if parsed.Hour() != 17 || parsed.Minute() != 2 {
			t.Fatalf("expected time 17:02, got %02d:%02d", parsed.Hour(), parsed.Minute())
		}
	})

	t.Run("date without time still keeps midnight", func(t *testing.T) {
		parsed := parseServiceDate("07.09.2023", false)
		if parsed.IsZero() {
			t.Fatal("expected parsed date, got zero time")
		}
		if parsed.Hour() != 0 || parsed.Minute() != 0 {
			t.Fatalf("expected midnight, got %02d:%02d", parsed.Hour(), parsed.Minute())
		}
	})

	t.Run("iso datetime", func(t *testing.T) {
		parsed := parseServiceDate("2023-09-07 17:02", false)
		if parsed.IsZero() {
			t.Fatal("expected parsed timestamp, got zero time")
		}
		if parsed.Year() != 2023 || parsed.Month() != time.September || parsed.Day() != 7 {
			t.Fatalf("unexpected date: %v", parsed)
		}
	})
}
