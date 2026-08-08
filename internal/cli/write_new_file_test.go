package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteNewFileCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "attachment.pdf")

	if err := writeNewFile(path, []byte("payload"), false); err != nil {
		t.Fatalf("writeNewFile() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "payload" {
		t.Errorf("content = %q, want %q", got, "payload")
	}
}

// TestWriteNewFileRefusesOverwrite is the regression guard: a malicious
// attachment filename must not be able to replace an existing file.
func TestWriteNewFileRefusesOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bashrc")
	original := "original user content\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	err := writeNewFile(path, []byte("attacker payload\n"), false)
	if err == nil {
		t.Fatal("writeNewFile() overwrote an existing file without --force")
	}
	if !strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("unexpected error text: %v", err)
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("error should tell the user how to proceed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("existing file was modified: %q", got)
	}
}

func TestWriteNewFileForceOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(path, []byte("stale content that is long"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := writeNewFile(path, []byte("new"), true); err != nil {
		t.Fatalf("writeNewFile(force) error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// O_TRUNC must clear the previous, longer content rather than leaving a tail.
	if string(got) != "new" {
		t.Errorf("content = %q, want %q", got, "new")
	}
}

func TestWriteNewFileReportsUnwritablePath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "file.txt")

	err := writeNewFile(path, []byte("x"), false)
	if err == nil {
		t.Fatal("expected an error for an unwritable path")
	}
	if strings.Contains(err.Error(), "refusing to overwrite") {
		t.Errorf("a missing directory must not be reported as an overwrite: %v", err)
	}
}

// TestWriteNewFileEmptyData covers a zero-byte attachment, which must still
// produce a file rather than being skipped.
func TestWriteNewFileEmptyData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.bin")

	if err := writeNewFile(path, nil, false); err != nil {
		t.Fatalf("writeNewFile() error = %v", err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Size() != 0 {
		t.Errorf("size = %d, want 0", st.Size())
	}
}
