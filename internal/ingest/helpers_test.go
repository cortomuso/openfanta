package ingest

import "github.com/openfanta/openfanta/internal/domain"

func pointsString(v int32) string { return domain.Points(v).Fixed2() }
