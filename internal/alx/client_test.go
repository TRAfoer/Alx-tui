package alx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestClientJSONCommands(t *testing.T) {
	c, logPath, _ := newFakeClient(t, "json-commands")
	ctx := context.Background()
	v, err := c.Version(ctx)
	if err != nil || v != "9.9.9" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	songs, err := c.Search(ctx, "周杰伦", "wy", 2, 10)
	if err != nil || len(songs) != 1 || songs[0].CLIID != "c1" || songs[0].Duration() != 269*time.Second {
		t.Fatalf("Search = %+v, %v", songs, err)
	}
	st, err := c.State(ctx)
	if err != nil || st.Status != "stopped" {
		t.Fatalf("State = %+v, %v", st, err)
	}
	lists, err := c.Playlists(ctx)
	if err != nil || len(lists) != 1 || lists[0].Name != "Favorites" || lists[0].Count != 2 {
		t.Fatalf("Playlists = %+v, %v", lists, err)
	}
	sources, err := c.Sources(ctx)
	if err != nil || len(sources) != 0 {
		t.Fatalf("Sources = %+v, %v", sources, err)
	}
	ly, err := c.Lyrics(ctx, "c1")
	if err != nil || ly.Lyric == "" {
		t.Fatalf("Lyrics = %+v, %v", ly, err)
	}
	assertCalls(t, logPath, [][]string{
		{"--version"},
		{"search", "周杰伦", "--source", "wy", "--page", "2", "--limit", "10", "--json", "--no-color"},
		{"state", "--json", "--no-color"},
		{"playlist", "list", "--json", "--no-color"},
		{"source", "list", "--json", "--no-color"},
		{"lyric", "c1", "--json", "--no-color"},
	})
}

func TestClientErrorJSON(t *testing.T) {
	c, _, _ := newFakeClient(t, "error-json")
	_, err := c.State(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no playback is running") {
		t.Fatalf("expected API error, got %v", err)
	}
}

func TestClientEmptyJSON(t *testing.T) {
	c, _, _ := newFakeClient(t, "empty-json")
	_, err := c.State(context.Background())
	if err == nil || !strings.Contains(err.Error(), "未返回数据") {
		t.Fatalf("expected empty-output error, got %v", err)
	}
}

func TestPlaylistAddCacheMissRetry(t *testing.T) {
	c, logPath, _ := newFakeClient(t, "cache-retry")
	song := Song{CLIID: "abc", SongID: "s1", Name: "晴天", Singer: "周杰伦", Source: "wy", Interval: "04:29"}
	if err := c.PlaylistAdd(context.Background(), "P", []Song{song}); err != nil {
		t.Fatalf("PlaylistAdd failed: %v", err)
	}
	assertCalls(t, logPath, [][]string{
		{"playlist", "add", "P", "abc", "--json", "--no-color"},
		{"search", "晴天 周杰伦", "--source", "wy", "--page", "1", "--limit", "30", "--json", "--no-color"},
		{"playlist", "add", "P", "c1", "--json", "--no-color"},
	})
}

func TestClientQuit(t *testing.T) {
	c, logPath, _ := newFakeClient(t, "quit")
	if err := c.Quit(context.Background()); err != nil {
		t.Fatalf("Quit failed: %v", err)
	}
	assertCalls(t, logPath, [][]string{{"quit", "--json", "--no-color"}})
}

func TestPlaylistImport(t *testing.T) {
	for _, tc := range []struct {
		name      string
		link      string
		localName string
		wantArgs  []string
	}{
		{"original name", "https://music.163.com/playlist?id=123", "", []string{"playlist", "import", "https://music.163.com/playlist?id=123", "--json", "--no-color"}},
		{"share text and local name", "  分享歌单：夜行 https://y.qq.com/n/ryqq/playlist/456  ", "  我的夜行  ", []string{"playlist", "import", "分享歌单：夜行 https://y.qq.com/n/ryqq/playlist/456", "--name", "我的夜行", "--json", "--no-color"}},
		{"blank optional name", "https://music.163.com/playlist?id=123", " \t ", []string{"playlist", "import", "https://music.163.com/playlist?id=123", "--json", "--no-color"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, logPath, _ := newFakeClient(t, "import-success")
			c.ConfigPath = filepath.Join(t.TempDir(), "my config.toml")
			got, err := c.PlaylistImport(context.Background(), tc.link, tc.localName)
			if err != nil {
				t.Fatal(err)
			}
			want := PlaylistImportResult{Name: "我的夜行", Count: 42, Source: "tx", Warnings: []string{"跳过 1 首不可用歌曲"}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("PlaylistImport = %+v, want %+v", got, want)
			}
			assertCalls(t, logPath, [][]string{append([]string{"--config", c.ConfigPath}, tc.wantArgs...)})
		})
	}
}

func TestPlaylistImportEmptyLink(t *testing.T) {
	c, logPath, _ := newFakeClient(t, "import-success")
	_, err := c.PlaylistImport(context.Background(), " \n\t ", "name")
	if err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("expected empty-link validation, got %v", err)
	}
	if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("empty import should not invoke alx; log stat error = %v", err)
	}
}

func TestPlaylistImportErrorJSON(t *testing.T) {
	c, logPath, _ := newFakeClient(t, "import-error")
	_, err := c.PlaylistImport(context.Background(), "https://example.com/list", "")
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || commandErr.Message != "不支持的歌单链接" {
		t.Fatalf("expected import API error, got %v", err)
	}
	if commandErr.Unwrap() == nil {
		t.Fatal("import error did not preserve nonzero process exit")
	}
	assertCalls(t, logPath, [][]string{{"playlist", "import", "https://example.com/list", "--json", "--no-color"}})
}

func TestPlaylistImportContextTimeout(t *testing.T) {
	c, _, _ := newFakeClient(t, "import-block")
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.PlaylistImport(ctx, "https://music.163.com/playlist?id=123", "")
	var commandErr *CommandError
	if !errors.As(err, &commandErr) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected import deadline CommandError, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timed out import took %v to return", elapsed)
	}
}

func TestClientVersionBrands(t *testing.T) {
	for _, tc := range []struct{ output, want string }{
		{"Lx-Cli 0.1.1-go", "0.1.1-go"},
		{"alx 0.1.0-go", "0.1.0-go"},
		{"alx 0.4.1", "0.4.1"},
		{"0.4.1", "0.4.1"},
	} {
		t.Run(tc.output, func(t *testing.T) {
			c, _, _ := newFakeClient(t, "version-output")
			c.Env = append(c.Env, "FAKE_VERSION="+tc.output)
			got, err := c.Version(context.Background())
			if err != nil || got != tc.want {
				t.Fatalf("Version = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestResolveBinaryExplicit(t *testing.T) {
	path := writeFakeExecutable(t, t.TempDir(), "external-cli")
	got, err := ResolveBinary(path)
	if err != nil || got != path {
		t.Fatalf("ResolveBinary = %q, %v; want %q", got, err, path)
	}
}

func TestResolveBinaryExplicitAndLxCliEnvironment(t *testing.T) {
	explicit := writeFakeExecutable(t, t.TempDir(), "explicit-cli")
	env := writeFakeExecutable(t, t.TempDir(), "env-cli")
	t.Setenv("LX_CLI_BIN", env)
	for _, tc := range []struct{ name, explicit, want string }{
		{"explicit before environment", explicit, explicit},
		{"LX_CLI_BIN", "", env},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveBinary(tc.explicit)
			if err != nil || got != tc.want {
				t.Fatalf("ResolveBinary = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestResolveBinaryExtensionlessExplicitPath(t *testing.T) {
	dir := t.TempDir()
	want := writeFakeExecutable(t, dir, "external-cli")
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")
	}
	got, err := ResolveBinary(filepath.Join(dir, "external-cli"))
	if err != nil || got != want {
		t.Fatalf("ResolveBinary = %q, %v; want %q", got, err, want)
	}
}

func TestResolveBinaryExplicitRelativeFile(t *testing.T) {
	cwd := t.TempDir()
	want := writeFakeExecutable(t, cwd, "chosen-cli")
	changeDirectory(t, cwd)
	t.Setenv("LX_CLI_BIN", "")
	t.Setenv("PATH", t.TempDir())
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE")
	}
	explicit := executableName("chosen-cli")
	if runtime.GOOS != "windows" {
		explicit = "./" + explicit
	}
	got, err := ResolveBinary(explicit)
	if err != nil || got != want {
		t.Fatalf("ResolveBinary = %q, %v; want %q", got, err, want)
	}
}

func siblingExecutableFixture(t *testing.T, name string) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(exe), executableName(name))
	if _, err = os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("fixture already exists: %s (%v)", path, err)
	}
	writeFakeExecutable(t, filepath.Dir(exe), name)
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func TestResolveBinaryUsesOnlyBundledLxCli(t *testing.T) {
	want := siblingExecutableFixture(t, "Lx-Cli")
	siblingExecutableFixture(t, "alx-go")
	siblingExecutableFixture(t, "alx")
	pathDir, cwd := t.TempDir(), t.TempDir()
	for _, dir := range []string{pathDir, cwd} {
		for _, name := range []string{"Lx-Cli", "alx-go", "alx"} {
			writeFakeExecutable(t, dir, name)
		}
	}
	changeDirectory(t, cwd)
	t.Setenv("PATH", pathDir)
	t.Setenv("LX_CLI_BIN", "")
	t.Setenv("ALX_BIN", filepath.Join(cwd, executableName("alx")))
	got, err := ResolveBinary("")
	if err != nil || got != want {
		t.Fatalf("default = %q, %v; want sibling %q", got, err, want)
	}
	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}
	got, err = ResolveBinary("")
	if err == nil || got != "" {
		t.Fatalf("default fell back outside bundled Lx-Cli: %q, %v", got, err)
	}
}

func TestResolveBinaryInvalidExplicitAndEnvironmentNeverFallback(t *testing.T) {
	siblingExecutableFixture(t, "Lx-Cli")
	missing := filepath.Join(t.TempDir(), executableName("missing-cli"))
	for _, viaEnv := range []bool{false, true} {
		t.Run(fmt.Sprint("environment=", viaEnv), func(t *testing.T) {
			explicit := missing
			t.Setenv("LX_CLI_BIN", "")
			if viaEnv {
				explicit = ""
				t.Setenv("LX_CLI_BIN", missing)
			}
			got, err := ResolveBinary(explicit)
			if err == nil || got != "" || !strings.Contains(err.Error(), missing) {
				t.Fatalf("invalid configured backend silently resolved: %q %v", got, err)
			}
		})
	}
}
