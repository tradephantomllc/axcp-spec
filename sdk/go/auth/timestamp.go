// Package auth provides timestamp validation for AXCP message authentication.
package auth

import (
	"errors"
	"time"
)

// Timestamp validation errors
var (
	// ErrTimestampMissing indicates the timestamp field is zero/missing
	ErrTimestampMissing = errors.New("auth: timestamp is missing or zero")

	// ErrTimestampExpired indicates the timestamp is too old (beyond MaxTimestampAge)
	ErrTimestampExpired = errors.New("auth: timestamp expired, message too old")

	// ErrTimestampFuture indicates the timestamp is too far in the future (beyond ClockSkew)
	ErrTimestampFuture = errors.New("auth: timestamp is in the future beyond allowed clock skew")
)

// Default timestamp validation parameters
const (
	// DefaultMaxTimestampAge is the default maximum age for message timestamps.
	// Messages older than this are rejected to prevent replay attacks.
	DefaultMaxTimestampAge = 5 * time.Minute

	// DefaultClockSkew is the default tolerance for clock differences between systems.
	// Messages with timestamps slightly in the future (within this window) are accepted.
	DefaultClockSkew = 30 * time.Second
)

// TimestampValidator provides configurable timestamp validation.
type TimestampValidator struct {
	// MaxAge is the maximum allowed age for message timestamps.
	// Messages with timestamps older than (now - MaxAge) are rejected.
	MaxAge time.Duration

	// ClockSkew is the tolerance for future timestamps.
	// Messages with timestamps up to (now + ClockSkew) are accepted.
	ClockSkew time.Duration

	// Clock provides the current time. If nil, time.Now() is used.
	// This field exists primarily for testing.
	Clock func() time.Time
}

// DefaultTimestampValidator returns a TimestampValidator with default settings.
func DefaultTimestampValidator() *TimestampValidator {
	return &TimestampValidator{
		MaxAge:    DefaultMaxTimestampAge,
		ClockSkew: DefaultClockSkew,
		Clock:     nil,
	}
}

// NewTimestampValidator creates a TimestampValidator with custom parameters.
func NewTimestampValidator(maxAge, clockSkew time.Duration) *TimestampValidator {
	return &TimestampValidator{
		MaxAge:    maxAge,
		ClockSkew: clockSkew,
		Clock:     nil,
	}
}

// now returns the current time, using the configured clock or time.Now().
func (v *TimestampValidator) now() time.Time {
	if v.Clock != nil {
		return v.Clock()
	}
	return time.Now()
}

// Validate checks if a timestamp (in milliseconds since Unix epoch) is valid.
// Returns nil if valid, or an appropriate error if invalid.
//
// Validation rules:
//   - Timestamp must be non-zero
//   - Timestamp must not be older than MaxAge
//   - Timestamp must not be more than ClockSkew in the future
func (v *TimestampValidator) Validate(timestampMs uint64) error {
	if timestampMs == 0 {
		return ErrTimestampMissing
	}

	// Convert milliseconds to time.Time
	msgTime := time.UnixMilli(int64(timestampMs))
	now := v.now()

	// Check if timestamp is too old
	oldestAllowed := now.Add(-v.MaxAge)
	if msgTime.Before(oldestAllowed) {
		return ErrTimestampExpired
	}

	// Check if timestamp is too far in the future
	newestAllowed := now.Add(v.ClockSkew)
	if msgTime.After(newestAllowed) {
		return ErrTimestampFuture
	}

	return nil
}

// ValidateTime checks if a time.Time timestamp is valid.
// This is a convenience method for when you already have a time.Time.
func (v *TimestampValidator) ValidateTime(ts time.Time) error {
	if ts.IsZero() {
		return ErrTimestampMissing
	}

	now := v.now()

	// Check if timestamp is too old
	oldestAllowed := now.Add(-v.MaxAge)
	if ts.Before(oldestAllowed) {
		return ErrTimestampExpired
	}

	// Check if timestamp is too far in the future
	newestAllowed := now.Add(v.ClockSkew)
	if ts.After(newestAllowed) {
		return ErrTimestampFuture
	}

	return nil
}

// IsExpired checks if a timestamp (in milliseconds) is expired.
// Returns true if the timestamp is older than MaxAge.
func (v *TimestampValidator) IsExpired(timestampMs uint64) bool {
	if timestampMs == 0 {
		return true
	}
	msgTime := time.UnixMilli(int64(timestampMs))
	oldestAllowed := v.now().Add(-v.MaxAge)
	return msgTime.Before(oldestAllowed)
}

// IsFuture checks if a timestamp (in milliseconds) is too far in the future.
// Returns true if the timestamp is beyond (now + ClockSkew).
func (v *TimestampValidator) IsFuture(timestampMs uint64) bool {
	if timestampMs == 0 {
		return false
	}
	msgTime := time.UnixMilli(int64(timestampMs))
	newestAllowed := v.now().Add(v.ClockSkew)
	return msgTime.After(newestAllowed)
}

// Package-level convenience functions using default settings

// ValidateTimestamp validates a timestamp using default settings.
// This is a convenience function for simple use cases.
func ValidateTimestamp(timestampMs uint64) error {
	return DefaultTimestampValidator().Validate(timestampMs)
}

// ValidateTimestampWithParams validates a timestamp with custom parameters.
func ValidateTimestampWithParams(timestampMs uint64, maxAge, clockSkew time.Duration) error {
	v := NewTimestampValidator(maxAge, clockSkew)
	return v.Validate(timestampMs)
}

// IsTimestampExpired checks if a timestamp is expired using default MaxAge.
func IsTimestampExpired(timestampMs uint64) bool {
	return DefaultTimestampValidator().IsExpired(timestampMs)
}

// IsTimestampFuture checks if a timestamp is too far in the future using default ClockSkew.
func IsTimestampFuture(timestampMs uint64) bool {
	return DefaultTimestampValidator().IsFuture(timestampMs)
}

// NowMs returns the current time as milliseconds since Unix epoch.
// Useful for setting timestamps on outgoing messages.
func NowMs() uint64 {
	return uint64(time.Now().UnixMilli())
}
