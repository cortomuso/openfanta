package settings

import (
	"strings"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
)

func TestRoundingApply(t *testing.T) {
	cases := []struct {
		mode string
		step d.Points
		in   d.Points
		want d.Points
	}{
		{RoundNone, 0, 638, 638},
		{"", 0, 638, 638},
		{RoundNearest, 100, 649, 600},
		{RoundNearest, 100, 650, 700}, // halves go up
		{RoundNearest, 100, 550, 600},
		{RoundNearest, 50, 624, 600},
		{RoundNearest, 50, 625, 650},
		{RoundNearest, 50, 650, 650},
		{RoundNearest, 100, -50, -100}, // halves go away from zero
		{RoundUp, 100, 601, 700},
		{RoundUp, 100, 600, 600},
		{RoundUp, 50, 651, 700},
		{RoundDown, 100, 699, 600},
		{RoundDown, 50, 649, 600},
		{RoundDown, 100, -1, -100},
		{RoundUp, 100, -1, 0},
	}
	for _, c := range cases {
		if got := (Rounding{Mode: c.mode, Step: c.step}).Apply(c.in); got != c.want {
			t.Errorf("%s/%d of %d = %d, want %d", c.mode, c.step, c.in, got, c.want)
		}
	}
}

func TestVotoSection(t *testing.T) {
	doc := func(schema, voto string) []byte {
		s := strings.Replace(string(classicJSON), `"openfanta.settings/1.0"`, `"`+schema+`"`, 1)
		return []byte(strings.Replace(s, `"game"`, `"voto": `+voto+`, "game"`, 1))
	}
	good := `{"source":"averaged","rounding":{"mode":"nearest","step":50}}`
	s, err := Parse(doc("openfanta.settings/1.2", good))
	if err != nil {
		t.Fatal(err)
	}
	if v := s.Doc.Voto; v.SourceName() != SourceAveraged || v.Rounding.Mode != RoundNearest || v.Rounding.Step != 50 {
		t.Errorf("voto = %+v", v)
	}
	if got := Classic().Doc.Voto.SourceName(); got != SourceProviderRating {
		t.Errorf("classic source = %s", got)
	}

	bad := map[string][]byte{
		"needs schema 1.2":    doc("openfanta.settings/1.1", good),
		"unknown source":      doc("openfanta.settings/1.2", `{"source":"astrology"}`),
		"unknown mode":        doc("openfanta.settings/1.2", `{"rounding":{"mode":"banker","step":50}}`),
		"mode without step":   doc("openfanta.settings/1.2", `{"rounding":{"mode":"nearest"}}`),
		"step without mode":   doc("openfanta.settings/1.2", `{"rounding":{"step":50}}`),
		"none with a step":    doc("openfanta.settings/1.2", `{"rounding":{"mode":"none","step":50}}`),
		"fractional step":     doc("openfanta.settings/1.2", `{"rounding":{"mode":"up","step":0.5}}`),
		"step beyond a point": doc("openfanta.settings/1.2", `{"rounding":{"mode":"up","step":250}}`),
	}
	for name, raw := range bad {
		if _, err := Parse(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
