package alx

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt in after building Lx-Cli.exe: validates the actual backend selection and
// subprocess protocol used by the TUI without touching the user's library.
func TestLiveLxCliBackendPlayback(t *testing.T) {
	if os.Getenv("ALX_TUI_LIVE_BACKEND_TEST") != "1" {
		t.Skip("set ALX_TUI_LIVE_BACKEND_TEST=1 after building Lx-Cli")
	}
	changeDirectory(t, filepath.Join("..", ".."))
	path := os.Getenv("LX_CLI_REAL_BACKEND")
	if path == "" {
		path = filepath.Join("bin", executableName("Lx-Cli"))
	}
	path, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ALX_BIN", "ALX_CONFIG", "ALX_DATA", "ALX_CACHE"} {
		t.Setenv(key, "")
	}
	// go test runs from a temporary executable directory. Explicitly locate the
	// built CLI; sibling-only default discovery is covered by resolution tests.
	t.Setenv("LX_CLI_BIN", path)
	t.Setenv("ALX_HOME", t.TempDir())
	c, err := NewClient("", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.EqualFold(c.Bin, path) {
		t.Fatalf("TUI backend selects %s; want %s", c.Bin, path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	v, err := c.Version(ctx)
	if err != nil || !strings.HasSuffix(v, "-go") {
		t.Fatalf("wrong backend version %q: %v", v, err)
	}
	if err = c.SetConfig(ctx, "player.mpv_args", `["--ao=null"]`); err != nil {
		t.Fatal(err)
	}
	if err = c.SetConfig(ctx, "player.default_volume", "0"); err != nil {
		t.Fatal(err)
	}
	wav := make([]byte, 44+44100*2*8)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], uint32(len(wav)-8))
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 44100)
	binary.LittleEndian.PutUint32(wav[28:], 88200)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], uint32(len(wav)-44))
	wavPath := filepath.Join(t.TempDir(), "backend-check.wav")
	if err = os.WriteFile(wavPath, wav, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Quit(context.Background()) })
	if _, err = c.Play(ctx, wavPath); err != nil {
		t.Fatalf("play through TUI client: %v", err)
	}
	st, err := c.State(ctx)
	if err != nil || !st.DaemonActive || st.Status != "playing" {
		t.Fatalf("state=%#v err=%v", st, err)
	}
	if err = c.Pause(ctx); err != nil {
		t.Fatal(err)
	}
	st, err = c.State(ctx)
	if err != nil || st.Status != "paused" {
		t.Fatalf("pause state=%#v err=%v", st, err)
	}
	t.Logf("Lx-Cli backend=%s version=%s; real mpv play/pause succeeded", c.Bin, v)
}
