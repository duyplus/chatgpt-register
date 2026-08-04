package models

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"
)

// LocalTime formats timestamps as "YYYY-MM-DD HH:MM:SS.mmm" (e.g. "2025-02-27 22:10:47.410")
type LocalTime time.Time

const ShortTimeFormat = "2006-01-02 15:04:05.000"
const ShortTimeSecFormat = "2006-01-02 15:04:05"

func (t LocalTime) MarshalJSON() ([]byte, error) {
	tTime := time.Time(t)
	if tTime.IsZero() {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%q", tTime.Format(ShortTimeFormat))), nil
}

func (t *LocalTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*t = LocalTime(time.Time{})
		return nil
	}
	parsed, err := ParseShortTime(s)
	if err != nil {
		return err
	}
	*t = LocalTime(parsed)
	return nil
}

func (t LocalTime) Value() (driver.Value, error) {
	tTime := time.Time(t)
	if tTime.IsZero() {
		return nil, nil
	}
	return tTime.Format(ShortTimeFormat), nil
}

func (t *LocalTime) Scan(v interface{}) error {
	if v == nil {
		*t = LocalTime(time.Time{})
		return nil
	}
	switch vt := v.(type) {
	case time.Time:
		*t = LocalTime(vt)
		return nil
	case string:
		parsed, err := ParseShortTime(vt)
		if err != nil {
			*t = LocalTime(time.Time{})
			return nil
		}
		*t = LocalTime(parsed)
		return nil
	case []byte:
		parsed, err := ParseShortTime(string(vt))
		if err != nil {
			*t = LocalTime(time.Time{})
			return nil
		}
		*t = LocalTime(parsed)
		return nil
	default:
		return fmt.Errorf("cannot scan %T into LocalTime", v)
	}
}

func (t LocalTime) String() string {
	tTime := time.Time(t)
	if tTime.IsZero() {
		return ""
	}
	return tTime.Format(ShortTimeFormat)
}

func (t LocalTime) Time() time.Time {
	return time.Time(t)
}

func NowLocal() LocalTime {
	return LocalTime(time.Now())
}

// ParseShortTime parses various time string formats into time.Time
func ParseShortTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "null" {
		return time.Time{}, nil
	}

	sWithT := strings.Replace(s, " ", "T", 1)
	sWithSpace := strings.Replace(s, "T", " ", 1)

	layouts := []string{
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999999",
		ShortTimeFormat,
		ShortTimeSecFormat,
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05Z07:00",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999999999Z07:00",
		"2006-01-02T15:04:05",
	}

	for _, strToTry := range []string{s, sWithT, sWithSpace} {
		for _, l := range layouts {
			if t, err := time.ParseInLocation(l, strToTry, time.Local); err == nil {
				return t, nil
			}
			if t, err := time.Parse(l, strToTry); err == nil {
				return t, nil
			}
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}

// FormatShortTime formats an ISO string or time string into "2006-01-02 15:04:05.000"
func FormatShortTime(s string) string {
	if s == "" {
		return ""
	}
	parsed, err := ParseShortTime(s)
	if err != nil {
		return s
	}
	return parsed.Format(ShortTimeFormat)
}
