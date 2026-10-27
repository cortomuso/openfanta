package domain

import (
	"errors"
	"testing"
)

func TestPointsString(t *testing.T) {
	cases := map[Points]string{
		0: "0", 600: "6", 650: "6.5", 625: "6.25", 605: "6.05",
		-50: "-0.5", -300: "-3", -25: "-0.25", 1000: "10",
	}
	for p, want := range cases {
		if got := p.String(); got != want {
			t.Errorf("Points(%d).String() = %q, want %q", int64(p), got, want)
		}
	}
}

func TestParsePoints(t *testing.T) {
	cases := map[string]Points{
		"0": 0, "6": 600, "6.5": 650, "6.50": 650, "6.25": 625, "6.05": 605,
		"-0.5": -50, "+3": 300, "-3": -300, "10.0": 1000,
	}
	for s, want := range cases {
		got, err := ParsePoints(s)
		if err != nil || got != want {
			t.Errorf("ParsePoints(%q) = %d, %v; want %d", s, got, err, want)
		}
	}
}

func TestParsePointsInvalid(t *testing.T) {
	for _, s := range []string{"", "-", ".5", "6.", "6.123", "6,5", "abc", "1e3", "6.-5", "99999999999"} {
		if _, err := ParsePoints(s); !errors.Is(err, ErrInvalidPoints) {
			t.Errorf("ParsePoints(%q) err = %v, want ErrInvalidPoints", s, err)
		}
	}
}

func TestPointsRoundTrip(t *testing.T) {
	for v := Points(-1200); v <= 1200; v++ {
		got, err := ParsePoints(v.String())
		if err != nil || got != v {
			t.Fatalf("round trip %d -> %q -> %d, %v", int64(v), v.String(), got, err)
		}
	}
}
