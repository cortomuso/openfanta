package voto

import (
	"context"
	"reflect"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
)

func TestRoundHalf(t *testing.T) {
	cases := map[d.Points]d.Points{
		600: 600, 624: 600, 625: 650, 674: 650, 675: 700, 699: 700, 0: 0, 12: 0, 13: 0, 25: 50, -25: -50, -24: 0,
	}
	for in, want := range cases {
		if got := RoundHalf(in); got != want {
			t.Errorf("RoundHalf(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestProviderRating(t *testing.T) {
	r := func(p d.Points) *d.Points { return &p }
	facts := d.MatchFacts{Appearances: []d.Appearance{
		{Player: "a", Rating: r(723)},
		{Player: "b", Rating: r(210)},
		{Player: "c", Rating: r(1000)},
		{Player: "d"},
		{Player: "e", Rating: r(624)},
	}}
	src := NewProviderRating()
	got, err := src.Votes(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[d.PlayerID]d.Points{"a": 700, "b": 300, "c": 1000, "e": 600}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("votes = %v, want %v", got, want)
	}
	if src.Name() != "provider-rating" {
		t.Errorf("name = %s", src.Name())
	}
}
