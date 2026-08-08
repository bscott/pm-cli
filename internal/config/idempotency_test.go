package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// useTempConfigDir points ConfigDir at a temporary location for the test.
func useTempConfigDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// os.UserConfigDir honors XDG_CONFIG_HOME on Linux and falls back to HOME
	// elsewhere, so set both.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return dir
}

func TestReserveIdempotencyKeyFirstUseSucceeds(t *testing.T) {
	useTempConfigDir(t)

	reserved, err := ReserveIdempotencyKey("send-001")
	if err != nil {
		t.Fatalf("ReserveIdempotencyKey() error = %v", err)
	}
	if !reserved {
		t.Error("first use of a key should reserve successfully")
	}
}

func TestReserveIdempotencyKeyRejectsDuplicate(t *testing.T) {
	useTempConfigDir(t)

	if _, err := ReserveIdempotencyKey("send-001"); err != nil {
		t.Fatal(err)
	}

	reserved, err := ReserveIdempotencyKey("send-001")
	if err != nil {
		t.Fatalf("ReserveIdempotencyKey() error = %v", err)
	}
	if reserved {
		t.Error("a second use of the same key must be rejected as a duplicate")
	}
}

func TestReserveIdempotencyKeyEmptyKeyAlwaysAllowed(t *testing.T) {
	useTempConfigDir(t)

	for i := 0; i < 3; i++ {
		reserved, err := ReserveIdempotencyKey("")
		if err != nil {
			t.Fatalf("ReserveIdempotencyKey(\"\") error = %v", err)
		}
		if !reserved {
			t.Error("an empty key disables the mechanism and must always reserve")
		}
	}
}

func TestReleaseAllowsRetry(t *testing.T) {
	useTempConfigDir(t)

	if _, err := ReserveIdempotencyKey("send-002"); err != nil {
		t.Fatal(err)
	}
	if err := ReleaseIdempotencyKey("send-002"); err != nil {
		t.Fatalf("ReleaseIdempotencyKey() error = %v", err)
	}

	reserved, err := ReserveIdempotencyKey("send-002")
	if err != nil {
		t.Fatal(err)
	}
	if !reserved {
		t.Error("a released key should be reservable again, so a failed send can retry")
	}
}

func TestReleaseUnheldKeyIsNotAnError(t *testing.T) {
	useTempConfigDir(t)

	if err := ReleaseIdempotencyKey("never-held"); err != nil {
		t.Errorf("releasing an unheld key should be a no-op, got %v", err)
	}
}

func TestReserveIdempotencyKeyExpiredMarkerIsReclaimed(t *testing.T) {
	useTempConfigDir(t)

	if _, err := ReserveIdempotencyKey("send-003"); err != nil {
		t.Fatal(err)
	}

	// Age the marker past the TTL.
	path, err := idempotencyMarkerPath("send-003")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-idempotencyTTL - time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	reserved, err := ReserveIdempotencyKey("send-003")
	if err != nil {
		t.Fatalf("ReserveIdempotencyKey() error = %v", err)
	}
	if !reserved {
		t.Error("a key older than the TTL should be reclaimable")
	}
}

// TestReserveIdempotencyKeyIsAtomic is the regression guard for the race the
// old check-then-record design allowed: many goroutines racing on one key must
// produce exactly one winner.
func TestReserveIdempotencyKeyIsAtomic(t *testing.T) {
	useTempConfigDir(t)

	const goroutines = 32
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		winners int
		errs    []error
	)

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			reserved, err := ReserveIdempotencyKey("concurrent-key")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			if reserved {
				winners++
			}
		}()
	}
	close(start)
	wg.Wait()

	for _, err := range errs {
		t.Errorf("unexpected error: %v", err)
	}
	if winners != 1 {
		t.Errorf("exactly one goroutine should win the reservation, got %d", winners)
	}
}

// TestKeyCannotEscapeIdempotencyDir checks that a key containing path
// separators cannot steer the marker outside the store.
func TestKeyCannotEscapeIdempotencyDir(t *testing.T) {
	useTempConfigDir(t)

	path, err := idempotencyMarkerPath("../../../etc/passwd")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := idempotencyDir()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("marker escaped the store: %q is not inside %q", path, dir)
	}
}

func TestMarkerFilePermissions(t *testing.T) {
	useTempConfigDir(t)

	if _, err := ReserveIdempotencyKey("perm-check"); err != nil {
		t.Fatal(err)
	}
	path, err := idempotencyMarkerPath("perm-check")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("marker permissions = %04o, want 0600", perm)
	}
}
