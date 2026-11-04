package api

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// Pagination of list endpoints: `limit` and an opaque `cursor`, answered with `next_cursor`.
//
// A cursor names the last row of a page by a key that is unique among the rows of the list and
// follows their order. The next page starts right after that row, so rows added or removed
// elsewhere never shift a page. A request with neither parameter returns the whole list, which
// keeps clients written before pagination working.

const (
	// MaxLimit is the largest page a client may ask for.
	MaxLimit = 200

	cursorPrefix = "v1."
)

var errBadCursor = errors.New("cursor is not valid for this list")

// encodeCursor wraps a row key into an opaque cursor.
func encodeCursor(key string) string {
	return cursorPrefix + base64.RawURLEncoding.EncodeToString([]byte(key))
}

func decodeCursor(c string) (string, error) {
	raw, ok := strings.CutPrefix(c, cursorPrefix)
	if !ok {
		return "", errBadCursor
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(b) == 0 {
		return "", errBadCursor
	}
	return string(b), nil
}

// paginate returns the page of rows selected by limit and cursor, and the cursor of the next
// page ("" on the last one). rows must already be in list order and key must be unique.
// A nil limit and cursor select every row.
func paginate[T any](rows []T, key func(T) string, limit *int, cursor *string) (page []T, next string, err error) {
	if limit != nil && (*limit < 1 || *limit > MaxLimit) {
		return nil, "", fmt.Errorf("limit must be between 1 and %d", MaxLimit)
	}
	start := 0
	if cursor != nil && *cursor != "" {
		k, err := decodeCursor(*cursor)
		if err != nil {
			return nil, "", err
		}
		start = -1
		for i, r := range rows {
			if key(r) == k {
				start = i + 1
				break
			}
		}
		if start < 0 {
			return nil, "", errBadCursor
		}
	}
	rows = rows[start:]
	if limit == nil || len(rows) <= *limit {
		return rows, "", nil
	}
	page = rows[:*limit]
	return page, encodeCursor(key(page[len(page)-1])), nil
}
