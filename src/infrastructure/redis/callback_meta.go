package redis

import (
	"encoding/json"

	e "github.com/ChatDetectiveORG/shared/errors"
	"github.com/google/uuid"
)

// Default TTL for inline callback metadata: long enough to survive a full export flow
// (invoice → payment → restore), but short enough that abandoned buttons get cleaned up.
const callbackMetaTTLSeconds = 24 * 60 * 60

// callbackMetaKey returns the Redis key for the given short id.
func callbackMetaKey(id string) string {
	return "cb:" + id
}

// StoreCallbackMeta serialises payload as JSON and stores it under a freshly generated id.
// Returns the id; callers attach it to inline button data as "<unique>\n<id>".
func StoreCallbackMeta(payload any) (string, *e.ErrorInfo) {
	body, rawErr := json.Marshal(payload)
	if rawErr != nil {
		return "", e.FromError(rawErr, "failed to marshal callback metadata").WithSeverity(e.Notice)
	}

	id := uuid.New().String()
	conn, err := RedisConn()
	if e.IsNonNil(err) {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	if _, rawErr := conn.Do("SET", callbackMetaKey(id), body, "EX", callbackMetaTTLSeconds); rawErr != nil {
		return "", e.FromError(rawErr, "failed to store callback metadata").WithSeverity(e.Notice)
	}
	return id, e.Nil()
}

// LoadCallbackMeta reads the JSON payload previously stored via StoreCallbackMeta.
// Missing keys are surfaced as a Notice-severity error so callers can fall back to UI hints.
func LoadCallbackMeta(id string, dst any) *e.ErrorInfo {
	if id == "" {
		return e.NewError("empty callback meta id", "callback metadata id is required").WithSeverity(e.Notice)
	}

	conn, err := RedisConn()
	if e.IsNonNil(err) {
		return err
	}
	defer func() { _ = conn.Close() }()

	reply, rawErr := conn.Do("GET", callbackMetaKey(id))
	if rawErr != nil {
		return e.FromError(rawErr, "failed to load callback metadata").WithSeverity(e.Notice)
	}
	if reply == nil {
		return e.NewError("callback metadata expired", "callback metadata expired or missing").WithSeverity(e.Notice)
	}
	body, ok := reply.([]byte)
	if !ok {
		if s, isString := reply.(string); isString {
			body = []byte(s)
		} else {
			return e.NewError("unexpected redis reply", "callback metadata stored in unexpected type").WithSeverity(e.Notice)
		}
	}
	if rawErr := json.Unmarshal(body, dst); rawErr != nil {
		return e.FromError(rawErr, "failed to unmarshal callback metadata").WithSeverity(e.Notice)
	}
	return e.Nil()
}
