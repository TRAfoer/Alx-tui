package ui

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"

	"alxtui/internal/alx"
	tea "github.com/charmbracelet/bubbletea"
)

func portableApp(t *testing.T) (*App, string) {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"ALX_HOME", "ALX_CONFIG", "ALX_DATA", "ALX_CACHE", "ALX_BIN", "LX_CLI_BIN", "ALX_TUI_CONFIG"} {
		t.Setenv(key, "")
	}
	t.Setenv("APPDATA", filepath.Join(dir, "legacy"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "legacy"))
	t.Setenv("ALX_TUI_HOME", filepath.Join(dir, "portable"))
	t.Setenv("ALX_UI_TEST_PROFILE_HELPER", "1")
	log := filepath.Join(dir, "calls.jsonl")
	t.Setenv("ALX_UI_TEST_PROFILE_LOG", log)
	t.Setenv("ALX_UI_TEST_INVALID_CONFIG", "")
	t.Setenv("ALX_UI_TEST_BLOCK_CONFIG", "")
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	name := "Lx-Cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	cliPath := filepath.Join(dir, name)
	if err := os.WriteFile(cliPath, data, 0700); err != nil {
		t.Fatal(err)
	}
	app, err := New(Options{CLIPath: cliPath, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	app.width, app.height, app.page = 80, 24, PageSettings
	return app, log
}

func TestOldBackendResultsAndDeferredBatchAreDiscarded(t *testing.T) {
	app := testApp(80, 24, PageSettings)
	app.ensureBackendLifecycle()
	old := app.backendCommand(func() tea.Msg { return infoMsg{version: "old"} })
	called := false
	batch := app.backendCommand(tea.Batch(func() tea.Msg {
		called = true
		return sourcesMsg{sources: []alx.Source{{ID: "old"}}}
	}, func() tea.Msg { return nil }))
	oldResult := old()
	deferred := batch().(tea.BatchMsg)
	app.lifecycle.epoch.Add(1)
	app.Update(oldResult)
	app.Update(deferred[0]())
	if app.alxVersion != "" || len(app.sources) != 0 || called {
		t.Fatalf("old session applied or ran: version=%q sources=%+v called=%v", app.alxVersion, app.sources, called)
	}
}

func TestBackendEpochPreservesQuitAndScreenCommands(t *testing.T) {
	app := testApp(80, 24, PageSettings)
	_, quit := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("epoch wrapper swallowed Bubble Tea's quit signal")
	}
	clear := app.backendCommand(tea.ClearScreen)
	if _, wrapped := clear().(backendEnvelope); wrapped {
		t.Fatal("epoch wrapper swallowed Bubble Tea's screen control")
	}
}

func TestShutdownCancelsSessionButStillQuitsBackend(t *testing.T) {
	client, logPath := testBackend(t, `{"status":"exited"}`, 0)
	app := &App{client: client}
	app.ensureBackendLifecycle()
	if err := app.Shutdown(); err != nil {
		t.Fatal(err)
	}
	if app.client.SessionContext.Err() == nil {
		t.Fatal("shutdown did not cancel in-flight work")
	}
	if got := testBackendArgs(t, logPath); got[0] != "quit" {
		t.Fatalf("shutdown args = %q", got)
	}
}

func TestThemeMigratesOnceIntoPortableRoot(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ALX_TUI_HOME", filepath.Join(root, "portable"))
	t.Setenv("ALX_TUI_CONFIG", "")
	t.Setenv("APPDATA", filepath.Join(root, "legacy"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "legacy"))
	legacyRoot, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(legacyRoot, "alx-tui", "theme.json")
	if err := saveThemePrefs(legacy, ThemePrefs{Accent: "#60A5FA"}); err != nil {
		t.Fatal(err)
	}
	path, err := themeConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "portable", "theme.json") {
		t.Fatalf("theme path = %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := saveThemePrefs(path, ThemePrefs{Accent: "#F472B6"}); err != nil {
		t.Fatal(err)
	}
	if _, err := themeConfigPath(); err != nil {
		t.Fatal(err)
	}
	prefs, err := loadThemePrefs(path)
	if err != nil || prefs.Accent != "#F472B6" {
		t.Fatalf("existing portable theme overwritten: %+v %v", prefs, err)
	}
}

func deliverBackendBatch(t *testing.T, app *App, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, child := range batch {
			deliverBackendBatch(t, app, child)
		}
		return
	}
	app.Update(msg)
}

func profileCalls(t *testing.T, path string) []profileCall {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var calls []profileCall
	decoder := json.NewDecoder(file)
	for {
		var call profileCall
		if err := decoder.Decode(&call); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		calls = append(calls, call)
	}
	return calls
}

func TestCanceledBackendDoesNotStartFallbackLyricRequest(t *testing.T) {
	var requests atomic.Int32
	withLRCLIBServer(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = w.Write([]byte(`[]`))
	})
	client, _ := testBackend(t, `{}`, 0)
	ctx, cancel := context.WithCancel(context.Background())
	client = client.WithSession(ctx)
	cancel()
	msg := cmdLoadLyrics(client, sampleSong(), 1)().(lyricsMsg)
	if requests.Load() != 0 || !errors.Is(msg.err, context.Canceled) {
		t.Fatalf("canceled session started %d lyric requests, err=%v", requests.Load(), msg.err)
	}
}

func TestNewPreparesPortableProfileAndMigratesTheme(t *testing.T) {
	fixture, _ := portableApp(t)
	legacyRoot, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := saveThemePrefs(filepath.Join(legacyRoot, "alx-tui", "theme.json"), ThemePrefs{Accent: "#60A5FA"}); err != nil {
		t.Fatal(err)
	}
	app, err := New(Options{CLIPath: fixture.client.Bin, Theme: "dark"})
	if err != nil {
		t.Fatal(err)
	}
	if app.profile.Root != fixture.profile.Root || app.themePath != filepath.Join(fixture.profile.Root, "theme.json") {
		t.Fatalf("New did not use portable profile: root=%s theme=%s", app.profile.Root, app.themePath)
	}
	if app.client.ConfigPath != filepath.Join(fixture.profile.Root, "go", "config.toml") || app.client.SessionContext == nil {
		t.Fatalf("New did not prepare backend: %+v %+v", app.profile, app.client)
	}
	if app.themePrefs.Accent != "#60A5FA" || string(app.th.Accent) != "#60A5FA" {
		t.Fatal("New did not load migrated theme")
	}
}
