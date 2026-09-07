package errtext_test

import (
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/errtext"
)

func TestLineKeepsAShortMessageAsItIs(t *testing.T) {
	got := errtext.Line(errors.New("jira: 403 Forbidden: Login required"))
	if got != "jira: 403 Forbidden: Login required" {
		t.Errorf("line = %q", got)
	}
}

func TestLineReducesAnHTMLLoginPageToOneLine(t *testing.T) {
	err := errors.New("jira: 403 Forbidden: <!DOCTYPE html>\n<html>\n<head><title>Log in</title></head>\n<body>\n  <form action=\"/login.jsp\">\n</body>\n</html>")
	got := errtext.Line(err)
	if strings.ContainsAny(got, "<>\n") {
		t.Errorf("line = %q, want no markup and no line break", got)
	}
	if got != "jira: 403 Forbidden:" {
		t.Errorf("line = %q, want the sentence in front of the page", got)
	}
}

func TestLineFallsBackWhenTheFirstLineIsAllMarkup(t *testing.T) {
	got := errtext.Line(errors.New("<!DOCTYPE html>\n<body>Login required</body>"))
	if got != "Login required" {
		t.Errorf("line = %q, want the text below the markup", got)
	}
}

func TestLineCapsALongMessage(t *testing.T) {
	got := errtext.Line(errors.New(strings.Repeat("a", 400)))
	if len([]rune(got)) != 120 {
		t.Errorf("line is %d characters, want the 120 cap", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("line = %q, want the cut marked", got)
	}
}

func TestLineOfNilIsEmpty(t *testing.T) {
	if got := errtext.Line(nil); got != "" {
		t.Errorf("line = %q, want empty", got)
	}
}
