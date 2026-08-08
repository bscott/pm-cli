package config

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestScrubSecretsRemovesBridgePassword(t *testing.T) {
	env := []string{
		"PATH=/usr/bin",
		EnvBridgePassword + "=super-secret",
		"HOME=/home/user",
	}

	got := ScrubSecrets(env)

	for _, kv := range got {
		if strings.HasPrefix(kv, EnvBridgePassword+"=") {
			t.Fatalf("ScrubSecrets left the credential in place: %q", kv)
		}
		if strings.Contains(kv, "super-secret") {
			t.Fatalf("ScrubSecrets leaked the secret value: %q", kv)
		}
	}
	if len(got) != 2 {
		t.Errorf("expected 2 surviving entries, got %d: %v", len(got), got)
	}
}

func TestScrubSecretsKeepsUnrelatedVars(t *testing.T) {
	env := []string{"PATH=/usr/bin", "PM_MSG_FROM=a@b.test", "LANG=C"}

	got := ScrubSecrets(env)

	if len(got) != len(env) {
		t.Fatalf("unrelated vars were dropped: %v", got)
	}
	for i := range env {
		if got[i] != env[i] {
			t.Errorf("entry %d changed: got %q, want %q", i, got[i], env[i])
		}
	}
}

// TestScrubSecretsDoesNotMatchPrefixes guards against dropping variables that
// merely start with the secret's name.
func TestScrubSecretsDoesNotMatchPrefixes(t *testing.T) {
	similar := EnvBridgePassword + "_BACKUP=keep-me"
	got := ScrubSecrets([]string{similar})

	if len(got) != 1 || got[0] != similar {
		t.Errorf("a similarly-named variable was dropped: %v", got)
	}
}

// TestScrubSecretsBareName covers an entry with no "=", which Go permits.
func TestScrubSecretsBareName(t *testing.T) {
	got := ScrubSecrets([]string{EnvBridgePassword, "PATH=/usr/bin"})

	for _, kv := range got {
		if kv == EnvBridgePassword {
			t.Error("bare credential name survived scrubbing")
		}
	}
}

func TestScrubSecretsDoesNotMutateInput(t *testing.T) {
	env := []string{"PATH=/usr/bin", EnvBridgePassword + "=secret"}
	_ = ScrubSecrets(env)

	if len(env) != 2 || env[1] != EnvBridgePassword+"=secret" {
		t.Errorf("input slice was modified: %v", env)
	}
}

func TestScrubSecretsEmpty(t *testing.T) {
	if got := ScrubSecrets(nil); len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

// TestScrubbedEnvironNotVisibleToChild is the end-to-end guard for the actual
// leak: a real child process must not be able to read the Bridge password.
func TestScrubbedEnvironNotVisibleToChild(t *testing.T) {
	t.Setenv(EnvBridgePassword, "bridge-secret-value")

	// Mirrors MailWatchCmd.executeCommand's environment construction.
	cmd := exec.Command("sh", "-c", `printf '%s' "$`+EnvBridgePassword+`"`)
	cmd.Env = append(ScrubSecrets(os.Environ()), "PM_MSG_SEQ=1")

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("child failed: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("child could read the Bridge password: %q", out)
	}
}

// TestUnscrubbedEnvironWouldLeak pins the regression this fix prevents: using
// os.Environ() directly does expose the credential. If this ever stops
// leaking, the scrubbing above is no longer load-bearing and the test should
// be revisited rather than deleted.
func TestUnscrubbedEnvironWouldLeak(t *testing.T) {
	t.Setenv(EnvBridgePassword, "bridge-secret-value")

	cmd := exec.Command("sh", "-c", `printf '%s' "$`+EnvBridgePassword+`"`)
	cmd.Env = os.Environ()

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("child failed: %v", err)
	}
	if string(out) != "bridge-secret-value" {
		t.Skipf("environment did not propagate as expected (got %q); "+
			"the scrubbed-child test above is the load-bearing assertion", out)
	}
}
