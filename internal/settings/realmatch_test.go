package settings

import (
	"strings"
	"testing"

	d "github.com/openfanta/openfanta/internal/domain"
)

func TestRealMatchSection(t *testing.T) {
	doc := func(schema, section string) []byte {
		s := strings.Replace(string(classicJSON), `"openfanta.settings/1.0"`, `"`+schema+`"`, 1)
		return []byte(strings.Replace(s, `"game"`, `"real_match": `+section+`, "game"`, 1))
	}
	good := `{"on_postponed":"automatic_voto","on_suspended":"wait","automatic_voto":600}`
	s, err := Parse(doc("openfanta.settings/1.3", good))
	if err != nil {
		t.Fatal(err)
	}
	rm := s.Doc.RealMatch
	if rm.PolicyFor(d.ExceptionPostponed) != PolicyAutomaticVoto || rm.PolicyFor(d.ExceptionSuspended) != PolicyWait || *rm.AutomaticVoto != 600 {
		t.Errorf("real_match = %+v", rm)
	}
	// Classic waits for the real voti, and a normal match has no policy to look up.
	classic := Classic().Doc.RealMatch
	if classic.PolicyFor(d.ExceptionPostponed) != PolicyWait || classic.PolicyFor(d.ExceptionSuspended) != PolicyWait || classic.PolicyFor(d.ExceptionNone) != PolicyWait {
		t.Errorf("classic policy = %+v", classic)
	}

	// A value without a policy is fine: an admin override may use it.
	if _, err := Parse(doc("openfanta.settings/1.3", `{"automatic_voto":600}`)); err != nil {
		t.Errorf("value only: %v", err)
	}
	bad := map[string][]byte{
		"needs schema 1.3":       doc("openfanta.settings/1.2", good),
		"automatic needs a voto": doc("openfanta.settings/1.3", `{"on_postponed":"automatic_voto"}`),
		"unknown policy":         doc("openfanta.settings/1.3", `{"on_postponed":"shrug"}`),
		"voto above 10":          doc("openfanta.settings/1.3", `{"on_suspended":"automatic_voto","automatic_voto":1100}`),
		"fractional voto":        doc("openfanta.settings/1.3", `{"automatic_voto":6.5}`),
		"unknown key":            doc("openfanta.settings/1.3", `{"on_cancelled":"wait"}`),
	}
	for name, raw := range bad {
		if _, err := Parse(raw); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
