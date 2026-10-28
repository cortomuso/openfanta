package settings

import (
	"reflect"
	"testing"
)

func TestDiff(t *testing.T) {
	prev := `{"a":1,"b":{"c":2,"d":[1,2]},"gone":true,"p/q":1}`
	next := `{"a":1,"b":{"c":3,"d":[1,2,3]},"new":{"k":1},"p/q":2}`
	got, err := Diff([]byte(prev), []byte(next))
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{
		{Path: "/b/c", Op: OpChange, Old: []byte(`2`), New: []byte(`3`)},
		{Path: "/b/d", Op: OpChange, Old: []byte(`[1,2]`), New: []byte(`[1,2,3]`)},
		{Path: "/gone", Op: OpRemove, Old: []byte(`true`)},
		{Path: "/new", Op: OpAdd, New: []byte(`{"k":1}`)},
		{Path: "/p~1q", Op: OpChange, Old: []byte(`1`), New: []byte(`2`)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestDiffIdenticalIsEmpty(t *testing.T) {
	got, err := Diff(classicJSON, classicJSON)
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestDiffTypeChange(t *testing.T) {
	got, err := Diff([]byte(`{"a":{"x":1}}`), []byte(`{"a":5}`))
	if err != nil || len(got) != 1 || got[0].Op != OpChange || got[0].Path != "/a" {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestDiffRejectsBadInput(t *testing.T) {
	if _, err := Diff([]byte(`{`), []byte(`{}`)); err == nil {
		t.Error("bad previous document accepted")
	}
	if _, err := Diff([]byte(`{}`), []byte(`{"a":1.5}`)); err == nil {
		t.Error("bad next document accepted")
	}
}
