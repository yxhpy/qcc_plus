package timeutil

import (
	"testing"
	"time"
)

func TestFormatBeijingTime(t *testing.T) {
	t.Run("format valid time", func(t *testing.T) {
		// Create a specific time in UTC
		utcTime := time.Date(2026, 2, 3, 12, 30, 45, 0, time.UTC)

		result := FormatBeijingTime(utcTime)

		// Beijing time should be UTC+8, so 12:30:45 UTC = 20:30:45 Beijing
		expected := "2026年02月03日 20时30分45秒"
		if result != expected {
			t.Errorf("FormatBeijingTime() = %s, want %s", result, expected)
		}
	})

	t.Run("format zero time", func(t *testing.T) {
		var zeroTime time.Time

		result := FormatBeijingTime(zeroTime)

		if result != "--" {
			t.Errorf("FormatBeijingTime(zero) = %s, want --", result)
		}
	})

	t.Run("format time already in Beijing timezone", func(t *testing.T) {
		bjTime := time.Date(2026, 2, 3, 20, 30, 45, 0, BeijingLocation)

		result := FormatBeijingTime(bjTime)

		expected := "2026年02月03日 20时30分45秒"
		if result != expected {
			t.Errorf("FormatBeijingTime() = %s, want %s", result, expected)
		}
	})
}

func TestFormatBeijingTimeShort(t *testing.T) {
	t.Run("format valid time short", func(t *testing.T) {
		utcTime := time.Date(2026, 2, 3, 12, 30, 45, 0, time.UTC)

		result := FormatBeijingTimeShort(utcTime)

		expected := "20时30分45秒"
		if result != expected {
			t.Errorf("FormatBeijingTimeShort() = %s, want %s", result, expected)
		}
	})

	t.Run("format zero time short", func(t *testing.T) {
		var zeroTime time.Time

		result := FormatBeijingTimeShort(zeroTime)

		if result != "--" {
			t.Errorf("FormatBeijingTimeShort(zero) = %s, want --", result)
		}
	})
}

func TestToBeijingTime(t *testing.T) {
	t.Run("convert UTC to Beijing time", func(t *testing.T) {
		utcTime := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)

		bjTime := ToBeijingTime(utcTime)

		// Check that the hour is 8 hours ahead
		if bjTime.Hour() != 20 {
			t.Errorf("ToBeijingTime() hour = %d, want 20", bjTime.Hour())
		}

		// Check that the location is Beijing
		if bjTime.Location() != BeijingLocation {
			t.Error("ToBeijingTime() location is not BeijingLocation")
		}
	})

	t.Run("convert already Beijing time", func(t *testing.T) {
		bjTime := time.Date(2026, 2, 3, 20, 0, 0, 0, BeijingLocation)

		result := ToBeijingTime(bjTime)

		if result.Hour() != 20 {
			t.Errorf("ToBeijingTime() hour = %d, want 20", result.Hour())
		}
	})
}

func TestNowBeijing(t *testing.T) {
	t.Run("returns current time in Beijing timezone", func(t *testing.T) {
		bjNow := NowBeijing()

		// Check that the location is Beijing
		if bjNow.Location() != BeijingLocation {
			t.Error("NowBeijing() location is not BeijingLocation")
		}

		// Check that it's close to current time (within 1 second)
		now := time.Now()
		diff := bjNow.Sub(now)
		if diff < -time.Second || diff > time.Second {
			t.Errorf("NowBeijing() time difference too large: %v", diff)
		}
	})
}

func TestParseBeijingTime(t *testing.T) {
	t.Run("parse valid time string", func(t *testing.T) {
		layout := "2006-01-02 15:04:05"
		value := "2026-02-03 20:30:45"

		parsed, err := ParseBeijingTime(layout, value)
		if err != nil {
			t.Fatalf("ParseBeijingTime() error = %v", err)
		}

		// Check that the parsed time is in Beijing location
		if parsed.Location() != BeijingLocation {
			t.Error("ParseBeijingTime() location is not BeijingLocation")
		}

		// Check the values
		if parsed.Year() != 2026 || parsed.Month() != 2 || parsed.Day() != 3 {
			t.Errorf("ParseBeijingTime() date = %v, want 2026-02-03", parsed)
		}
		if parsed.Hour() != 20 || parsed.Minute() != 30 || parsed.Second() != 45 {
			t.Errorf("ParseBeijingTime() time = %v, want 20:30:45", parsed)
		}
	})

	t.Run("parse invalid time string returns error", func(t *testing.T) {
		layout := "2006-01-02 15:04:05"
		value := "invalid-time"

		_, err := ParseBeijingTime(layout, value)
		if err == nil {
			t.Error("ParseBeijingTime() expected error for invalid input, got nil")
		}
	})

	t.Run("parse with wrong layout returns error", func(t *testing.T) {
		layout := "2006-01-02"
		value := "2026-02-03 20:30:45"

		_, err := ParseBeijingTime(layout, value)
		if err == nil {
			t.Error("ParseBeijingTime() expected error for wrong layout, got nil")
		}
	})
}

func TestBeijingLocation(t *testing.T) {
	t.Run("Beijing location is initialized", func(t *testing.T) {
		if BeijingLocation == nil {
			t.Fatal("BeijingLocation is nil")
		}
	})

	t.Run("Beijing location has correct offset", func(t *testing.T) {
		// Create a time in Beijing location
		bjTime := time.Date(2026, 2, 3, 12, 0, 0, 0, BeijingLocation)

		// Get the offset from UTC
		_, offset := bjTime.Zone()

		// Beijing is UTC+8, which is 8*3600 = 28800 seconds
		expectedOffset := 8 * 3600
		if offset != expectedOffset {
			t.Errorf("BeijingLocation offset = %d, want %d", offset, expectedOffset)
		}
	})
}
