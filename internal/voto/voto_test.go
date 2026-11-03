package voto

import (
	"context"
	"reflect"
	"slices"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
	"github.com/openfanta/openfanta/internal/settings"
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

// match: home 2-1 away. Wins and losses move the statistical voto by 0.25.
func matchFacts() d.MatchFacts {
	r := func(p d.Points) *d.Points { return &p }
	return d.MatchFacts{
		HomeTeam: "H", AwayTeam: "A", Length: 90,
		Appearances: []d.Appearance{
			{Player: "h1", Team: "H", Role: d.RoleOutfield, Starter: true, Rating: r(700)},
			{Player: "a1", Team: "A", Role: d.RoleOutfield, Starter: true, Rating: r(551)},
			{Player: "a2", Team: "A", Role: d.RoleOutfield, Starter: true}, // no rating
			{Player: "bench", Team: "H", Role: d.RoleOutfield},             // never came on
		},
		Events: []d.Event{
			{Seq: 1, Minute: 10, Kind: d.EventGoal, Team: "H", Player: "h1"},
			{Seq: 2, Minute: 40, Kind: d.EventGoal, Team: "A", Player: "a1"},
			{Seq: 3, Minute: 70, Kind: d.EventOwnGoal, Team: "A", Player: "a2"}, // counts for home
		},
	}
}

func TestStatisticalUsesTheResult(t *testing.T) {
	got, err := NewStatistical().Votes(context.Background(), matchFacts())
	if err != nil {
		t.Fatal(err)
	}
	want := map[d.PlayerID]d.Points{"h1": 625, "a1": 575, "a2": 575}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("votes = %v, want %v (bench player never played)", got, want)
	}
}

func TestAveragedNeedsARating(t *testing.T) {
	got, err := NewAveraged().Votes(context.Background(), matchFacts())
	if err != nil {
		t.Fatal(err)
	}
	// h1: (700+625)/2 = 662.5 -> 663; a1: (551+575)/2 = 563.
	want := map[d.PlayerID]d.Points{"h1": 663, "a1": 563}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("votes = %v, want %v", got, want)
	}
}

func TestRegistryMatchesTheSettingsSchema(t *testing.T) {
	var names []string
	for _, s := range All() {
		names = append(names, s.Name())
		if s.Description() == "" {
			t.Errorf("%s has no description", s.Name())
		}
		if got, err := ByName(s.Name()); err != nil || got.Name() != s.Name() {
			t.Errorf("ByName(%s) = %v, %v", s.Name(), got, err)
		}
	}
	if !slices.Equal(names, settings.VotoSources) {
		t.Errorf("sources %v != settings.VotoSources %v", names, settings.VotoSources)
	}
	if _, err := ByName("nope"); err == nil {
		t.Error("unknown source accepted")
	}
}
