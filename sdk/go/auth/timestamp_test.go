package auth

import (
	"testing"
	"time"
)

func TestTimestampValidator_Validate_Valid(t *testing.T) {
	v := DefaultTimestampValidator()

	// Current time should be valid
	now := NowMs()
	if err := v.Validate(now); err != nil {
		t.Errorf("current timestamp should be valid: %v", err)
	}

	// 1 minute ago should be valid (within 5 min default)
	oneMinAgo := uint64(time.Now().Add(-1 * time.Minute).UnixMilli())
	if err := v.Validate(oneMinAgo); err != nil {
		t.Errorf("1 minute ago should be valid: %v", err)
	}

	// 4 minutes ago should be valid (within 5 min default)
	fourMinAgo := uint64(time.Now().Add(-4 * time.Minute).UnixMilli())
	if err := v.Validate(fourMinAgo); err != nil {
		t.Errorf("4 minutes ago should be valid: %v", err)
	}
}

func TestTimestampValidator_Validate_Missing(t *testing.T) {
	v := DefaultTimestampValidator()

	// Zero timestamp should fail
	err := v.Validate(0)
	if err != ErrTimestampMissing {
		t.Errorf("expected ErrTimestampMissing, got: %v", err)
	}
}

func TestTimestampValidator_Validate_Expired(t *testing.T) {
	v := DefaultTimestampValidator()

	// 6 minutes ago should be expired (beyond 5 min default)
	sixMinAgo := uint64(time.Now().Add(-6 * time.Minute).UnixMilli())
	err := v.Validate(sixMinAgo)
	if err != ErrTimestampExpired {
		t.Errorf("expected ErrTimestampExpired, got: %v", err)
	}

	// 1 hour ago should definitely be expired
	oneHourAgo := uint64(time.Now().Add(-1 * time.Hour).UnixMilli())
	err = v.Validate(oneHourAgo)
	if err != ErrTimestampExpired {
		t.Errorf("expected ErrTimestampExpired for 1 hour ago, got: %v", err)
	}
}

func TestTimestampValidator_Validate_Future(t *testing.T) {
	v := DefaultTimestampValidator()

	// 1 minute in future should be rejected (beyond 30s default clock skew)
	oneMinFuture := uint64(time.Now().Add(1 * time.Minute).UnixMilli())
	err := v.Validate(oneMinFuture)
	if err != ErrTimestampFuture {
		t.Errorf("expected ErrTimestampFuture, got: %v", err)
	}

	// 10 seconds in future should be valid (within 30s clock skew)
	tenSecFuture := uint64(time.Now().Add(10 * time.Second).UnixMilli())
	if err := v.Validate(tenSecFuture); err != nil {
		t.Errorf("10 seconds in future should be valid: %v", err)
	}
}

func TestTimestampValidator_Validate_ClockSkew(t *testing.T) {
	v := DefaultTimestampValidator()

	// 25 seconds in future should be valid (within 30s clock skew)
	ts := uint64(time.Now().Add(25 * time.Second).UnixMilli())
	if err := v.Validate(ts); err != nil {
		t.Errorf("25 seconds in future should be within clock skew: %v", err)
	}

	// 35 seconds in future should be rejected (beyond 30s clock skew)
	ts = uint64(time.Now().Add(35 * time.Second).UnixMilli())
	err := v.Validate(ts)
	if err != ErrTimestampFuture {
		t.Errorf("35 seconds in future should be rejected, got: %v", err)
	}
}

func TestTimestampValidator_CustomParams(t *testing.T) {
	// Create validator with 1 minute max age and 10 second clock skew
	v := NewTimestampValidator(1*time.Minute, 10*time.Second)

	// 30 seconds ago should be valid
	ts := uint64(time.Now().Add(-30 * time.Second).UnixMilli())
	if err := v.Validate(ts); err != nil {
		t.Errorf("30 seconds ago should be valid with 1 min max age: %v", err)
	}

	// 2 minutes ago should be expired
	ts = uint64(time.Now().Add(-2 * time.Minute).UnixMilli())
	err := v.Validate(ts)
	if err != ErrTimestampExpired {
		t.Errorf("2 minutes ago should be expired with 1 min max age, got: %v", err)
	}

	// 5 seconds in future should be valid
	ts = uint64(time.Now().Add(5 * time.Second).UnixMilli())
	if err := v.Validate(ts); err != nil {
		t.Errorf("5 seconds in future should be valid with 10s clock skew: %v", err)
	}

	// 15 seconds in future should be rejected
	ts = uint64(time.Now().Add(15 * time.Second).UnixMilli())
	err = v.Validate(ts)
	if err != ErrTimestampFuture {
		t.Errorf("15 seconds in future should be rejected with 10s clock skew, got: %v", err)
	}
}

func TestTimestampValidator_MockClock(t *testing.T) {
	// Fixed time for testing
	fixedTime := time.Date(2026, 1, 31, 12, 0, 0, 0, time.UTC)

	v := &TimestampValidator{
		MaxAge:    5 * time.Minute,
		ClockSkew: 30 * time.Second,
		Clock:     func() time.Time { return fixedTime },
	}

	// Exactly at fixed time should be valid
	ts := uint64(fixedTime.UnixMilli())
	if err := v.Validate(ts); err != nil {
		t.Errorf("timestamp at fixed time should be valid: %v", err)
	}

	// 3 minutes before fixed time should be valid
	ts = uint64(fixedTime.Add(-3 * time.Minute).UnixMilli())
	if err := v.Validate(ts); err != nil {
		t.Errorf("3 minutes before fixed time should be valid: %v", err)
	}

	// 6 minutes before fixed time should be expired
	ts = uint64(fixedTime.Add(-6 * time.Minute).UnixMilli())
	err := v.Validate(ts)
	if err != ErrTimestampExpired {
		t.Errorf("6 minutes before fixed time should be expired, got: %v", err)
	}
}

func TestTimestampValidator_ValidateTime(t *testing.T) {
	v := DefaultTimestampValidator()

	// Current time should be valid
	if err := v.ValidateTime(time.Now()); err != nil {
		t.Errorf("current time should be valid: %v", err)
	}

	// Zero time should be missing
	err := v.ValidateTime(time.Time{})
	if err != ErrTimestampMissing {
		t.Errorf("zero time should be missing, got: %v", err)
	}

	// Old time should be expired
	err = v.ValidateTime(time.Now().Add(-10 * time.Minute))
	if err != ErrTimestampExpired {
		t.Errorf("10 minutes ago should be expired, got: %v", err)
	}

	// Future time should be rejected
	err = v.ValidateTime(time.Now().Add(2 * time.Minute))
	if err != ErrTimestampFuture {
		t.Errorf("2 minutes in future should be rejected, got: %v", err)
	}
}

func TestTimestampValidator_IsExpired(t *testing.T) {
	v := DefaultTimestampValidator()

	// Zero should be expired
	if !v.IsExpired(0) {
		t.Error("zero timestamp should be expired")
	}

	// Current time should not be expired
	if v.IsExpired(NowMs()) {
		t.Error("current timestamp should not be expired")
	}

	// 6 minutes ago should be expired
	ts := uint64(time.Now().Add(-6 * time.Minute).UnixMilli())
	if !v.IsExpired(ts) {
		t.Error("6 minutes ago should be expired")
	}

	// 3 minutes ago should not be expired
	ts = uint64(time.Now().Add(-3 * time.Minute).UnixMilli())
	if v.IsExpired(ts) {
		t.Error("3 minutes ago should not be expired")
	}
}

func TestTimestampValidator_IsFuture(t *testing.T) {
	v := DefaultTimestampValidator()

	// Zero should not be future
	if v.IsFuture(0) {
		t.Error("zero timestamp should not be future")
	}

	// Current time should not be future
	if v.IsFuture(NowMs()) {
		t.Error("current timestamp should not be future")
	}

	// 1 minute in future should be future
	ts := uint64(time.Now().Add(1 * time.Minute).UnixMilli())
	if !v.IsFuture(ts) {
		t.Error("1 minute in future should be future")
	}

	// 10 seconds in future should not be future (within clock skew)
	ts = uint64(time.Now().Add(10 * time.Second).UnixMilli())
	if v.IsFuture(ts) {
		t.Error("10 seconds in future should not be future (within clock skew)")
	}
}

func TestPackageLevelFunctions(t *testing.T) {
	// Test ValidateTimestamp
	if err := ValidateTimestamp(NowMs()); err != nil {
		t.Errorf("ValidateTimestamp(NowMs()) should succeed: %v", err)
	}

	if err := ValidateTimestamp(0); err != ErrTimestampMissing {
		t.Errorf("ValidateTimestamp(0) should return ErrTimestampMissing, got: %v", err)
	}

	// Test ValidateTimestampWithParams
	ts := uint64(time.Now().Add(-2 * time.Minute).UnixMilli())
	err := ValidateTimestampWithParams(ts, 1*time.Minute, 10*time.Second)
	if err != ErrTimestampExpired {
		t.Errorf("ValidateTimestampWithParams should return ErrTimestampExpired, got: %v", err)
	}

	// Test IsTimestampExpired
	if IsTimestampExpired(NowMs()) {
		t.Error("IsTimestampExpired(NowMs()) should be false")
	}

	ts = uint64(time.Now().Add(-10 * time.Minute).UnixMilli())
	if !IsTimestampExpired(ts) {
		t.Error("IsTimestampExpired for 10 min ago should be true")
	}

	// Test IsTimestampFuture
	if IsTimestampFuture(NowMs()) {
		t.Error("IsTimestampFuture(NowMs()) should be false")
	}

	ts = uint64(time.Now().Add(5 * time.Minute).UnixMilli())
	if !IsTimestampFuture(ts) {
		t.Error("IsTimestampFuture for 5 min future should be true")
	}
}

func TestNowMs(t *testing.T) {
	before := time.Now().UnixMilli()
	now := NowMs()
	after := time.Now().UnixMilli()

	if int64(now) < before || int64(now) > after {
		t.Errorf("NowMs() = %d, expected between %d and %d", now, before, after)
	}
}

func TestDefaultTimestampValidator(t *testing.T) {
	v := DefaultTimestampValidator()

	if v.MaxAge != DefaultMaxTimestampAge {
		t.Errorf("MaxAge = %v, expected %v", v.MaxAge, DefaultMaxTimestampAge)
	}

	if v.ClockSkew != DefaultClockSkew {
		t.Errorf("ClockSkew = %v, expected %v", v.ClockSkew, DefaultClockSkew)
	}

	if v.Clock != nil {
		t.Error("Clock should be nil for default validator")
	}
}
