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

// SanitizeSingleLine is SanitizeForTerminal plus newline and tab folding, for
// values rendered as one field of a single line (a Subject or From in the
// `mail list` table, or a header line in `mail read`).
//
// SanitizeForTerminal deliberately preserves newlines and tabs, which is right
// for message bodies but not for these fields: an RFC 2047 encoded-word can
// carry a raw newline through envelope decoding, letting a crafted Subject
// close its row and forge additional, entirely fake rows in the output. Tabs
// are folded too because the table is tab-delimited.
func SanitizeSingleLine(s string) string {
	s = SanitizeForTerminal(s)
	if !strings.ContainsAny(s, "\n\t") {
		return s
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

// TruncateRunes shortens s to at most max runes, appending an ellipsis when it
// had to cut. Truncation is rune-aware: slicing a UTF-8 string by bytes can
// split a multi-byte character and emit invalid UTF-8.
func TruncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
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
