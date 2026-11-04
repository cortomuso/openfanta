package api

import (
	"strconv"
	"testing"

	"github.com/openfanta/openfanta/internal/store/db"
)

func ids(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "r" + strconv.Itoa(i)
	}
	return out
}

func id(s string) string { return s }

func TestPaginateWalksEveryRowOnce(t *testing.T) {
	for _, n := range []int{0, 1, 4, 5, 6, 10, 11} {
		rows := ids(n)
		var got []string
		var cursor *string
		limit := 5
		for pages := 0; ; pages++ {
			if pages > n+1 {
				t.Fatalf("n=%d: paging does not end", n)
			}
			page, next, err := paginate(rows, id, &limit, cursor)
			if err != nil {
				t.Fatalf("n=%d: %v", n, err)
			}
			got = append(got, page...)
			if next == "" {
				break
			}
			cursor = &next
		}
		if len(got) != n {
			t.Fatalf("n=%d: got %d rows", n, len(got))
		}
		for i := range got {
			if got[i] != rows[i] {
				t.Fatalf("n=%d: row %d is %s", n, i, got[i])
			}
		}
	}
}

func TestPaginateExactMultipleHasNoEmptyLastPage(t *testing.T) {
	limit := 5
	page, next, err := paginate(ids(10), id, &limit, nil)
	if err != nil || len(page) != 5 || next == "" {
		t.Fatalf("first page: %v %v %q", page, err, next)
	}
	page, next, err = paginate(ids(10), id, &limit, &next)
	if err != nil || len(page) != 5 || next != "" {
		t.Fatalf("second page: %v %v %q", page, err, next)
	}
}

func TestPaginateWithoutParamsReturnsAll(t *testing.T) {
	page, next, err := paginate(ids(300), id, nil, nil)
	if err != nil || len(page) != 300 || next != "" {
		t.Fatalf("%d rows, %v, %q", len(page), err, next)
	}
}

func TestPaginateRejectsBadInput(t *testing.T) {
	bad := []string{"garbage", "v1.", "v1.!!!", encodeCursor("not-in-list")}
	for _, c := range bad {
		if _, _, err := paginate(ids(3), id, nil, &c); err == nil {
			t.Errorf("cursor %q accepted", c)
		}
	}
	for _, l := range []int{0, -1, MaxLimit + 1} {
		if _, _, err := paginate(ids(3), id, &l, nil); err == nil {
			t.Errorf("limit %d accepted", l)
		}
	}
}

func TestPlayerFromClubHistory(t *testing.T) {
	p := db.Player{Name: "Mover"}
	d, c := "D", "C"
	// Latest season first: the profile follows it, the history keeps both.
	got := player(p, []db.PlayerClubHistoryRow{
		{SeasonName: "2026/2027", ClubName: "New FC", Role: &c, Listed: false},
		{SeasonName: "2025/2026", ClubName: "Old FC", Role: &d, Listed: true},
	})
	if got.Club == nil || got.Club.Name != "New FC" || got.Status != NoLongerListed || len(got.Roles) != 1 || got.Roles[0] != C {
		t.Errorf("profile = %+v", got)
	}
	if len(got.ClubHistory) != 2 || got.ClubHistory[1].Club.Name != "Old FC" || got.ClubHistory[1].Status != Listed || got.ClubHistory[1].Roles[0] != D {
		t.Errorf("history = %+v", got.ClubHistory)
	}
	if nobody := player(p, nil); nobody.Status != Unknown || nobody.Club != nil || nobody.Roles == nil || nobody.ClubHistory == nil {
		t.Errorf("no squad entry = %+v", nobody)
	}
}

func TestSetResultOmitsWhatTheProviderDidNotGive(t *testing.T) {
	var none Fixture
	setResult(&none, nil, nil, "")
	if none.Score != nil || none.Period != nil {
		t.Errorf("empty result = %+v", none)
	}
	one := int16(1)
	var half Fixture
	setResult(&half, &one, nil, "")
	if half.Score != nil {
		t.Errorf("a score of one side only = %+v", half.Score)
	}
	var live Fixture
	setResult(&live, &one, &one, "half_time")
	if live.Score == nil || live.Score.Home != 1 || live.Period == nil || *live.Period != "half_time" {
		t.Errorf("live = %+v", live)
	}
}
