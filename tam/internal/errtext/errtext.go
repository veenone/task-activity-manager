// Package errtext turns an error into text a person can read. It exists
// because the strings that reach TAM's summaries and banners come straight
// off the wire: a Data Center answering 403 with an HTML login page hands
// the transport a kilobyte of markup, and the UI renders whatever it is
// given.
package errtext

import "strings"

// maxLine is how much of a reason a summary line or a banner can carry
// before it stops being one line. Long enough for a status plus a Jira
// message, short enough to sit in a table cell.
const maxLine = 120

// Line reduces err to a single readable line: the first line of the
// message, with any HTML tags removed, whitespace collapsed, and the
// result capped at maxLine characters with an ellipsis when the cap bites.
// A nil error is the empty string. When the first line is nothing but
// markup, the rest of the message is tried rather than returning nothing.
func Line(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	s := collapse(stripTags(firstLine(msg)))
	if s == "" {
		s = collapse(stripTags(msg))
	}
	return truncate(s, maxLine)
}

// firstLine returns everything up to the first line break, ignoring any
// blank lines the message starts with.
func firstLine(s string) string {
	s = strings.TrimLeft(s, " \t\r\n")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// stripTags drops everything between angle brackets, which is what turns a
// login page into the sentence in front of it.
func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>':
			if depth > 0 {
				depth--
				continue
			}
			b.WriteRune(r)
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// collapse turns every run of whitespace into one space and trims the ends.
func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// truncate caps s at n characters, counting runes, and marks the cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
