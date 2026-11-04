package api

import (
	"strconv"
	"testing"
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
