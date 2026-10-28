package settings

import (
	"errors"
	"strings"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
)

// with returns the Classic document with one top-level key replaced.
func with(t *testing.T, key, valueJSON string) []byte {
	t.Helper()
	s := string(classicJSON)
	switch key {
	case "extra": // add a top-level key
		s = strings.Replace(s, `"game"`, valueJSON+`, "game"`, 1)
	default:
		t.Fatalf("unknown key %q", key)
	}
	return []byte(s)
}

func TestClassicParses(t *testing.T) {
	s := Classic()
	if s.SHA256 == "" || len(s.Canonical) == 0 {
		t.Fatal("classic has no hash or canonical form")
	}
	p := s.Doc.Player
	cases := []struct {
		name string
		got  d.Points
		want d.Points
	}{
		{"goal", p.Points(EventGoal, d.RoleOutfield), 300},
		{"penalty goal follows goal", p.PenaltyGoalPoints(d.RoleOutfield), 300},
		{"assist", p.Points(EventAssist, d.RoleOutfield), 100},
		{"yellow", p.Points(EventYellow, d.RoleOutfield), -50},
		{"red", p.Points(EventRed, d.RoleOutfield), -100},
		{"own goal", p.Points(EventOwnGoal, d.RoleOutfield), -200},
		{"conceded keeper", p.Points(EventGoalConceded, d.RoleGoalkeeper), -100},
		{"conceded outfield", p.Points(EventGoalConceded, d.RoleOutfield), 0},
		{"clean sheet keeper", p.Points(EventCleanSheet, d.RoleGoalkeeper), 100},
		{"clean sheet outfield", p.Points(EventCleanSheet, d.RoleOutfield), 0},
		{"penalty saved", p.Points(EventPenaltySaved, d.RoleOutfield), 300},
		{"penalty missed", p.Points(EventPenaltyMiss, d.RoleOutfield), -300},
		{"missing event", p.Points("nope", d.RoleOutfield), 0},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, c.got, c.want)
		}
	}
	if !p.Cards.RedReplacesYellow {
		t.Error("red_replaces_yellow should be on")
	}
	if got := p.SenzaVoto.MinPlayed(); got != 15 {
		t.Errorf("senza voto min minutes = %d, want 15", got)
	}
	if got := p.CleanSheetMinMinutes(); got != 0 {
		t.Errorf("clean sheet min minutes = %d, want 0", got)
	}
	if again := Classic(); again.SHA256 != s.SHA256 {
		t.Error("hash is not stable")
	}
}

func TestParseKeepsOpaqueKeysVerbatim(t *testing.T) {
	raw := with(t, "extra", `"x-client": {"theme": "verde", "n": 1}`)
	s, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(s.Canonical), `"x-client":{"n":1,"theme":"verde"}`) {
		t.Errorf("x-client not preserved: %s", s.Canonical)
	}
	if !strings.Contains(string(s.Canonical), `"meta":{"name":"Classic"}`) {
		t.Errorf("meta not preserved: %s", s.Canonical)
	}
	// Opaque data changes the hash: history must record it.
	if s.SHA256 == Classic().SHA256 {
		t.Error("x-client did not change the hash")
	}
}

func TestParseRejects(t *testing.T) {
	cases := []struct {
		name string
		edit func(string) string
	}{
		{"unknown top-level key", func(s string) string { return strings.Replace(s, `"game"`, `"bogus": 1, "game"`, 1) }},
		{"unknown key in scoring module", func(s string) string { return strings.Replace(s, `"goal": 300`, `"goal": 300, "gol": 1`, 1) }},
		{"unknown key in event object", func(s string) string {
			return strings.Replace(s, `"mode": "same_as_goal"`, `"mode": "same_as_goal", "x": 1`, 1)
		}},
		{"float points", func(s string) string { return strings.Replace(s, `"goal": 300`, `"goal": 3.5`, 1) }},
		{"exponent points", func(s string) string { return strings.Replace(s, `"goal": 300`, `"goal": 3e2`, 1) }},
		{"points out of range", func(s string) string { return strings.Replace(s, `"goal": 300`, `"goal": 99999`, 1) }},
		{"duplicate key", func(s string) string {
			return strings.Replace(s, `"game": "classic"`, `"game": "classic", "game": "classic"`, 1)
		}},
		{"unsupported engine", func(s string) string { return strings.Replace(s, `"2026.10"`, `"2031.01"`, 1) }},
		{"schema major 2", func(s string) string { return strings.Replace(s, `settings/1.0`, `settings/2.0`, 1) }},
		{"unsupported requires", func(s string) string { return strings.Replace(s, `["scoring.penalty_mode"]`, `["scoring.nope"]`, 1) }},
		{"mode on wrong event", func(s string) string {
			return strings.Replace(s, `"goal": 300`, `"goal": {"value": 300, "mode": "add_to_goal"}`, 1)
		}},
		{"min_minutes on wrong event", func(s string) string {
			return strings.Replace(s, `"goal": 300`, `"goal": {"value": 300, "min_minutes": 5}`, 1)
		}},
		{"replace_goal without value", func(s string) string {
			return strings.Replace(s, `{ "mode": "same_as_goal" }`, `{ "mode": "replace_goal" }`, 1)
		}},
		{"same_as_goal with value", func(s string) string {
			return strings.Replace(s, `{ "mode": "same_as_goal" }`, `{ "mode": "same_as_goal", "value": 5 }`, 1)
		}},
		{"empty event object", func(s string) string { return strings.Replace(s, `"goal": 300`, `"goal": {}`, 1) }},
		{"missing player", func(s string) string { return strings.Replace(s, `"player"`, `"x-player"`, 1) }},
		{"trailing data", func(s string) string { return s + ` {}` }},
		{"not json", func(string) string { return `nope` }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse([]byte(c.edit(string(classicJSON))))
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("got %v, want ErrInvalid", err)
			}
		})
	}
}

func TestPenaltyModes(t *testing.T) {
	mk := func(penalty string) Player {
		t.Helper()
		raw := strings.Replace(string(classicJSON), `{ "mode": "same_as_goal" }`, penalty, 1)
		s, err := Parse([]byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		return s.Doc.Player
	}
	cases := []struct {
		name    string
		penalty string
		want    d.Points
	}{
		{"same as goal", `{ "mode": "same_as_goal" }`, 300},
		{"replace goal", `{ "mode": "replace_goal", "value": 250 }`, 250},
		{"add to goal", `{ "mode": "add_to_goal", "value": 50 }`, 350},
		{"plain value replaces", `275`, 275},
	}
	for _, c := range cases {
		if got := mk(c.penalty).PenaltyGoalPoints(d.RoleOutfield); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	// No penalty_goal entry at all: a penalty is worth a goal.
	p := Player{Events: map[string]Event{EventGoal: {Value: ptr(d.Points(300))}}}
	if got := p.PenaltyGoalPoints(d.RoleOutfield); got != 300 {
		t.Errorf("absent penalty_goal: got %d, want 300", got)
	}
}

func ptr[T any](v T) *T { return &v }

func TestByRoleOverridesValue(t *testing.T) {
	p := Player{Events: map[string]Event{
		EventGoal: {Value: ptr(d.Points(300)), ByRole: map[string]d.Points{"P": 900}},
	}}
	if got := p.Points(EventGoal, d.RoleGoalkeeper); got != 900 {
		t.Errorf("keeper: got %d, want 900", got)
	}
	if got := p.Points(EventGoal, d.RoleOutfield); got != 300 {
		t.Errorf("outfield: got %d, want 300", got)
	}
}

func TestMinutesDefaultsAndOverrides(t *testing.T) {
	var p Player
	if p.CleanSheetMinMinutes() != 0 || p.SenzaVoto.MinPlayed() != 0 {
		t.Error("unset minutes should be 0")
	}
	raw := strings.Replace(string(classicJSON), `"min_minutes": null`, `"min_minutes": 30`, 1)
	raw = strings.Replace(raw, `"min_minutes": 15`, `"min_minutes": null`, 1)
	s, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Doc.Player.CleanSheetMinMinutes(); got != 30 {
		t.Errorf("clean sheet min minutes = %d, want 30", got)
	}
	if got := s.Doc.Player.SenzaVoto.MinPlayed(); got != 0 {
		t.Errorf("null senza voto min minutes = %d, want 0", got)
	}
}

func TestEventUnmarshal(t *testing.T) {
	var e Event
	if err := e.UnmarshalJSON([]byte(`"x"`)); err == nil {
		t.Error("string accepted as an integer shorthand")
	}
	if err := e.UnmarshalJSON([]byte(`{"nope":1}`)); err == nil {
		t.Error("unknown field accepted")
	}
	if err := e.UnmarshalJSON([]byte(` 7 `)); err != nil || e.Value == nil || *e.Value != 7 {
		t.Errorf("shorthand: %+v, %v", e, err)
	}
}

func TestParseRejectsNonObject(t *testing.T) {
	if _, err := Parse([]byte(`[]`)); !errors.Is(err, ErrInvalid) {
		t.Errorf("got %v, want ErrInvalid", err)
	}
}
