// Package safetext provides helpers for defanging attacker-controlled
// strings before they reach sensitive sinks (mail headers, terminal).
package safetext

import "strings"

// SanitizeHeaderValue strips CR and LF from v to prevent RFC 5322 header
// injection. Call this on any header value derived from untrusted input
// (e.g., Message-ID, Subject, or From parsed out of received emails) before
// writing it to an SMTP message or a local RFC 822 message (draft, etc.).
func SanitizeHeaderValue(v string) string {
	if !strings.ContainsAny(v, "\r\n") {
		return v
	}
	v = strings.ReplaceAll(v, "\r", "")
	v = strings.ReplaceAll(v, "\n", "")
	return v
}

// SanitizeForTerminal strips C0/C1 control characters (including ANSI
// escape prefix 0x1B) and DEL, preserving tab and newline. Use on
// attacker-controlled strings (email Subject, From, Body, attachment
// filename) before printing to a TTY — unfiltered escapes can obscure
// output, spoof hyperlinks via OSC 8, or manipulate the terminal clipboard
// via OSC 52.
func SanitizeForTerminal(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7F:
			// drop C0 controls (incl. ESC, BEL) and DEL
		case r >= 0x80 && r <= 0x9F:
			// drop C1 controls
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
