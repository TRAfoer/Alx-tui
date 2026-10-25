package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestOverlayWidth(t *testing.T) {
	base := []string{strings.Repeat("a", 120)}
	popup := []string{"[" + strings.Repeat("-", 33) + "]"}
	out := overlayLines(base, popup, 20, 0, 120)
	t.Logf("line width=%d len=%d", ansi.StringWidth(out[0]), len(out[0]))
	if w := ansi.StringWidth(out[0]); w != 120 {
		t.Fatalf("width=%d", w)
	}
}

func TestPrettyTextSanitizes(t *testing.T) {
	in := "A\x1b[31mB\x00C&nbsp;D\tE"
	got := prettyText(in)
	if got != "ABC D E" {
		t.Fatalf("prettyText = %q", got)
	}
	if strings.ContainsAny(prettyText("x\x1b]0;pwned\x07y"), "\x1b\x07") {
		t.Fatal("control sequences survived sanitization")
	}
}
