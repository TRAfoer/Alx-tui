package alx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Command timeouts. Search and source operations talk to remote music APIs,
// so they are allowed to take considerably longer than local database calls.
const (
	defaultTimeout  = 25 * time.Second
	searchTimeout   = 75 * time.Second
	lyricTimeout    = 45 * time.Second
	sourceTimeout   = 120 * time.Second
	downloadTimeout = 45 * time.Second
)

// CommandError is returned for failed, timed out or unparsable alx commands.
type CommandError struct {
	Op      string
	Message string
	Stderr  string
	Err     error
}

func (e *CommandError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "alx command failed"
}

func (e *CommandError) Unwrap() error { return e.Err }

// Client is a thin, JSON-oriented wrapper around the alx executable.
type Client struct {
	Bin        string
	ConfigPath string
	Env        []string
	// SessionContext ends all commands belonging to a backend selected by the
	// TUI. Nil preserves independent CLI callers and cleanup operations.
	SessionContext context.Context
	activity       *clientActivity
}

type clientActivity struct {
	mu    sync.Mutex
	count int
	idle  chan struct{}
}

// WithSession binds a copied client to a cancellable TUI session. It also
// enables command joining for clients constructed as struct literals.
func (c *Client) WithSession(ctx context.Context) *Client {
	if c == nil {
		return nil
	}
	next := *c
	next.SessionContext = ctx
	if next.activity == nil {
		next.activity = &clientActivity{}
	}
	return &next
}

func (a *clientActivity) begin(session context.Context) (func(), error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := session.Err(); err != nil {
		return nil, err
	}
	if a.count == 0 {
		a.idle = make(chan struct{})
	}
	a.count++
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.count--
		if a.count == 0 {
			close(a.idle)
		}
	}, nil
}

// WaitIdle joins commands after the caller cancels SessionContext, so detached
// backend startup cannot race a following Quit. Copies share this activity.
func (c *Client) WaitIdle(ctx context.Context) error {
	if c == nil || c.activity == nil {
		return nil
	}
	c.activity.mu.Lock()
	if c.activity.count == 0 {
		c.activity.mu.Unlock()
		return nil
	}
	idle := c.activity.idle
	c.activity.mu.Unlock()
	select {
	case <-idle:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ResolveBinary uses the Lx-Cli executable beside the TUI, unless the caller
// explicitly chooses an external CLI with --cli or LX_CLI_BIN.
func ResolveBinary(explicit string) (string, error) {
	resolve := func(cand string) (string, error) {
		p, err := exec.LookPath(cand)
		// A configured relative executable is deliberate, so ErrDot is allowed.
		if err != nil && !errors.Is(err, exec.ErrDot) {
			return "", err
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	if explicit != "" {
		p, err := resolve(explicit)
		if err != nil {
			return "", fmt.Errorf("--cli 指定的 Lx-Cli 不可用 %s: %w", explicit, err)
		}
		return p, nil
	}
	if env := os.Getenv("LX_CLI_BIN"); env != "" {
		p, err := resolve(env)
		if err != nil {
			return "", fmt.Errorf("LX_CLI_BIN 指定的 Lx-Cli 不可用 %s: %w", env, err)
		}
		return p, nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("无法定位 TUI 程序: %w", err)
	}
	name := "Lx-Cli"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(filepath.Dir(exe), name)
	if p, err := resolve(path); err == nil {
		return p, nil
	}
	return "", fmt.Errorf("找不到同目录 Lx-Cli 可执行文件 %s（或使用 --cli / LX_CLI_BIN 指定路径）", path)
}

// NewClient creates a client for the given binary. bin may be empty.
func NewClient(bin, configPath string) (*Client, error) {
	resolved, err := ResolveBinary(bin)
	if err != nil {
		return nil, err
	}
	return &Client{Bin: resolved, ConfigPath: configPath, Env: os.Environ(), activity: &clientActivity{}}, nil
}

func (c *Client) baseArgs(args ...string) []string {
	var out []string
	if c.ConfigPath != "" {
		out = append(out, "--config", c.ConfigPath)
	}
	out = append(out, args...)
	return out
}

func (c *Client) environment() []string {
	if c.Env != nil {
		return c.Env
	}
	return os.Environ()
}

func (c *Client) run(ctx context.Context, timeout time.Duration, args ...string) (stdout, stderr []byte, err error) {
	if c == nil {
		return nil, nil, errors.New("alx client is nil")
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	if c.SessionContext != nil {
		if c.activity != nil {
			finish, err := c.activity.begin(c.SessionContext)
			if err != nil {
				return nil, nil, &CommandError{Op: strings.Join(args, " "), Message: "后端会话已结束", Err: err}
			}
			defer finish()
		}
		if sessionErr := c.SessionContext.Err(); sessionErr != nil {
			return nil, nil, &CommandError{Op: strings.Join(args, " "), Message: "后端会话已结束", Err: sessionErr}
		}
		stop := context.AfterFunc(c.SessionContext, cancel)
		defer stop()
	}

	cmd := exec.CommandContext(ctx, c.Bin, c.baseArgs(args...)...)
	cmd.Env = c.environment()
	cmd.Stdin = nil
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	// A daemonized child may inherit our stdout/stderr pipes. Without a
	// WaitDelay, Cmd.Wait can block forever waiting for those pipes to close
	// even after the alx process itself has exited.
	cmd.WaitDelay = 2 * time.Second
	err = cmd.Run()
	if errors.Is(err, exec.ErrWaitDelay) {
		if cmd.ProcessState != nil && cmd.ProcessState.ExitCode() == 0 {
			err = nil
		}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return outBuf.Bytes(), errBuf.Bytes(), &CommandError{Op: strings.Join(args, " "), Message: "alx 命令超时（" + timeout.String() + "）", Err: ctx.Err()}
	}
	if ctx.Err() == context.Canceled {
		return outBuf.Bytes(), errBuf.Bytes(), &CommandError{Op: strings.Join(args, " "), Message: "操作已取消", Err: ctx.Err()}
	}
	return outBuf.Bytes(), errBuf.Bytes(), err
}

func (c *Client) commandError(args []string, runErr error, stdout, stderr []byte) error {
	msg := strings.TrimSpace(string(stderr))
	if msg == "" {
		msg = strings.TrimSpace(string(stdout))
	}
	msg = cleanCommandMessage(msg)
	if msg == "" && runErr != nil {
		msg = runErr.Error()
	}
	return &CommandError{
		Op:      strings.Join(args, " "),
		Message: msg,
		Stderr:  strings.TrimSpace(string(stderr)),
		Err:     runErr,
	}
}

func cleanCommandMessage(s string) string {
	s = strings.TrimSpace(s)
	for _, prefix := range []string{"error:", "Error:", "ERROR:"} {
		if strings.HasPrefix(s, prefix) {
			s = strings.TrimSpace(strings.TrimPrefix(s, prefix))
		}
	}
	// JSON error bodies can arrive without --json being honored.
	if strings.HasPrefix(s, "{") {
		var env struct {
			Error string `json:"error"`
		}
		if json.Unmarshal([]byte(s), &env) == nil && strings.TrimSpace(env.Error) != "" {
			s = strings.TrimSpace(env.Error)
		}
	}
	return s
}

// extractJSON finds the first JSON value in output. Some alx commands print a
// human-readable preamble even when --json is requested.
func extractJSON(b []byte) []byte {
	t := bytes.TrimSpace(b)
	if len(t) == 0 {
		return nil
	}
	if t[0] == '{' || t[0] == '[' {
		return t
	}
	for i, ch := range t {
		if ch != '{' && ch != '[' {
			continue
		}
		var raw json.RawMessage
		dec := json.NewDecoder(bytes.NewReader(t[i:]))
		if err := dec.Decode(&raw); err == nil {
			return raw
		}
	}
	return t
}

func apiError(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var env struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(data, &env) == nil && strings.TrimSpace(env.Error) != "" {
		return strings.TrimSpace(env.Error)
	}
	return ""
}

func (c *Client) callJSON(ctx context.Context, timeout time.Duration, out any, args ...string) error {
	stdout, stderr, runErr := c.run(ctx, timeout, args...)
	if len(bytes.TrimSpace(stdout)) == 0 {
		if runErr != nil {
			return c.commandError(args, runErr, stdout, stderr)
		}
		return &CommandError{Op: strings.Join(args, " "), Message: "alx 未返回数据"}
	}
	data := extractJSON(stdout)
	if msg := apiError(data); msg != "" {
		return &CommandError{Op: strings.Join(args, " "), Message: msg, Stderr: strings.TrimSpace(string(stderr)), Err: runErr}
	}
	if runErr != nil {
		return c.commandError(args, runErr, stdout, stderr)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return &CommandError{Op: strings.Join(args, " "), Message: fmt.Sprintf("无法解析 alx 输出：%v", err), Stderr: strings.TrimSpace(string(stdout)), Err: err}
	}
	return nil
}

func (c *Client) callAction(ctx context.Context, timeout time.Duration, args ...string) error {
	stdout, stderr, runErr := c.run(ctx, timeout, args...)
	data := extractJSON(stdout)
	if msg := apiError(data); msg != "" {
		return &CommandError{Op: strings.Join(args, " "), Message: msg, Stderr: strings.TrimSpace(string(stderr)), Err: runErr}
	}
	if runErr != nil {
		return c.commandError(args, runErr, stdout, stderr)
	}
	trimmed := strings.TrimSpace(string(stdout))
	if strings.HasPrefix(trimmed, "error:") || strings.HasPrefix(trimmed, "Error:") {
		return &CommandError{Op: strings.Join(args, " "), Message: cleanCommandMessage(trimmed), Stderr: strings.TrimSpace(string(stderr))}
	}
	return nil
}

// Version removes the Lx-Cli or legacy alx brand from the version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	out, stderr, err := c.run(ctx, 10*time.Second, "--version")
	if err != nil {
		return "", c.commandError([]string{"--version"}, err, out, stderr)
	}
	v := strings.TrimSpace(string(out))
	v = strings.TrimPrefix(v, "Lx-Cli ")
	v = strings.TrimPrefix(v, "alx ")
	return v, nil
}

// ConfigFile returns the resolved alx config file path.
func (c *Client) ConfigFile(ctx context.Context) (string, error) {
	out, stderr, err := c.run(ctx, 10*time.Second, "config", "path")
	if err != nil {
		return "", c.commandError([]string{"config", "path"}, err, out, stderr)
	}
	return strings.TrimSpace(string(out)), nil
}

// Config reads the current alx configuration as JSON.
func (c *Client) Config(ctx context.Context) (Config, error) {
	var cfg Config
	err := c.callJSON(ctx, defaultTimeout, &cfg, "config", "show", "--json", "--no-color")
	return cfg, err
}

// SetConfig updates one alx configuration key.
func (c *Client) SetConfig(ctx context.Context, key, value string) error {
	return c.callAction(ctx, defaultTimeout, "config", "set", key, value, "--json", "--no-color")
}

// Search searches all or one platform. source is "all" or wy/kw/tx/mg/kg.
func (c *Client) Search(ctx context.Context, keyword, source string, page, limit int) ([]Song, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, errors.New("搜索关键词不能为空")
	}
	if source == "" {
		source = "all"
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 50
	}
	args := []string{"search", keyword,
		"--source", source,
		"--page", strconv.Itoa(page),
		"--limit", strconv.Itoa(limit),
		"--json", "--no-color"}
	var songs []Song
	if err := c.callJSON(ctx, searchTimeout, &songs, args...); err != nil {
		return nil, err
	}
	return songs, nil
}

// State reads the current daemon/player state.
func (c *Client) State(ctx context.Context) (PlayerState, error) {
	var st PlayerState
	err := c.callJSON(ctx, 8*time.Second, &st, "state", "--json", "--no-color")
	return st, err
}

// Queue lists the active play queue.
func (c *Client) Queue(ctx context.Context) (Queue, error) {
	var q Queue
	err := c.callJSON(ctx, 10*time.Second, &q, "queue", "show", "--json", "--no-color")
	return q, err
}

// Play starts a song by CLI ID (or URL / local path).
func (c *Client) Play(ctx context.Context, id string) (PlayResult, error) {
	var res PlayResult
	id = strings.TrimSpace(id)
	if id == "" {
		return res, errors.New("歌曲 ID 为空")
	}
	err := c.callJSON(ctx, searchTimeout, &res, "play", id, "--json", "--no-color")
	return res, err
}

// PlayPlaylist replaces the queue with a playlist and starts playing it.
func (c *Client) PlayPlaylist(ctx context.Context, name string, shuffle bool) error {
	args := []string{"playlist", "play", name, "--json", "--no-color"}
	if shuffle {
		args = append(args, "--shuffle")
	}
	return c.callAction(ctx, searchTimeout, args...)
}

// Pause pauses playback.
func (c *Client) Pause(ctx context.Context) error {
	return c.callAction(ctx, defaultTimeout, "pause", "--json", "--no-color")
}

// Resume resumes playback.
func (c *Client) Resume(ctx context.Context) error {
	return c.callAction(ctx, defaultTimeout, "resume", "--json", "--no-color")
}

// Stop stops playback without exiting the daemon.
func (c *Client) Stop(ctx context.Context) error {
	return c.callAction(ctx, defaultTimeout, "stop", "--json", "--no-color")
}

// Quit stops the alx-managed mpv process and exits the background daemon.
func (c *Client) Quit(ctx context.Context) error {
	return c.callAction(ctx, 20*time.Second, "quit", "--json", "--no-color")
}

// Next skips to the next song.
func (c *Client) Next(ctx context.Context) error {
	return c.callAction(ctx, defaultTimeout, "next", "--json", "--no-color")
}

// Previous skips to the previous song.
func (c *Client) Previous(ctx context.Context) error {
	return c.callAction(ctx, defaultTimeout, "prev", "--json", "--no-color")
}

// Seek jumps to an absolute position.
func (c *Client) Seek(ctx context.Context, seconds int) error {
	if seconds < 0 {
		seconds = 0
	}
	return c.callAction(ctx, defaultTimeout, "seek", strconv.Itoa(seconds), "--json", "--no-color")
}

// SetVolume sets the player volume (0-100).
func (c *Client) SetVolume(ctx context.Context, volume int) error {
	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}
	return c.callAction(ctx, defaultTimeout, "volume", strconv.Itoa(volume), "--json", "--no-color")
}

// SetRepeat sets repeat mode: off, one or all.
func (c *Client) SetRepeat(ctx context.Context, mode string) error {
	if mode != "off" && mode != "one" && mode != "all" {
		return fmt.Errorf("无效的循环模式：%s", mode)
	}
	return c.callAction(ctx, defaultTimeout, "repeat", mode, "--json", "--no-color")
}

// SetShuffle toggles shuffle playback.
func (c *Client) SetShuffle(ctx context.Context, enabled bool) error {
	v := "off"
	if enabled {
		v = "on"
	}
	return c.callAction(ctx, defaultTimeout, "shuffle", v, "--json", "--no-color")
}

// Lyrics fetches the LRC document for a song.
func (c *Client) Lyrics(ctx context.Context, id string) (LyricResponse, error) {
	var res LyricResponse
	id = strings.TrimSpace(id)
	if id == "" {
		return res, errors.New("歌曲 ID 为空")
	}
	err := c.callJSON(ctx, lyricTimeout, &res, "lyric", id, "--json", "--no-color")
	return res, err
}

// Playlists lists all playlists.
func (c *Client) Playlists(ctx context.Context) ([]Playlist, error) {
	var out []Playlist
	err := c.callJSON(ctx, defaultTimeout, &out, "playlist", "list", "--json", "--no-color")
	return out, err
}

// PlaylistSongs lists the songs in one playlist.
func (c *Client) PlaylistSongs(ctx context.Context, name string) ([]Song, error) {
	var out []Song
	err := c.callJSON(ctx, defaultTimeout, &out, "playlist", "show", name, "--json", "--no-color")
	return out, err
}

// PlaylistImport imports a remote playlist link or share text. An empty name
// keeps the remote playlist's original name.
func (c *Client) PlaylistImport(ctx context.Context, link, name string) (PlaylistImportResult, error) {
	var result PlaylistImportResult
	link = strings.TrimSpace(link)
	if link == "" {
		return result, errors.New("歌单链接不能为空")
	}
	args := []string{"playlist", "import", link}
	if name = strings.TrimSpace(name); name != "" {
		args = append(args, "--name", name)
	}
	args = append(args, "--json", "--no-color")
	err := c.callJSON(ctx, sourceTimeout, &result, args...)
	return result, err
}

// PlaylistCreate creates a playlist. description may be empty.
func (c *Client) PlaylistCreate(ctx context.Context, name, description string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("歌单名称不能为空")
	}
	args := []string{"playlist", "create", name, "--json", "--no-color"}
	if strings.TrimSpace(description) != "" {
		args = append(args, description)
	}
	return c.callAction(ctx, defaultTimeout, args...)
}

// PlaylistDelete deletes a playlist.
func (c *Client) PlaylistDelete(ctx context.Context, name string) error {
	return c.callAction(ctx, defaultTimeout, "playlist", "delete", name, "--json", "--no-color")
}

// PlaylistRename renames a playlist.
func (c *Client) PlaylistRename(ctx context.Context, oldName, newName string) error {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("歌单名称不能为空")
	}
	return c.callAction(ctx, defaultTimeout, "playlist", "rename", oldName, newName, "--json", "--no-color")
}

// PlaylistAdd adds songs to a playlist. If the CLI complains that a song is
// missing from its search cache (which happens for songs opened from an old
// playlist), it searches for the song to repopulate the cache and retries.
func (c *Client) PlaylistAdd(ctx context.Context, playlist string, songs []Song) error {
	if len(songs) == 0 {
		return errors.New("没有要添加的歌曲")
	}
	ids := make([]string, 0, len(songs))
	for _, s := range songs {
		if s.CLIID == "" {
			return fmt.Errorf("歌曲 %q 缺少 CLI ID", s.Name)
		}
		ids = append(ids, s.CLIID)
	}
	err := c.playlistAddRaw(ctx, playlist, ids)
	if err == nil || !isCacheMiss(err) {
		return err
	}
	resolved, rerr := c.resolveCached(ctx, songs)
	if rerr != nil {
		return err
	}
	return c.playlistAddRaw(ctx, playlist, resolved)
}

func (c *Client) playlistAddRaw(ctx context.Context, playlist string, ids []string) error {
	args := []string{"playlist", "add", playlist}
	args = append(args, ids...)
	args = append(args, "--json", "--no-color")
	return c.callAction(ctx, defaultTimeout, args...)
}

// PlaylistRemove removes a song from a playlist by CLI ID.
func (c *Client) PlaylistRemove(ctx context.Context, playlist, cliID string) error {
	return c.callAction(ctx, defaultTimeout, "playlist", "remove", playlist, cliID, "--json", "--no-color")
}

// QueueAdd appends songs to the play queue.
func (c *Client) QueueAdd(ctx context.Context, songs []Song) (QueueAddResult, error) {
	var res QueueAddResult
	if len(songs) == 0 {
		return res, errors.New("没有要加入队列的歌曲")
	}
	ids := make([]string, 0, len(songs))
	for _, s := range songs {
		if s.CLIID == "" {
			return res, fmt.Errorf("歌曲 %q 缺少 CLI ID", s.Name)
		}
		ids = append(ids, s.CLIID)
	}
	args := append([]string{"queue", "add"}, ids...)
	args = append(args, "--json", "--no-color")
	err := c.callJSON(ctx, defaultTimeout, &res, args...)
	if err == nil || !isCacheMiss(err) {
		return res, err
	}
	resolved, rerr := c.resolveCached(ctx, songs)
	if rerr != nil {
		return res, err
	}
	args = append([]string{"queue", "add"}, resolved...)
	args = append(args, "--json", "--no-color")
	err = c.callJSON(ctx, defaultTimeout, &res, args...)
	return res, err
}

// DownloadAdd queues songs for background download.
func (c *Client) DownloadAdd(ctx context.Context, songs []Song) (DownloadAddResult, error) {
	var res DownloadAddResult
	if len(songs) == 0 {
		return res, errors.New("没有要下载的歌曲")
	}
	ids := make([]string, 0, len(songs))
	for _, s := range songs {
		if s.CLIID == "" {
			return res, fmt.Errorf("歌曲 %q 缺少 CLI ID", s.Name)
		}
		ids = append(ids, s.CLIID)
	}
	args := append([]string{"download", "add"}, ids...)
	args = append(args, "--json", "--no-color")
	err := c.callJSON(ctx, downloadTimeout, &res, args...)
	if err == nil || !isCacheMiss(err) {
		return res, err
	}
	resolved, rerr := c.resolveCached(ctx, songs)
	if rerr != nil {
		return res, err
	}
	args = append([]string{"download", "add"}, resolved...)
	args = append(args, "--json", "--no-color")
	err = c.callJSON(ctx, downloadTimeout, &res, args...)
	return res, err
}

// Downloads returns the download history.
func (c *Client) Downloads(ctx context.Context) ([]DownloadTask, error) {
	var out []DownloadTask
	err := c.callJSON(ctx, defaultTimeout, &out, "download", "list", "--json", "--no-color")
	return out, err
}

// Sources lists installed custom JavaScript sources. The CLI prints a plain
// sentence instead of JSON when the list is empty.
func (c *Client) Sources(ctx context.Context) ([]Source, error) {
	stdout, stderr, runErr := c.run(ctx, defaultTimeout, "source", "list", "--json", "--no-color")
	trimmed := bytes.TrimSpace(stdout)
	if runErr != nil {
		return nil, c.commandError([]string{"source", "list"}, runErr, stdout, stderr)
	}
	if len(trimmed) == 0 {
		return []Source{}, nil
	}
	if !bytes.HasPrefix(trimmed, []byte("[")) && !bytes.HasPrefix(trimmed, []byte("{")) {
		if strings.Contains(string(trimmed), "No custom music sources") {
			return []Source{}, nil
		}
		return nil, &CommandError{Op: "source list", Message: fmt.Sprintf("无法解析音源列表：%s", strings.TrimSpace(string(trimmed)))}
	}
	data := extractJSON(trimmed)
	if msg := apiError(data); msg != "" {
		return nil, &CommandError{Op: "source list", Message: msg}
	}
	var out []Source
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, &CommandError{Op: "source list", Message: fmt.Sprintf("无法解析音源列表：%v", err)}
	}
	return out, nil
}

// SourceAdd installs a source from a local path or URL.
func (c *Client) SourceAdd(ctx context.Context, pathOrURL string) error {
	pathOrURL = strings.TrimSpace(pathOrURL)
	if pathOrURL == "" {
		return errors.New("音源路径或 URL 不能为空")
	}
	return c.callAction(ctx, sourceTimeout, "source", "add", pathOrURL, "--json", "--no-color")
}

// SourceRemove removes an installed source by ID.
func (c *Client) SourceRemove(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("音源 ID 不能为空")
	}
	return c.callAction(ctx, sourceTimeout, "source", "remove", id, "--json", "--no-color")
}

func isCacheMiss(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "not found in cache") || strings.Contains(msg, "缓存")
}

// resolveCached searches for the songs so the CLI's in-database result cache is
// populated, then returns the CLI IDs of the best matches.
func (c *Client) resolveCached(ctx context.Context, songs []Song) ([]string, error) {
	ids := make([]string, 0, len(songs))
	for _, song := range songs {
		query := strings.TrimSpace(song.Name + " " + song.Singer)
		if query == "" {
			return nil, fmt.Errorf("无法重新查找歌曲：缺少名称")
		}
		source := song.Source
		if source == "" {
			source = "all"
		}
		results, err := c.Search(ctx, query, source, 1, 30)
		if err != nil {
			return nil, err
		}
		var match *Song
		for i := range results {
			r := &results[i]
			if song.SongID != "" && r.SongID == song.SongID && (song.Source == "" || r.Source == song.Source) {
				match = r
				break
			}
			if song.CLIID != "" && r.CLIID == song.CLIID {
				match = r
				break
			}
		}
		if match == nil {
			for i := range results {
				r := &results[i]
				if strings.EqualFold(strings.TrimSpace(r.Name), strings.TrimSpace(song.Name)) &&
					strings.EqualFold(strings.TrimSpace(r.Singer), strings.TrimSpace(song.Singer)) {
					match = r
					break
				}
			}
		}
		if match == nil || match.CLIID == "" {
			return nil, fmt.Errorf("在缓存中找不到歌曲 %q", song.Label())
		}
		ids = append(ids, match.CLIID)
	}
	return ids, nil
}
