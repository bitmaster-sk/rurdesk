package urlutil

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var daysComponent = regexp.MustCompile(`([0-9]*\.?[0-9]+)d`)

// ParsePositiveDuration parses the time.ParseDuration grammar plus a `d` unit
// (exactly 24h), so "2h", "30d" and "1d8h6m" all work. Zero and negative are errors.
func ParsePositiveDuration(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, fmt.Errorf("empty duration")
	}

	expanded, err := expandDays(trimmed)
	if err != nil {
		return 0, err
	}

	parsed, err := time.ParseDuration(expanded)
	if err != nil {
		return 0, fmt.Errorf("parsing duration %q: %w", raw, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", raw)
	}
	return parsed, nil
}

// expandDays rewrites `<number>d` as hours so time.ParseDuration can take it.
func expandDays(raw string) (string, error) {
	var conversionErr error
	expanded := daysComponent.ReplaceAllStringFunc(raw, func(match string) string {
		days, err := strconv.ParseFloat(strings.TrimSuffix(match, "d"), 64)
		if err != nil {
			conversionErr = fmt.Errorf("parsing days in %q: %w", raw, err)
			return match
		}
		return strconv.FormatFloat(days*24, 'f', -1, 64) + "h"
	})
	if conversionErr != nil {
		return "", conversionErr
	}
	return expanded, nil
}
