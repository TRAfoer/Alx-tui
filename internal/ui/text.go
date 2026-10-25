package ui

import (
	"html"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"
)

const ellipsis = "…"

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

func displayWidth(s string) int { return ansi.StringWidth(s) }

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if displayWidth(s) <= width {
		return s
	}
	if width == 1 {
		return ellipsis
	}
	return ansi.Truncate(s, width, ellipsis)
}

func padVisible(s string, width int) string {
	if width <= 0 {
		return ""
	}
	s = truncate(s, width)
	if gap := width - displayWidth(s); gap > 0 {
		s += spaces(gap)
	}
	return s
}

func centerText(s string, width int) string {
	s = truncate(s, width)
	gap := width - displayWidth(s)
	if gap <= 0 {
		return s
	}
	left := gap / 2
	return spaces(left) + s + spaces(gap-left)
}

// composeLR places right at the right edge of a width-wide line.
func composeLR(left, right string, width int) string {
	if width <= 0 {
		return ""
	}
	rw := displayWidth(right)
	if rw >= width {
		return truncate(right, width)
	}
	lw := displayWidth(left)
	avail := width - rw
	if lw > avail {
		if avail <= 2 {
			return truncate(right, width)
		}
		left = truncate(left, avail-1) + " "
		lw = displayWidth(left)
	}
	gap := width - lw - rw
	if gap < 0 {
		gap = 0
	}
	return left + spaces(gap) + right
}

// normalizeLines clips/pads a view to an exact height and width.
func normalizeLines(lines []string, height, width int) []string {
	if height < 0 {
		height = 0
	}
	out := make([]string, height)
	for i := 0; i < height; i++ {
		if i < len(lines) {
			out[i] = padVisible(lines[i], width)
		} else {
			out[i] = spaces(width)
		}
	}
	return out
}

// overlayLines draws popup on top of base at terminal cell coordinates.
func overlayLines(base []string, popup []string, x, y, screenWidth int) []string {
	if len(popup) == 0 {
		return base
	}
	out := append([]string(nil), base...)
	pw := 0
	for _, line := range popup {
		if w := displayWidth(line); w > pw {
			pw = w
		}
	}
	if pw == 0 {
		return out
	}
	if x < 0 {
		x = 0
	}
	if x+pw > screenWidth {
		x = screenWidth - pw
		if x < 0 {
			x = 0
		}
	}
	for i, raw := range popup {
		row := y + i
		if row < 0 || row >= len(out) {
			continue
		}
		line := padVisible(raw, pw)
		left := ansi.Truncate(out[row], x, "")
		if lw := displayWidth(left); lw < x {
			left += spaces(x - lw)
		}
		right := ansi.TruncateLeft(out[row], x+pw, "")
		merged := left + "\x1b[0m" + line + "\x1b[0m" + right
		// ANSI-aware truncation can add a padding cell when the cut lands in
		// the middle of a wide glyph. Never let the overlay grow the line past
		// the terminal width; a single extra cell makes the terminal wrap and
		// scroll the whole TUI.
		out[row] = padVisible(ansi.Truncate(merged, screenWidth, ""), screenWidth)
	}
	return out
}

// wrapText wraps plain text (including CJK) to a cell width.
func wrapText(s string, width int) []string {
	if width <= 0 {
		return nil
	}
	var out []string
	for _, paragraph := range strings.Split(strings.ReplaceAll(s, "\t", "    "), "\n") {
		if paragraph == "" {
			out = append(out, "")
			continue
		}
		var b strings.Builder
		n := 0
		for _, r := range paragraph {
			rw := runewidth.RuneWidth(r)
			if rw < 0 {
				rw = 0
			}
			if n+rw > width && n > 0 {
				out = append(out, b.String())
				b.Reset()
				n = 0
			}
			if rw > width {
				continue
			}
			b.WriteRune(r)
			n += rw
		}
		if b.Len() > 0 {
			out = append(out, b.String())
		}
	}
	if len(out) == 0 {
		out = []string{""}
	}
	return out
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ensureVisible adjusts a scroll offset so selected is inside a window.
func ensureVisible(selected int, offset *int, height, total int) {
	if height <= 0 || total <= 0 {
		*offset = 0
		return
	}
	if selected < *offset {
		*offset = selected
	}
	if selected >= *offset+height {
		*offset = selected - height + 1
	}
	clampScroll(offset, total, height)
}

func clampScroll(offset *int, total, height int) {
	maxOff := total - height
	if maxOff < 0 {
		maxOff = 0
	}
	*offset = clamp(*offset, 0, maxOff)
}

func windowBounds(total, offset, height int) (int, int) {
	if height <= 0 || total <= 0 {
		return 0, 0
	}
	start := clamp(offset, 0, max(0, total-height))
	end := min(total, start+height)
	return start, end
}

func (a *App) fakeCursor() string {
	if a.clock.IsZero() {
		return a.th.AccentText("▏")
	}
	if (a.clock.UnixNano()/int64(500*time.Millisecond))%2 == 0 {
		return a.th.AccentText("▏")
	}
	return " "
}

// prettyText decodes HTML entities that some sources include in metadata
// (e.g. "J&nbsp;III"), normalizes non-breaking spaces, and removes terminal
// control sequences from untrusted metadata before it is rendered.
func prettyText(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\u00a0", " ")
	s = ansi.Strip(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			return ' '
		case r < 0x20 || r == 0x7f:
			return -1
		default:
			return r
		}
	}, s)
	return strings.TrimSpace(s)
}
