package safetext

import "testing"

func TestSanitizeHeaderValue(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Hello", "Hello"},
		{"empty", "", ""},
		{"strips CR", "a\rb", "ab"},
		{"strips LF", "a\nb", "ab"},
		{"strips CRLF", "a\r\nb", "ab"},
		{"inject Bcc", "Hello\r\nBcc: attacker@evil.example", "HelloBcc: attacker@evil.example"},
		{"multiple injections", "Subject\r\nX-Evil: 1\r\nY-Evil: 2", "SubjectX-Evil: 1Y-Evil: 2"},
		{"leaves tabs", "a\tb", "a\tb"},
		{"leaves unicode", "résumé", "résumé"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeHeaderValue(tc.in)
			if got != tc.want {
				t.Errorf("SanitizeHeaderValue(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSanitizeForTerminal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "Hello", "Hello"},
		{"empty", "", ""},
		{"preserves tab", "a\tb", "a\tb"},
		{"preserves newline", "a\nb", "a\nb"},
		{"strips ESC", "\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"strips BEL", "ding\x07dong", "dingdong"},
		{"strips DEL", "abc\x7fdef", "abcdef"},
		{"strips C0 NUL", "a\x00b", "ab"},
		{"strips C1 CSI (valid UTF-8)", "ab", "ab"},
		{"preserves unicode", "héllo 日本", "héllo 日本"},
		{"OSC 8 hyperlink", "\x1b]8;;http://evil.example\x1b\\click\x1b]8;;\x1b\\", "]8;;http://evil.example\\click]8;;\\"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SanitizeForTerminal(tc.in)
			if got != tc.want {
				t.Errorf("SanitizeForTerminal(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
