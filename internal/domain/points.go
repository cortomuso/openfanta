// Package domain holds provider-agnostic value types shared across OpenFanta.
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Points is a fantacalcio score in hundredths: 650 is 6.5, -50 is -0.5.
//
// Integer fixed point keeps every computation exact and deterministic across
// platforms and implementations; floats are never used for scores. In JSON and
// YAML it is a plain integer of hundredths; the public API formats it with String.
type Points int64

// Scale is the number of Points in one whole unit.
const Scale = 100

// String formats p as a minimal decimal: 600 -> "6", 650 -> "6.5", 625 -> "6.25".
func (p Points) String() string {
	sign := ""
	v := int64(p)
	if v < 0 {
		sign = "-"
		v = -v
	}
	whole, frac := v/Scale, v%Scale
	switch {
	case frac == 0:
		return fmt.Sprintf("%s%d", sign, whole)
	case frac%10 == 0:
		return fmt.Sprintf("%s%d.%d", sign, whole, frac/10)
	default:
		return fmt.Sprintf("%s%d.%02d", sign, whole, frac)
	}
}

// ErrInvalidPoints is returned by ParsePoints for malformed input.
var ErrInvalidPoints = errors.New("invalid points")

// ParsePoints parses a decimal with at most two fractional digits, e.g. "6.5" or "-0.25".
func ParsePoints(s string) (Points, error) {
	in := s
	neg := false
	switch {
	case strings.HasPrefix(s, "-"):
		neg, s = true, s[1:]
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}
	wholeStr, fracStr, hasDot := strings.Cut(s, ".")
	if wholeStr == "" || (hasDot && (fracStr == "" || len(fracStr) > 2)) {
		return 0, fmt.Errorf("%w: %q", ErrInvalidPoints, in)
	}
	if !digitsOnly(wholeStr) || !digitsOnly(fracStr) {
		return 0, fmt.Errorf("%w: %q", ErrInvalidPoints, in)
	}
	whole, err := strconv.ParseInt(wholeStr, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", ErrInvalidPoints, in)
	}
	var frac int64
	if fracStr != "" {
		frac, _ = strconv.ParseInt(fracStr, 10, 64)
		if len(fracStr) == 1 {
			frac *= 10
		}
	}
	v := whole*Scale + frac
	if neg {
		v = -v
	}
	return Points(v), nil
}

func digitsOnly(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
