package safetext

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSanitizeSingleLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Hello", "Hello"},
		{"empty", "", ""},
		{"folds newline", "a\nb", "a b"},
		{"folds tab", "a\tb", "a b"},
		{"folds CRLF", "a\r\nb", "a b"},
		// The ESC byte is removed, which is what neutralizes the sequence; the
		// remaining "[31m" is ordinary printable text and renders literally.
		{"still strips the ESC byte", "a\x1b[31mb", "a[31mb"},
		{"keeps unicode", "résumé", "résumé"},
		{
			"forged table row",
			"Invoice\n999   N  attacker@evil.test   Payroll",
			"Invoice 999   N  attacker@evil.test   Payroll",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeSingleLine(tc.in)
			if got != tc.want {
				t.Errorf("SanitizeSingleLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if strings.ContainsAny(got, "\n\t\r") {
				t.Errorf("result still contains a line/field break: %q", got)
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"under limit", "short", 10, "short"},
		{"exactly at limit", "abcde", 5, "abcde"},
		{"truncates ascii", "abcdefghij", 5, "ab..."},
		{"zero max", "abc", 0, ""},
		{"negative max", "abc", -1, ""},
		{"tiny max", "abcdef", 2, "ab"},
		{"empty input", "", 5, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := TruncateRunes(tc.in, tc.max); got != tc.want {
				t.Errorf("TruncateRunes(%q, %d) = %q, want %q", tc.in, tc.max, got, tc.want)
			}
		})
	}
}

// TestTruncateRunesKeepsValidUTF8 is the regression guard: the previous
// byte-slice truncation split multi-byte characters.
func TestTruncateRunesKeepsValidUTF8(t *testing.T) {
	subject := strings.Repeat("é", 40) // 80 bytes, 40 runes

	got := TruncateRunes(subject, 50)
	if !utf8.ValidString(got) {
		t.Errorf("produced invalid UTF-8: %q", got)
	}

	got = TruncateRunes(subject, 10)
	if !utf8.ValidString(got) {
		t.Errorf("produced invalid UTF-8: %q", got)
	}
	if r := []rune(got); len(r) != 10 {
		t.Errorf("rune count = %d, want 10", len(r))
	}
}

// TestTruncateRunesCountsRunesNotBytes documents that the limit is a display
// width in characters, so a multi-byte subject is not cut far shorter than an
// ASCII one.
func TestTruncateRunesCountsRunesNotBytes(t *testing.T) {
	if got := TruncateRunes(strings.Repeat("é", 20), 25); got != strings.Repeat("é", 20) {
		t.Errorf("a 20-rune string was truncated under a 25-rune limit: %q", got)
	}
}
