package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSettingsUsesSingleCLIWithoutBackendSwitchControls(t *testing.T) {
	app := testApp(56, 16, PageSettings)
	app.alxVersion = "0.1.1-go"
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnd})
	if cmd != nil {
		t.Fatal("End unexpectedly started a backend command")
	}
	view := strings.Join(renderView(t, app), "\n")
	for _, forbidden := range []string{"选择 [b]", "切换后端", "Rust", "alx-go.exe"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("obsolete backend control %q remains: %s", forbidden, view)
		}
	}
	if !strings.Contains(view, "Lx-Cli") {
		t.Fatalf("single CLI identity missing: %s", view)
	}
	_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	if cmd != nil {
		t.Fatal("settings b started an obsolete backend switch")
	}
}
