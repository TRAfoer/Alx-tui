package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"alxtui/internal/alx"
)

// actionButton is a small inline clickable action used in page headers.
type actionButton struct {
	label    string
	kind     HitKind
	value    string
	index    int
	primary  bool
	danger   bool
	disabled bool
}

func buttonWidths(specs []actionButton) (total int) {
	for i, s := range specs {
		total += displayWidth(s.label) + 2
		if i < len(specs)-1 {
			total++
		}
	}
	return total
}

// renderButtons draws action buttons at absolute (x,y), records hits, and
// returns the rendered string.
func (a *App) renderButtons(x, y int, specs []actionButton) string {
	t := a.th
	var b strings.Builder
	cursor := x
	for i, s := range specs {
		style := lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 1)
		switch {
		case s.disabled:
			style = lipgloss.NewStyle().Foreground(t.Faint).Padding(0, 1)
		case s.primary:
			style = lipgloss.NewStyle().Background(t.AccentStrong).Foreground(t.OnAccent).Bold(true).Padding(0, 1)
		case s.danger:
			style = lipgloss.NewStyle().Foreground(t.Bad).Padding(0, 1)
		}
		rendered := style.Render(s.label)
		w := displayWidth(rendered)
		b.WriteString(rendered)
		if !s.disabled {
			a.addHit(s.kind, Rect{X: cursor, Y: y, W: w, H: 1}, s.value, s.index, alx.Song{})
		}
		cursor += w
		if i < len(specs)-1 {
			b.WriteString(" ")
			cursor++
		}
	}
	return b.String()
}
