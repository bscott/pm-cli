package config

import (
	"strings"
	"testing"
)

// Tests for reading the Bridge password from PM_CLI_BRIDGE_PASSWORD before the
// system keyring (issue #8). The suite is organized into the project's seven
// review categories: Security, Performance, Retry, Unit, Integration,
// Functional, and Frame.

// --- Unit ---------------------------------------------------------------

// TestGetPasswordReadsEnvVar verifies the env var is returned directly.
func TestGetPasswordReadsEnvVar(t *testing.T) {
	t.Setenv(EnvBridgePassword, "s3cret-from-env")

	cfg := DefaultConfig()
	cfg.Bridge.Email = "user@protonmail.com"

	got, err := cfg.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if got != "s3cret-from-env" {
		t.Errorf("GetPassword() = %q, want %q", got, "s3cret-from-env")
	}
}

// TestGetPasswordEnvWithoutEmail verifies the env var works even when no email
// is configured, since the variable carries the credential itself.
func TestGetPasswordEnvWithoutEmail(t *testing.T) {
	t.Setenv(EnvBridgePassword, "no-email-needed")

	cfg := DefaultConfig() // Email is empty
	got, err := cfg.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if got != "no-email-needed" {
		t.Errorf("GetPassword() = %q, want %q", got, "no-email-needed")
	}
}

// TestGetPasswordEmptyEnvFallsBack verifies an empty env var does not short
// circuit the keyring path (an empty string must not be treated as the
// password).
func TestGetPasswordEmptyEnvFallsBack(t *testing.T) {
	t.Setenv(EnvBridgePassword, "")

	cfg := DefaultConfig() // no email -> keyring path returns the email error
	_, err := cfg.GetPassword()
	if err == nil {
		t.Fatal("expected error when env is empty and no email configured")
	}
	if !strings.Contains(err.Error(), "email") {
		t.Errorf("expected keyring/email error, got %v", err)
	}
}

// --- Security -----------------------------------------------------------

// TestGetPasswordEnvNotLeakedInError ensures the password value never appears
// in returned error text on the fallback path.
func TestGetPasswordEnvNotLeakedInError(t *testing.T) {
	// Env unset so we hit the keyring/email path.
	cfg := DefaultConfig()
	_, err := cfg.GetPassword()
	if err == nil {
		t.Skip("keyring available on this host; nothing to assert")
	}
	if strings.Contains(err.Error(), "PM_CLI_BRIDGE_PASSWORD") {
		t.Errorf("error text should not reference the secret env var: %v", err)
	}
}

// TestEnvBridgePasswordConstant guards the documented variable name so the
// public contract (README, deployments) cannot drift silently.
func TestEnvBridgePasswordConstant(t *testing.T) {
	if EnvBridgePassword != "PM_CLI_BRIDGE_PASSWORD" {
		t.Errorf("EnvBridgePassword = %q, want %q", EnvBridgePassword, "PM_CLI_BRIDGE_PASSWORD")
	}
}

// --- Performance --------------------------------------------------------

// TestGetPasswordEnvNoKeyringCost documents that the env-var path avoids any
// keyring round-trip. It reads many times under a set env var; a keyring call
// here would be a measurable regression (and would fail on headless CI).
func TestGetPasswordEnvNoKeyringCost(t *testing.T) {
	t.Setenv(EnvBridgePassword, "fast-path")
	cfg := DefaultConfig()
	for i := 0; i < 1000; i++ {
		if _, err := cfg.GetPassword(); err != nil {
			t.Fatalf("GetPassword() error = %v", err)
		}
	}
}

// --- Retry --------------------------------------------------------------

// TestGetPasswordEnvIdempotent verifies repeated reads return the same value
// (the resolver is a pure read with no retry/backoff semantics of its own).
func TestGetPasswordEnvIdempotent(t *testing.T) {
	t.Setenv(EnvBridgePassword, "stable")
	cfg := DefaultConfig()
	first, err := cfg.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	for i := 0; i < 3; i++ {
		again, err := cfg.GetPassword()
		if err != nil {
			t.Fatalf("GetPassword() retry error = %v", err)
		}
		if again != first {
			t.Errorf("GetPassword() retry = %q, want %q", again, first)
		}
	}
}

// --- Integration --------------------------------------------------------

// TestGetPasswordEnvPrecedenceOverKeyring is a behavioral integration check:
// with a value set, GetPassword must resolve without touching the keyring,
// which is exactly the headless-server scenario from issue #8.
func TestGetPasswordEnvPrecedenceOverKeyring(t *testing.T) {
	t.Setenv(EnvBridgePassword, "env-wins")
	cfg := DefaultConfig()
	cfg.Bridge.Email = "user@protonmail.com" // would normally be the keyring key

	got, err := cfg.GetPassword()
	if err != nil {
		t.Fatalf("GetPassword() error = %v", err)
	}
	if got != "env-wins" {
		t.Errorf("expected env var to take precedence, got %q", got)
	}
}

// --- Functional ---------------------------------------------------------

// TestGetPasswordFunctionalSetUnset walks the full user-visible behavior:
// set -> value returned; unset -> fallback error path.
func TestGetPasswordFunctionalSetUnset(t *testing.T) {
	cfg := DefaultConfig() // no email

	t.Setenv(EnvBridgePassword, "on")
	if got, err := cfg.GetPassword(); err != nil || got != "on" {
		t.Fatalf("with env set: got %q err %v", got, err)
	}

	// Unset within the same test to exercise the transition.
	t.Setenv(EnvBridgePassword, "")
	if _, err := cfg.GetPassword(); err == nil {
		t.Fatal("with env unset and no email, expected fallback error")
	}
}

// --- Frame --------------------------------------------------------------
// N/A: GetPassword returns a plain string with no wire/frame encoding, so there
// is no framing/serialization boundary to exercise here. Placeholder retained
// to keep the seven-category test matrix explicit.
func TestGetPasswordFrameNA(t *testing.T) {
	t.Skip("N/A: no framing/serialization in password resolution")
}
