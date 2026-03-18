// Package dbutil provides conversion helpers between Go types and pgtype types used by SQLC.
package dbutil

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// Timestamptz creates a valid pgtype.Timestamptz from a time.Time.
func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// TimestamptzPtr creates a pgtype.Timestamptz from a *time.Time (null if nil).
func TimestamptzPtr(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

// NullTimestamptz returns an invalid (NULL) pgtype.Timestamptz.
func NullTimestamptz() pgtype.Timestamptz {
	return pgtype.Timestamptz{Valid: false}
}

// Int4 creates a valid pgtype.Int4.
func Int4(v int32) pgtype.Int4 {
	return pgtype.Int4{Int32: v, Valid: true}
}

// NullInt4 returns an invalid (NULL) pgtype.Int4.
func NullInt4() pgtype.Int4 {
	return pgtype.Int4{Valid: false}
}

// Int8 creates a valid pgtype.Int8.
func Int8(v int64) pgtype.Int8 {
	return pgtype.Int8{Int64: v, Valid: true}
}

// NullInt8 returns an invalid (NULL) pgtype.Int8.
func NullInt8() pgtype.Int8 {
	return pgtype.Int8{Valid: false}
}

// Text creates a valid pgtype.Text.
func Text(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

// NullText returns an invalid (NULL) pgtype.Text.
func NullText() pgtype.Text {
	return pgtype.Text{Valid: false}
}

// TextPtr creates a pgtype.Text from *string.
func TextPtr(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}
