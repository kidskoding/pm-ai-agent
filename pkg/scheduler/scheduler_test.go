package scheduler

import (
	"testing"
	"time"
)

func TestShouldFire_Daily(t *testing.T) {
	job := Job{Name: "test", Hour: 9, Minute: 0}

	at9 := time.Date(2026, 1, 1, 9, 0, 30, 0, time.UTC)
	if !shouldFire(job, at9, time.Time{}) {
		t.Error("expected to fire at 09:00")
	}

	at901 := time.Date(2026, 1, 1, 9, 1, 0, 0, time.UTC)
	if shouldFire(job, at901, time.Time{}) {
		t.Error("should not fire at 09:01")
	}

	// must not fire twice in same minute
	if shouldFire(job, at9, at9.Add(-10*time.Second)) {
		t.Error("should not fire twice in same minute")
	}
}

func TestShouldFire_Weekly(t *testing.T) {
	monday := time.Monday
	job := Job{Name: "weekly", Hour: 9, Minute: 0, Weekday: &monday}

	// 2026-04-27 is a Monday
	mon := time.Date(2026, 4, 27, 9, 0, 0, 0, time.UTC)
	if !shouldFire(job, mon, time.Time{}) {
		t.Error("expected to fire on Monday at 09:00")
	}

	// 2026-04-28 is a Tuesday
	tue := time.Date(2026, 4, 28, 9, 0, 0, 0, time.UTC)
	if shouldFire(job, tue, time.Time{}) {
		t.Error("should not fire on Tuesday")
	}
}
