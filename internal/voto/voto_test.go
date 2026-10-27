package voto

import (
	"context"
	"reflect"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
)

func TestProviderRatingIsUnrounded(t *testing.T) {
	r := func(p d.Points) *d.Points { return &p }
	facts := d.MatchFacts{Appearances: []d.Appearance{
		{Player: "a", Rating: r(638)},
		{Player: "b", Rating: r(210)},
		{Player: "c", Rating: r(1000)},
		{Player: "d"},
		{Player: "e", Rating: r(625)},
	}}
	src := NewProviderRating()
	got, err := src.Votes(context.Background(), facts)
	if err != nil {
		t.Fatal(err)
	}
	want := map[d.PlayerID]d.Points{"a": 638, "b": 210, "c": 1000, "e": 625}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("votes = %v, want %v", got, want)
	}
	if src.Name() != "provider-rating" {
		t.Errorf("name = %s", src.Name())
	}
}
