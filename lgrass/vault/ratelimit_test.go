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
