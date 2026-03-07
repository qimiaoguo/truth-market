package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgtype <-> Go native type conversion helpers.

func textFromString(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func textFromOptionalString(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func stringFromText(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func tstz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func tstzFromOptional(t *time.Time) pgtype.Timestamptz {
	if t == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: *t, Valid: true}
}

func optionalTimeFromTstz(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	return &t.Time
}

func uuidFromOptionalString(s *string) pgtype.UUID {
	if s == nil {
		return pgtype.UUID{}
	}
	b, err := parseUUID(*s)
	if err != nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: b, Valid: true}
}

func optionalStringFromUUID(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := formatUUID(u.Bytes)
	return &s
}

// parseUUID parses a UUID string (with or without hyphens) into [16]byte.
func parseUUID(s string) ([16]byte, error) {
	var b [16]byte
	src := make([]byte, 0, 32)
	for _, c := range []byte(s) {
		if c != '-' {
			src = append(src, c)
		}
	}
	if len(src) != 32 {
		return b, pgtype.ErrScanTargetTypeChanged
	}
	for i := 0; i < 16; i++ {
		hi := unhex(src[i*2])
		lo := unhex(src[i*2+1])
		b[i] = hi<<4 | lo
	}
	return b, nil
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// formatUUID formats [16]byte as a standard UUID string.
func formatUUID(b [16]byte) string {
	const hex = "0123456789abcdef"
	buf := make([]byte, 36)
	idx := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			buf[idx] = '-'
			idx++
		}
		buf[idx] = hex[v>>4]
		buf[idx+1] = hex[v&0x0f]
		idx += 2
	}
	return string(buf)
}
