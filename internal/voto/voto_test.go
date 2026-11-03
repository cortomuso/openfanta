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

func TestAverageNeedsEveryChild(t *testing.T) {
	avg, err := NewAverage(NewProviderRating(), NewStatistical())
	if err != nil {
		t.Fatal(err)
	}
	got, err := avg.Votes(context.Background(), matchFacts())
	if err != nil {
		t.Fatal(err)
	}
	// h1: (700+625)/2 = 662.5 -> 663; a1: (551+575)/2 = 563; a2 has no rating.
	want := map[d.PlayerID]d.Points{"h1": 663, "a1": 563}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("votes = %v, want %v", got, want)
	}
	if v := avg.Version(); v != "provider-rating@1+openfanta-stat@placeholder;min=all" {
		t.Errorf("version = %s", v)
	}
}

func TestDivRoundIsHalfAwayFromZero(t *testing.T) {
	for _, c := range []struct{ a, b, want d.Points }{
		{1325, 2, 663}, {1324, 2, 662}, {-1325, 2, -663}, {5, 2, 3}, {-5, 2, -3}, {4, 2, 2}, {10, 3, 3}, {11, 3, 4},
	} {
		if got := divRound(c.a, c.b); got != c.want {
			t.Errorf("%d/%d = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestAverageRejectsBadChildren(t *testing.T) {
	if _, err := NewAverage(NewProviderRating()); err == nil {
		t.Error("one child accepted")
	}
	avg, _ := NewAverage(NewProviderRating(), NewStatistical())
	if _, err := NewAverage(NewProviderRating(), avg); err == nil {
		t.Error("average inside an average accepted")
	}
}

func TestBuild(t *testing.T) {
	var names []string
	for _, s := range Default() {
		names = append(names, s.Name())
		if s.Description() == "" || s.Version() == "" {
			t.Errorf("%s lacks description or version", s.Name())
		}
	}
	if !slices.Equal(names, settings.VotoSources) {
		t.Errorf("default sources %v != settings.VotoSources %v", names, settings.VotoSources)
	}

	only, err := Build([]string{"provider-rating"}, nil)
	if err != nil || len(only) != 1 {
		t.Fatalf("Build one = %v, %v", only, err)
	}
	for name, c := range map[string]struct{ names, of []string }{
		"unknown source":    {[]string{"astrology"}, nil},
		"duplicate":         {[]string{"provider-rating", "provider-rating"}, nil},
		"average of itself": {[]string{"average"}, []string{"provider-rating", "average"}},
		"unknown child":     {[]string{"average"}, []string{"provider-rating", "astrology"}},
		"single child":      {[]string{"average"}, []string{"provider-rating"}},
	} {
		if _, err := Build(c.names, c.of); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
