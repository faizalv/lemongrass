package vault

import (
	"errors"
	"testing"
	"time"
)

func TestFailureLimiterLocksOutAtThreshold(t *testing.T) {
	f := NewFailureLimiter(3, time.Minute, time.Hour)
	for i := 0; i < 2; i++ {
		if err := f.Check(); err != nil {
			t.Fatalf("Check before threshold: %v", err)
		}
		f.RecordFailure()
	}
	if err := f.Check(); err != nil {
		t.Fatalf("Check on the failure just short of threshold: %v", err)
	}
	f.RecordFailure()
	if err := f.Check(); !errors.Is(err, ErrLockedOut) {
		t.Errorf("Check at threshold: got %v, want ErrLockedOut", err)
	}
}

func TestFailureLimiterSuccessResetsCount(t *testing.T) {
	f := NewFailureLimiter(3, time.Minute, time.Hour)
	f.RecordFailure()
	f.RecordFailure()
	f.RecordSuccess()
	f.RecordFailure()
	if err := f.Check(); err != nil {
		t.Errorf("Check after a success reset the count: got %v, want nil", err)
	}
}

func TestFailureLimiterOldFailuresOutsideWindowDontCount(t *testing.T) {
	f := NewFailureLimiter(2, 10*time.Millisecond, time.Hour)
	f.RecordFailure()
	time.Sleep(20 * time.Millisecond)
	f.RecordFailure()
	if err := f.Check(); err != nil {
		t.Errorf("Check with the first failure outside the window: got %v, want nil", err)
	}
}

func TestFailureLimiterLockoutStatesWaitTime(t *testing.T) {
	f := NewFailureLimiter(1, time.Minute, 5*time.Minute)
	f.RecordFailure()
	err := f.Check()
	if !errors.Is(err, ErrLockedOut) {
		t.Fatalf("Check = %v, want ErrLockedOut", err)
	}
	if got, want := err.Error(), "vault: too many failed attempts, locked out, try again in 5 minutes"; got != want {
		t.Errorf("Check error = %q, want %q", got, want)
	}
}

func TestWaitText(t *testing.T) {
	cases := map[time.Duration]string{
		time.Second:                    "1 second",
		30 * time.Second:               "30 seconds",
		time.Minute:                    "1 minute",
		time.Minute + time.Second:      "2 minutes",
		4*time.Minute + 30*time.Second: "5 minutes",
	}
	for d, want := range cases {
		if got := waitText(d); got != want {
			t.Errorf("waitText(%v) = %q, want %q", d, got, want)
		}
	}
}
