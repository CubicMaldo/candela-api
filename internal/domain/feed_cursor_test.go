package domain

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestFeedCursorRoundTrip(t *testing.T) {
	original := FeedCursor{
		Layer:       2,
		CheckedInAt: time.Date(2024, time.June, 5, 21, 30, 15, 123456789, time.UTC),
		EventID:     uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
	}

	decoded, err := DecodeFeedCursor(original.Encode())
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if decoded.Layer != original.Layer {
		t.Errorf("Layer = %d, want %d", decoded.Layer, original.Layer)
	}
	if !decoded.CheckedInAt.Equal(original.CheckedInAt) {
		t.Errorf("CheckedInAt = %s, want %s (nanosecond precision must survive)",
			decoded.CheckedInAt, original.CheckedInAt)
	}
	if decoded.EventID != original.EventID {
		t.Errorf("EventID = %s, want %s", decoded.EventID, original.EventID)
	}
}

func TestFeedCursorEncodeIsURLSafe(t *testing.T) {
	token := FeedCursor{Layer: 1, CheckedInAt: time.Now().UTC(), EventID: uuid.New()}.Encode()
	for _, r := range token {
		ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_'
		if !ok {
			t.Fatalf("token %q contains %q, which needs escaping in a query string", token, r)
		}
	}
}

func TestDecodeFeedCursorRejectsGarbage(t *testing.T) {
	tests := map[string]string{
		"not base64":     "!!!not-base64!!!",
		"too few fields": encodeRaw("1|2024-06-05T21:30:00Z"),
		"bad layer":      encodeRaw("x|2024-06-05T21:30:00Z|6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		"bad timestamp":  encodeRaw("1|yesterday|6ba7b810-9dad-11d1-80b4-00c04fd430c8"),
		"bad uuid":       encodeRaw("1|2024-06-05T21:30:00Z|nope"),
		"empty":          "",
	}

	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeFeedCursor(token); err != ErrInvalidCursor {
				t.Errorf("got err = %v, want ErrInvalidCursor", err)
			}
		})
	}
}

// encodeRaw builds a cursor token from an already-formed payload, so decode
// failures can be exercised without going through Encode.
func encodeRaw(payload string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(payload))
}
