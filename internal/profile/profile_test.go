package profile

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// The child is a real executable so Prepare's argument, environment and failure
// handling are exercised without opening a player or a user's library.
func TestMain(m *testing.M) {
	if os.Getenv("PROFILE_TEST_BACKEND") == "1" {
		args := os.Args[1:]
		if len(args) == 1 && args[0] == "--version" {
			fmt.Println(os.Getenv("PROFILE_TEST_VERSION"))
			os.Exit(0)
		}
		if len(args) >= 2 && args[0] == "--config" {
			if marker := os.Getenv("PROFILE_TEST_CONFIG_MARKER"); marker != "" {
				_ = os.WriteFile(marker, []byte("config executed"), 0600)
			}
			if os.Getenv("PROFILE_TEST_INVALID") == "1" {
				fmt.Fprintln(os.Stderr, "invalid configuration")
				os.Exit(1)
			}
			if len(args) < 4 || args[2] != "config" || args[3] != "show" {
				os.Exit(2)
			}
			if os.Getenv("ALX_HOME") != "" || os.Getenv("ALX_CONFIG") != args[1] {
				fmt.Fprintln(os.Stderr, "incorrect portable environment")
				os.Exit(3)
			}
			for _, p := range []string{filepath.Dir(args[1]), os.Getenv("ALX_DATA"), os.Getenv("ALX_CACHE")} {
				if err := os.MkdirAll(p, 0700); err != nil {
					os.Exit(4)
				}
			}
			if _, err := os.Stat(args[1]); os.IsNotExist(err) {
				_ = os.WriteFile(args[1], []byte("[source]\n"), 0600)
			}
			fmt.Println(`{}`)
			os.Exit(0)
		}
		os.Exit(5)
	}
	os.Exit(m.Run())
}

func portableTest(t *testing.T) (*Manager, string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ALX_TUI_HOME", filepath.Join(dir, "portable"))
	t.Setenv("ALX_BIN", "")
	t.Setenv("LX_CLI_BIN", "")
	t.Setenv("ALX_CONFIG", "")
	t.Setenv("ALX_DATA", "")
	t.Setenv("ALX_CACHE", "")
	t.Setenv("ALX_HOME", filepath.Join(dir, "legacy"))
	t.Setenv("PROFILE_TEST_BACKEND", "1")
	t.Setenv("PROFILE_TEST_VERSION", "Lx-Cli 0.1.1-go")
	t.Setenv("PROFILE_TEST_INVALID", "")
	t.Setenv("PROFILE_TEST_CONFIG_MARKER", "")
	m, err := New(filepath.Join(dir, "tui.exe"))
	if err != nil {
		t.Fatal(err)
	}
	return m, dir
}

func backendFile(t *testing.T, dir, name string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	p := filepath.Join(dir, name)
	src, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0700); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSelectionUsesOnlyBundledLxCliAndIgnoresOldPreference(t *testing.T) {
	m, dir := portableTest(t)
	rust := backendFile(t, dir, "alx")
	backendFile(t, dir, "alx-go")
	cliPath := backendFile(t, dir, "Lx-Cli")
	list, err := m.Discover()
	if err != nil || len(list) != 1 || list[0].Path != cliPath {
		t.Fatalf("Discover = %v, %v", list, err)
	}
	if err := os.MkdirAll(m.Root, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ALX_BIN", rust)
	for _, oldPreference := range []string{`{"backend":"alx.exe"}`, `invalid old preference`} {
		if err := os.WriteFile(filepath.Join(m.Root, "tui.json"), []byte(oldPreference), 0600); err != nil {
			t.Fatal(err)
		}
		got, err := m.Selected("")
		if err != nil || got.Path != cliPath {
			t.Fatalf("default = %+v, %v; want %s", got, err, cliPath)
		}
	}
	if err := os.Remove(cliPath); err != nil {
		t.Fatal(err)
	}
	if got, err := m.Selected(""); err == nil || got.Path != "" {
		t.Fatalf("fell back to old backend: %+v, %v", got, err)
	}
}

func TestSelectionExplicitAndLxCliEnvironment(t *testing.T) {
	m, dir := portableTest(t)
	backendFile(t, dir, "Lx-Cli")
	envPath := backendFile(t, t.TempDir(), "external-cli")
	explicitPath := backendFile(t, t.TempDir(), "chosen-cli")
	t.Setenv("LX_CLI_BIN", envPath)
	got, err := m.Selected("")
	if err != nil || got.Path != envPath {
		t.Fatalf("env = %+v, %v", got, err)
	}
	got, err = m.Selected(explicitPath)
	if err != nil || got.Path != explicitPath {
		t.Fatalf("explicit = %+v, %v", got, err)
	}
	if _, err := m.Selected(filepath.Join(dir, "missing.exe")); err == nil {
		t.Fatal("invalid explicit override fell back")
	}
	t.Setenv("LX_CLI_BIN", filepath.Join(dir, "missing-env.exe"))
	if _, err := m.Selected(""); err == nil {
		t.Fatal("invalid env override fell back")
	}
}

func TestPortableLayoutAndRenamedGo(t *testing.T) {
	m, dir := portableTest(t)
	p := backendFile(t, dir, "Lx-Cli")
	prepared, err := m.Prepare(context.Background(), Candidate{Name: filepath.Base(p), Path: p, Kind: "rust"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if prepared.Backend.Kind != "go" || prepared.Backend.Version != "0.1.1-go" {
		t.Fatalf("backend = %+v", prepared.Backend)
	}
	if prepared.Client.ConfigPath != filepath.Join(m.Root, "go", "config.toml") {
		t.Fatalf("config = %q", prepared.Client.ConfigPath)
	}
	if got := m.ThemePath(); got != filepath.Join(m.Root, "theme.json") {
		t.Fatalf("theme = %q", got)
	}
	for _, entry := range prepared.Client.Env {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "ALX_HOME") {
			t.Fatal("inherited ALX_HOME")
		}
		if strings.EqualFold(key, "ALX_DATA") && value != filepath.Join(m.Root, "go", "data") {
			t.Fatalf("data = %q", value)
		}
		if strings.EqualFold(key, "ALX_CACHE") && value != filepath.Join(m.Root, "go", "cache") {
			t.Fatalf("cache = %q", value)
		}
	}
	override := filepath.Join(dir, "explicit", "config.toml")
	second, err := m.Prepare(context.Background(), Candidate{Path: p}, override)
	if err != nil {
		t.Fatal(err)
	}
	if second.Client.ConfigPath != override {
		t.Fatalf("override = %q", second.Client.ConfigPath)
	}
}

func TestPrepareRejectsRustBeforeConfigOrMigration(t *testing.T) {
	for _, version := range []string{"alx 0.4.1", "0.4.1", "Lx-Cli 0.1.1", "unrelated-program 0.1.1-go", "alx 0.4.1-google"} {
		t.Run(version, func(t *testing.T) {
			m, dir := portableTest(t)
			p := backendFile(t, dir, "explicit-rust")
			t.Setenv("PROFILE_TEST_VERSION", version)
			marker := filepath.Join(dir, "config-command-ran")
			t.Setenv("PROFILE_TEST_CONFIG_MARKER", marker)
			chosen, err := m.Selected(p)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := m.Prepare(context.Background(), chosen, ""); err == nil {
				t.Fatal("accepted non-Go backend")
			}
			for _, path := range []string{m.Root, marker} {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("rejected backend initialized %s: %v", path, err)
				}
			}
		})
	}
}

func TestPrepareAcceptsLegacyGoVersion(t *testing.T) {
	m, dir := portableTest(t)
	p := backendFile(t, dir, "renamed-old-go")
	t.Setenv("PROFILE_TEST_VERSION", "alx 0.1.0-go")
	got, err := m.Prepare(context.Background(), Candidate{Path: p}, "")
	if err != nil || got.Backend.Kind != "go" || got.Backend.Version != "0.1.0-go" {
		t.Fatalf("legacy Go = %+v, %v", got.Backend, err)
	}
}

func TestRootDefaultAvoidsUnixExecutableConflict(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALX_TUI_HOME", "")
	if err := os.WriteFile(filepath.Join(dir, "alx"), []byte("backend"), 0700); err != nil {
		t.Fatal(err)
	}
	m, err := New(filepath.Join(dir, "tui"))
	if err != nil {
		t.Fatal(err)
	}
	want := "alx"
	if runtime.GOOS != "windows" {
		want = "alx-config"
	}
	if m.Root != filepath.Join(dir, want) {
		t.Fatalf("root = %q", m.Root)
	}
}

func TestMigrationSnapshotsActiveWALAndRewritesOnlyInternalSources(t *testing.T) {
	m, dir := portableTest(t)
	p := backendFile(t, dir, "Lx-Cli")
	legacy := os.Getenv("ALX_HOME")
	sources := filepath.Join(legacy, "data", "sources")
	if err := os.MkdirAll(sources, 0700); err != nil {
		t.Fatal(err)
	}
	config := "[download]\noutput_dir = 'D:/external/music'\n"
	if err := os.WriteFile(filepath.Join(legacy, "config.toml"), []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	internal, external := filepath.Join(sources, "one.js"), filepath.Join(dir, "outside.js")
	for path, content := range map[string]string{
		internal: "// source", external: "// external",
		filepath.Join(sources, "daemon.lock"):        "runtime",
		filepath.Join(sources, "lock"):               "runtime",
		filepath.Join(legacy, "data", "daemon.json"): "runtime",
	} {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	dbPath := filepath.Join(legacy, "data", "alx-go.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{"PRAGMA journal_mode=WAL", "PRAGMA wal_autocheckpoint=0", "CREATE TABLE music(path TEXT)", "INSERT INTO music VALUES ('D:/music/keep.flac')", "CREATE TABLE sources(id TEXT PRIMARY KEY, payload TEXT NOT NULL)"} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	for i, path := range []string{internal, external} {
		payload, _ := json.Marshal(map[string]any{"script_path": path, "name": "source", "url": "https://example.com"})
		if _, err := db.Exec("INSERT INTO sources VALUES (?, ?)", fmt.Sprint(i), string(payload)); err != nil {
			t.Fatal(err)
		}
	}
	if info, err := os.Stat(dbPath + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("WAL fixture = %v, %v", info, err)
	}
	got, err := m.Prepare(context.Background(), Candidate{Path: p}, "")
	if err != nil || !got.Migrated {
		t.Fatalf("migration = %+v, %v", got, err)
	}
	target := filepath.Join(m.Root, "go")
	data, err := os.ReadFile(filepath.Join(target, "config.toml"))
	if err != nil || string(data) != config {
		t.Fatalf("config changed: %q, %v", data, err)
	}
	nextDB, err := sql.Open("sqlite", filepath.Join(target, "data", "alx-go.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer nextDB.Close()
	var music, path string
	if err := nextDB.QueryRow("SELECT path FROM music").Scan(&music); err != nil || music != "D:/music/keep.flac" {
		t.Fatalf("music = %q, %v", music, err)
	}
	for i, want := range []string{filepath.Join(target, "data", "sources", "one.js"), external} {
		if err := nextDB.QueryRow("SELECT json_extract(payload,'$.script_path') FROM sources WHERE id=?", fmt.Sprint(i)).Scan(&path); err != nil || path != want {
			t.Fatalf("source %d = %q want %q: %v", i, path, want, err)
		}
	}
	for _, rel := range []string{"data/daemon.json", "data/sources/daemon.lock", "data/sources/lock"} {
		if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("runtime file copied: %s", rel)
		}
	}
	for _, path := range []string{internal, external, dbPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal("legacy file removed", path, err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "config.toml"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}
	second, err := m.Prepare(context.Background(), Candidate{Path: p}, "")
	if err != nil || second.Migrated {
		t.Fatalf("repeat migration = %+v, %v", second, err)
	}
	data, _ = os.ReadFile(filepath.Join(target, "config.toml"))
	if string(data) != "existing" {
		t.Fatal("overwrote existing target")
	}
}

func TestExistingGoAndRustFilesRemainUntouched(t *testing.T) {
	m, dir := portableTest(t)
	p := backendFile(t, dir, "Lx-Cli")
	files := map[string]string{
		"go/config.toml":              "existing config",
		"go/data/alx-go.db":           "existing playlists",
		"go/data/sources/custom.js":   "existing script",
		"go/downloads/local.flac":     "existing download",
		"rust/config.toml":            "old rust config",
		"rust/data/agent-lx-music.db": "old rust playlists",
		"rust/data/sources/rust.js":   "old rust source",
	}
	for rel, content := range files {
		path := filepath.Join(m.Root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := m.Prepare(context.Background(), Candidate{Path: p}, "")
	if err != nil || got.Migrated || got.Backend.Kind != "go" {
		t.Fatalf("existing profile = %+v, %v", got, err)
	}
	for rel, want := range files {
		content, err := os.ReadFile(filepath.Join(m.Root, filepath.FromSlash(rel)))
		if err != nil || string(content) != want {
			t.Fatalf("changed %s = %q, %v", rel, content, err)
		}
	}
}

func TestMigrationFailureDoesNotPublishAndInvalidConfigRollsBack(t *testing.T) {
	for _, failure := range []string{"database", "config"} {
		t.Run(failure, func(t *testing.T) {
			m, dir := portableTest(t)
			p := backendFile(t, dir, "Lx-Cli")
			legacy := os.Getenv("ALX_HOME")
			if err := os.MkdirAll(filepath.Join(legacy, "data"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(legacy, "config.toml"), []byte("legacy"), 0600); err != nil {
				t.Fatal(err)
			}
			if failure == "database" {
				if err := os.WriteFile(filepath.Join(legacy, "data", "alx-go.db"), []byte("corrupt sqlite"), 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				t.Setenv("PROFILE_TEST_INVALID", "1")
			}
			if _, err := m.Prepare(context.Background(), Candidate{Path: p}, ""); err == nil {
				t.Fatal("invalid migration succeeded")
			}
			if _, err := os.Stat(filepath.Join(m.Root, "go")); !os.IsNotExist(err) {
				t.Fatalf("published failed migration: %v", err)
			}
			entries, err := os.ReadDir(m.Root)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			for _, e := range entries {
				if strings.Contains(e.Name(), "stage") {
					t.Fatalf("leaked staging %s", e.Name())
				}
			}
		})
	}
}

func TestEnvironmentRemovesEveryCaseVariant(t *testing.T) {
	env := portableEnv([]string{"PATH=tools", "ALX_HOME=old", "alx_home=older", "Alx_Config=old.toml", "ALX_CONFIG=other.toml", "alx_data=data", "ALX_CACHE=cache", "HOME=home"}, "config.toml", "portable-data", "portable-cache")
	want := map[string]string{"ALX_CONFIG": "config.toml", "ALX_DATA": "portable-data", "ALX_CACHE": "portable-cache"}
	seen := map[string]int{}
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(key)
		if upper == "ALX_HOME" {
			t.Fatal("retained ALX_HOME")
		}
		if expected, ok := want[upper]; ok {
			seen[upper]++
			if key != upper || value != expected {
				t.Fatalf("environment %q", entry)
			}
		}
	}
	for key := range want {
		if seen[key] != 1 {
			t.Fatalf("%s count = %d", key, seen[key])
		}
	}
	if !strings.Contains(strings.Join(env, "\n"), "HOME=home") {
		t.Fatal("removed unrelated environment")
	}
}

func TestExistingProfileSkipsEvenCorruptLegacyDatabase(t *testing.T) {
	m, dir := portableTest(t)
	p := backendFile(t, dir, "Lx-Cli")
	if err := os.MkdirAll(filepath.Join(m.Root, "go"), 0700); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(os.Getenv("ALX_HOME"), "data")
	if err := os.MkdirAll(legacy, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "alx-go.db"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := m.Prepare(context.Background(), Candidate{Path: p}, "")
	if err != nil || got.Migrated {
		t.Fatalf("existing profile = %+v, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(m.Root, "go", "data", "alx-go.db")); !os.IsNotExist(err) {
		t.Fatalf("copied legacy database: %v", err)
	}
}

func TestPortableRootIsNeverReimportedAsLegacy(t *testing.T) {
	m, dir := portableTest(t)
	p := backendFile(t, dir, "Lx-Cli")
	t.Setenv("ALX_HOME", m.Root)
	if err := os.MkdirAll(filepath.Join(m.Root, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.Root, "config.toml"), []byte("must not import"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(m.Root, "data", "alx-go.db"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := m.Prepare(context.Background(), Candidate{Path: p}, "")
	if err != nil || got.Migrated {
		t.Fatalf("self migration = %+v, %v", got, err)
	}
	data, err := os.ReadFile(filepath.Join(m.Root, "go", "config.toml"))
	if err != nil || string(data) == "must not import" {
		t.Fatalf("self-imported config: %q %v", data, err)
	}
}

func TestPortableRootNestedInLegacySourcesDoesNotCopyItself(t *testing.T) {
	m, dir := portableTest(t)
	p := backendFile(t, dir, "Lx-Cli")
	oldSources := filepath.Join(os.Getenv("ALX_HOME"), "data", "sources")
	if err := os.MkdirAll(oldSources, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldSources, "one.js"), []byte("// source"), 0600); err != nil {
		t.Fatal(err)
	}
	m.Root = filepath.Join(oldSources, "portable")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := m.Prepare(ctx, Candidate{Path: p}, ""); err != nil {
		t.Fatal(err)
	}
	sources := filepath.Join(m.Root, "go", "data", "sources")
	entries, err := os.ReadDir(sources)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "one.js" {
		t.Fatalf("copied portable destination into itself: %v", entries)
	}
}

func TestInvalidVersionAndCanceledPreparationNeverInitialize(t *testing.T) {
	for _, invalid := range []string{"version", "canceled"} {
		t.Run(invalid, func(t *testing.T) {
			m, dir := portableTest(t)
			p := backendFile(t, dir, "Lx-Cli")
			ctx := context.Background()
			if invalid == "version" {
				t.Setenv("PROFILE_TEST_VERSION", "unrelated-program")
			} else {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			if _, err := m.Prepare(ctx, Candidate{Path: p}, ""); err == nil {
				t.Fatal("invalid preparation succeeded")
			}
			if _, err := os.Stat(m.Root); !os.IsNotExist(err) {
				t.Fatalf("initialized rejected backend: %v", err)
			}
		})
	}
}

// Run explicitly with PROFILE_REAL_BACKEND set to a built backend. Every path
// passed to it is temporary; Config and Sources cannot start playback.
func TestRealBackendPortablePaths(t *testing.T) {
	path := os.Getenv("PROFILE_REAL_BACKEND")
	if path == "" {
		t.Skip("PROFILE_REAL_BACKEND is not set")
	}
	m, _ := portableTest(t)
	got, err := m.Prepare(context.Background(), Candidate{Path: path}, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(m.Root, got.Backend.Kind, "config.toml")
	config, err := got.Client.ConfigFile(context.Background())
	if err != nil || !strings.EqualFold(filepath.Clean(config), want) {
		t.Fatalf("config path = %q want %q: %v", config, want, err)
	}
	if _, err := got.Client.Config(context.Background()); err != nil {
		t.Fatal(err)
	}
	sources, err := got.Client.Sources(context.Background())
	if err != nil || len(sources) != 0 {
		t.Fatalf("sources = %v: %v", sources, err)
	}
	for _, part := range []string{"data", "cache"} {
		if info, err := os.Stat(filepath.Join(m.Root, got.Backend.Kind, part)); err != nil || !info.IsDir() {
			t.Fatalf("%s directory = %v: %v", part, info, err)
		}
	}
	t.Logf("%s %s: config=%s, data/cache confined to %s", got.Backend.Kind, got.Backend.Version, config, filepath.Join(m.Root, got.Backend.Kind))
}
