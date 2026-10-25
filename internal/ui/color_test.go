package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeHex(t *testing.T) {
	cases := map[string]string{
		"#abc":     "#AABBCC",
		"a1b2c3":   "#A1B2C3",
		" #FF00ff": "#FF00FF",
		"#12345":   "",
		"red":      "",
		"#gggggg":  "",
	}
	for in, want := range cases {
		got, ok := normalizeHex(in)
		if want == "" {
			if ok {
				t.Errorf("normalizeHex(%q) unexpectedly valid: %q", in, got)
			}
			continue
		}
		if !ok || got != want {
			t.Errorf("normalizeHex(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestColorMathAndTheme(t *testing.T) {
	if got := contrastFG("#000000"); got != "#FFFFFF" {
		t.Fatalf("contrastFG(black) = %s", got)
	}
	if got := contrastFG("#FFFFFF"); got != "#111827" {
		t.Fatalf("contrastFG(white) = %s", got)
	}
	if got := mixHex("#000000", "#FFFFFF", 0.5); got != "#808080" {
		t.Fatalf("mixHex = %s", got)
	}
	base := NewTheme(true)
	th := buildTheme(true, ThemePrefs{Accent: "#FF0000", Text: "#00FF00"})
	if th.Accent == base.Accent || th.Text == base.Text {
		t.Fatal("custom colors were not applied")
	}
	if th.OnAccent == "" {
		t.Fatal("custom accent has no contrasting foreground")
	}
	if th.Muted == base.Muted || th.Faint == base.Faint {
		t.Fatal("secondary colors were not derived from the custom text color")
	}
	if th.SelBg == base.SelBg {
		t.Fatal("selection background was not derived from the custom accent")
	}
}

func TestThemePrefsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "theme.json")
	prefs := ThemePrefs{Accent: "#60a5fa", Text: "#f1f5f9"}
	if err := saveThemePrefs(path, prefs); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadThemePrefs(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Accent != "#60A5FA" || loaded.Text != "#F1F5F9" {
		t.Fatalf("loaded = %+v", loaded)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"accent\"") || !strings.Contains(string(data), "\"text\"") {
		t.Fatalf("unexpected theme file: %s", data)
	}
}

func TestInvalidThemePrefs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	if err := os.WriteFile(path, []byte(`{"accent":"not-a-color","text":"#123456"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	prefs, err := loadThemePrefs(path)
	if err == nil {
		t.Fatal("expected an invalid-color warning")
	}
	if prefs.Accent != "" || prefs.Text != "#123456" {
		t.Fatalf("prefs = %+v", prefs)
	}
	if _, err := loadThemePrefs(filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Fatalf("missing file should not be an error: %v", err)
	}
}

func TestThemePathEnvOverride(t *testing.T) {
	want := filepath.Join(t.TempDir(), "custom-theme.json")
	t.Setenv("ALX_TUI_CONFIG", want)
	got, err := themeConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("themeConfigPath = %q, want %q", got, want)
	}
}

func TestColorControlHitsAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "theme.json")
	app := testApp(120, 80, PageSettings)
	app.baseDark = true
	app.themePrefs = ThemePrefs{}
	app.themePath = path
	app.applyTheme()
	renderView(t, app)

	accentHits := findHit(app, HitSettingsAccentColor)
	if len(accentHits) == 0 {
		t.Fatal("no accent color swatches registered")
	}
	var blue *Hit
	for i := range accentHits {
		if accentHits[i].Value == "#60A5FA" {
			blue = &accentHits[i]
		}
	}
	if blue == nil {
		t.Fatalf("blue accent swatch missing: %+v", accentHits)
	}
	cmd := app.dispatchHit(*blue, blue.Rect.X+1, blue.Rect.Y)
	if cmd == nil {
		t.Fatal("accent click did not return a save command")
	}
	if got := string(app.th.Accent); got != "#60A5FA" {
		t.Fatalf("accent = %s", got)
	}
	app.Update(cmd())
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("theme file was not written: %v", err)
	}

	// Reload from disk to prove the setting is persisted rather than only
	// applied in memory.
	prefs, err := loadThemePrefs(path)
	if err != nil || prefs.Accent != "#60A5FA" {
		t.Fatalf("persisted prefs = %+v, %v", prefs, err)
	}
	restarted := buildTheme(true, prefs)
	if string(restarted.Accent) != "#60A5FA" {
		t.Fatalf("rebuilt accent = %s", restarted.Accent)
	}

	// Scrolling must not break the color hit boxes: re-render and click the
	// "follow theme" text color option.
	renderView(t, app)
	textHits := findHit(app, HitSettingsTextColor)
	var follow *Hit
	for i := range textHits {
		if textHits[i].Value == "" {
			follow = &textHits[i]
		}
	}
	if follow == nil {
		t.Fatalf("follow-theme text option missing: %+v", textHits)
	}
	cmd = app.dispatchHit(*follow, follow.Rect.X+1, follow.Rect.Y)
	if cmd != nil {
		app.Update(cmd())
	}
	if app.themePrefs.Text != "" {
		t.Fatalf("text pref = %q, want empty", app.themePrefs.Text)
	}

	// Reset clears both preferences and rewrites the file.
	renderView(t, app)
	reset := findHit(app, HitSettingsResetColors)
	if len(reset) == 0 {
		t.Fatal("reset color action missing")
	}
	cmd = app.dispatchHit(reset[0], reset[0].Rect.X+1, reset[0].Rect.Y)
	if cmd == nil {
		t.Fatal("reset did not return a save command")
	}
	app.Update(cmd())
	prefs, err = loadThemePrefs(path)
	if err != nil {
		t.Fatal(err)
	}
	if prefs.Accent != "" || prefs.Text != "" {
		t.Fatalf("prefs after reset = %+v", prefs)
	}
}

func TestCustomColorDialogValidates(t *testing.T) {
	app := testApp(100, 24, PageSettings)
	app.themePath = filepath.Join(t.TempDir(), "theme.json")
	app.baseDark = true
	renderView(t, app)
	if cmd := app.openAccentColorDialog(); cmd == nil {
		t.Fatal("dialog focus command is nil")
	}
	if app.dlg == nil {
		t.Fatal("accent dialog did not open")
	}
	app.dlg.input.SetValue("nope")
	app.submitDialog()
	if app.dlg == nil || app.dlg.err == "" {
		t.Fatal("invalid color should keep the dialog open with an error")
	}
	app.dlg.input.SetValue("#123")
	cmd := app.submitDialog()
	if app.dlg != nil {
		t.Fatal("valid color should close the dialog")
	}
	if cmd == nil {
		t.Fatal("valid color should return a save command")
	}
	msg := cmd()
	if _, ok := msg.(themeSavedMsg); !ok {
		t.Fatalf("unexpected save message %T", msg)
	}
	if app.themePrefs.Accent != "#112233" {
		t.Fatalf("accent = %q, want #112233", app.themePrefs.Accent)
	}
}

func TestWrappedColorSwatchHits(t *testing.T) {
	app := testApp(56, 80, PageSettings)
	app.baseDark = true
	app.themePath = filepath.Join(t.TempDir(), "theme.json")
	app.applyTheme()
	renderView(t, app)

	// At 56 columns the accent row wraps; the pink swatch is on the second
	// line. Its hit box must still be translated to the right row.
	hits := findHit(app, HitSettingsAccentColor)
	var pink *Hit
	for i := range hits {
		if hits[i].Value == "#F472B6" {
			pink = &hits[i]
		}
	}
	if pink == nil {
		t.Fatalf("pink accent swatch missing after wrapping: %+v", hits)
	}
	cmd := app.dispatchHit(*pink, pink.Rect.X+1, pink.Rect.Y)
	if cmd == nil || app.themePrefs.Accent != "#F472B6" {
		t.Fatalf("wrapped swatch click failed: pref=%q cmd=%v", app.themePrefs.Accent, cmd)
	}
}
