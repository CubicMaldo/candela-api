package domain

import (
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidCursor is returned when a feed cursor cannot be decoded.
var ErrInvalidCursor = errors.New("invalid cursor")

// FeedCursor points at the last event of a page. The feed is ordered by
// priority layer first and timestamp second, so a timestamp alone cannot
// resume it: layer 3 rows are older than layer 1 rows the reader has not seen
// yet, and paging on time alone re-serves them. The layer, the timestamp and
// the event id together identify exactly one position in that ordering.
type FeedCursor struct {
	Layer       int
	CheckedInAt time.Time
	EventID     uuid.UUID
}

// Encode renders the cursor as an opaque token for the client.
func (c FeedCursor) Encode() string {
	raw := strings.Join([]string{
		strconv.Itoa(c.Layer),
		c.CheckedInAt.UTC().Format(time.RFC3339Nano),
		c.EventID.String(),
	}, "|")
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// DecodeFeedCursor parses a token produced by FeedCursor.Encode.
func DecodeFeedCursor(token string) (FeedCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return FeedCursor{}, ErrInvalidCursor
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 3 {
		return FeedCursor{}, ErrInvalidCursor
	}

	layer, err := strconv.Atoi(parts[0])
	if err != nil {
		return FeedCursor{}, ErrInvalidCursor
	}
	ts, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return FeedCursor{}, ErrInvalidCursor
	}
	id, err := uuid.Parse(parts[2])
	if err != nil {
		return FeedCursor{}, ErrInvalidCursor
	}

	return FeedCursor{Layer: layer, CheckedInAt: ts, EventID: id}, nil
}
