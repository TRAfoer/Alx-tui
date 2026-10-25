package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Theme holds the adaptive color palette used across the TUI.
type Theme struct {
	Dark         bool
	Accent       lipgloss.Color
	AccentStrong lipgloss.Color
	OnAccent     lipgloss.Color
	Accent2      lipgloss.Color
	Text         lipgloss.Color
	Muted        lipgloss.Color
	Faint        lipgloss.Color
	Border       lipgloss.Color
	PanelBg      lipgloss.Color
	InputBg      lipgloss.Color
	PanelFg      lipgloss.Color
	SelBg        lipgloss.Color
	SelFg        lipgloss.Color
	Good         lipgloss.Color
	Bad          lipgloss.Color
	Warn         lipgloss.Color
	White        lipgloss.Color
	Black        lipgloss.Color
}

func NewTheme(dark bool) Theme {
	if dark {
		return Theme{
			Dark:         true,
			Accent:       lipgloss.Color("#A78BFA"),
			AccentStrong: lipgloss.Color("#7C3AED"),
			OnAccent:     lipgloss.Color("#FFFFFF"),
			Accent2:      lipgloss.Color("#67E8F9"),
			Text:         lipgloss.Color("#E5E7EB"),
			Muted:        lipgloss.Color("#A5B1C2"),
			Faint:        lipgloss.Color("#7E8CA0"),
			Border:       lipgloss.Color("#3B4A61"),
			PanelBg:      lipgloss.Color("#0F172A"),
			InputBg:      lipgloss.Color("#16213C"),
			PanelFg:      lipgloss.Color("#E2E8F0"),
			SelBg:        lipgloss.Color("#312E81"),
			SelFg:        lipgloss.Color("#EEF2FF"),
			Good:         lipgloss.Color("#34D399"),
			Bad:          lipgloss.Color("#FB7185"),
			Warn:         lipgloss.Color("#FBBF24"),
			White:        lipgloss.Color("#FFFFFF"),
			Black:        lipgloss.Color("#0B1020"),
		}
	}
	return Theme{
		Dark:         false,
		Accent:       lipgloss.Color("#6D28D9"),
		AccentStrong: lipgloss.Color("#5B21B6"),
		OnAccent:     lipgloss.Color("#FFFFFF"),
		Accent2:      lipgloss.Color("#0E7490"),
		Text:         lipgloss.Color("#1F2937"),
		Muted:        lipgloss.Color("#4B5563"),
		Faint:        lipgloss.Color("#6B7280"),
		Border:       lipgloss.Color("#CBD5E1"),
		PanelBg:      lipgloss.Color("#FFFFFF"),
		InputBg:      lipgloss.Color("#F1F5F9"),
		PanelFg:      lipgloss.Color("#111827"),
		SelBg:        lipgloss.Color("#EDE9FE"),
		SelFg:        lipgloss.Color("#4C1D95"),
		Good:         lipgloss.Color("#047857"),
		Bad:          lipgloss.Color("#BE123C"),
		Warn:         lipgloss.Color("#B45309"),
		White:        lipgloss.Color("#FFFFFF"),
		Black:        lipgloss.Color("#111827"),
	}
}

func (t Theme) Style(c lipgloss.Color) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(c)
}

func (t Theme) Bold(c lipgloss.Color, s string) string {
	return lipgloss.NewStyle().Foreground(c).Bold(true).Render(s)
}

func (t Theme) Dim(s string) string        { return t.Style(t.Muted).Render(s) }
func (t Theme) FaintText(s string) string  { return t.Style(t.Faint).Render(s) }
func (t Theme) MutedText(s string) string  { return t.Style(t.Muted).Render(s) }
func (t Theme) TextPlain(s string) string  { return t.Style(t.Text).Render(s) }
func (t Theme) AccentText(s string) string { return t.Style(t.Accent).Render(s) }
func (t Theme) GoodText(s string) string   { return t.Style(t.Good).Render(s) }
func (t Theme) BadText(s string) string    { return t.Style(t.Bad).Render(s) }
func (t Theme) WarnText(s string) string   { return t.Style(t.Warn).Render(s) }
func (t Theme) Brand(s string) string {
	return lipgloss.NewStyle().Foreground(t.Accent).Bold(true).Render(s)
}

// ActiveChip renders a selectable pill.
func (t Theme) ActiveChip(label string, active bool) string {
	// The leading marker keeps the active choice visible even when the
	// terminal (or NO_COLOR) suppresses colors.
	if active {
		return lipgloss.NewStyle().
			Background(t.AccentStrong).
			Foreground(t.OnAccent).
			Bold(true).
			Padding(0, 1).
			Render("▸" + label)
	}
	return lipgloss.NewStyle().
		Foreground(t.Muted).
		Padding(0, 1).
		Render(" " + label)
}

// Chip renders a chip with a background suitable for menus/dialogs.
func (t Theme) Chip(label string, bg, fg lipgloss.Color) string {
	return lipgloss.NewStyle().Background(bg).Foreground(fg).Padding(0, 1).Render(label)
}

// DangerChip renders a destructive action.
func (t Theme) DangerChip(label string) string {
	return lipgloss.NewStyle().Foreground(t.Bad).Bold(true).Padding(0, 1).Render(label)
}

// Button renders an action button on the panel background.
func (t Theme) Button(label string, primary bool) string {
	if primary {
		return lipgloss.NewStyle().Background(t.AccentStrong).Foreground(t.OnAccent).Bold(true).Padding(0, 1).Render(label)
	}
	return lipgloss.NewStyle().Foreground(t.Muted).Background(t.PanelBg).Padding(0, 1).Render(label)
}

func (t Theme) Sep(width int) string {
	if width <= 0 {
		return ""
	}
	return t.Style(t.Border).Render(strings.Repeat("─", width))
}

func (t Theme) Badge(label string) string {
	return lipgloss.NewStyle().Background(t.AccentStrong).Foreground(t.OnAccent).Bold(true).Padding(0, 1).Render(strings.ToUpper(label))
}

func (t Theme) SoftBadge(label string) string {
	return lipgloss.NewStyle().Foreground(t.Accent2).Render(label)
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (t Theme) Spinner(frame int) string {
	if len(spinnerFrames) == 0 {
		return ""
	}
	return t.Style(t.Accent2).Render(spinnerFrames[frame%len(spinnerFrames)])
}

var sourceNames = map[string]string{
	"all":   "全部",
	"wy":    "网易云",
	"tx":    "QQ音乐",
	"kg":    "酷狗",
	"kw":    "酷我",
	"mg":    "咪咕",
	"qsvip": "全民K歌",
}

func sourceLabel(code string) string {
	code = prettyText(code)
	if v, ok := sourceNames[strings.ToLower(code)]; ok {
		return v
	}
	if code == "" {
		return "未知"
	}
	return strings.ToUpper(code)
}

func sourceCodeLabel(code string) string {
	code = prettyText(code)
	if code == "" {
		return "--"
	}
	return strings.ToUpper(code)
}

func sourceAccent(t Theme, code string) lipgloss.Color {
	switch strings.ToLower(code) {
	case "wy":
		return t.Good
	case "tx":
		return t.Accent2
	case "kw":
		return t.Warn
	case "kg":
		return t.Accent
	case "mg":
		return t.Accent
	default:
		return t.Faint
	}
}

// FrameTop renders a rounded top border, optionally with a highlighted title.
func (t Theme) FrameTop(title string, width int, color lipgloss.Color) string {
	if width < 2 {
		return ""
	}
	if title == "" {
		return t.Style(color).Render("╭" + strings.Repeat("─", width-2) + "╮")
	}
	title = truncate(title, max(0, width-5))
	fill := width - 5 - displayWidth(title)
	if fill < 0 {
		fill = 0
	}
	return t.Style(color).Render("╭─ ") +
		t.Bold(t.Accent, title) +
		t.Style(color).Render(" "+strings.Repeat("─", fill)+"╮")
}

// FrameLine renders a bordered content line. The content area is width-4.
func (t Theme) FrameLine(content string, width int, color lipgloss.Color) string {
	if width < 2 {
		return ""
	}
	inner := max(0, width-4)
	return t.Style(color).Render("│") + " " + padVisible(content, inner) + " " + t.Style(color).Render("│")
}

// FrameBottom renders a rounded bottom border.
func (t Theme) FrameBottom(width int, color lipgloss.Color) string {
	if width < 2 {
		return ""
	}
	return t.Style(color).Render("╰" + strings.Repeat("─", width-2) + "╯")
}
