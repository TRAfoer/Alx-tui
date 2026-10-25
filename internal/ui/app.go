package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"alxtui/internal/alx"
	"alxtui/internal/profile"
)

const (
	Version     = "1.5.1"
	headerRows  = 2
	footerRows  = 3
	minWidth    = 56
	minHeight   = 16
	searchLimit = 50
)

// Page identifies one of the four main views.
type Page int

const (
	PagePlayback Page = iota
	PageSearch
	PagePlaylists
	PageSettings
)

var pageTitles = []string{"播放", "搜索", "歌单", "设置"}

func (p Page) String() string {
	if int(p) >= 0 && int(p) < len(pageTitles) {
		return pageTitles[p]
	}
	return "?"
}

type toastKind int

const (
	toastInfo toastKind = iota
	toastSuccess
	toastError
)

type toastState struct {
	text  string
	kind  toastKind
	until time.Time
}

// Options configures the TUI.
type Options struct {
	CLIPath    string
	ConfigPath string
	Theme      string
}

// App is the root Bubble Tea model.
type App struct {
	client    *alx.Client
	th        Theme
	profile   *profile.Manager
	lifecycle *backendLifecycle

	// appearance / locally stored theme preferences
	baseDark       bool
	themePrefs     ThemePrefs
	themePath      string
	themeConfigErr error

	width, height int
	page          Page

	// backend metadata
	alxVersion string
	configFile string
	backendErr error

	// configuration
	cfg          alx.Config
	cfgLoaded    bool
	cfgLoading   bool
	cfgErr       error
	downloadDir  string
	quality      string
	searchSource string

	// custom sources
	sources         []alx.Source
	sourcesLoaded   bool
	sourcesLoading  bool
	sourcesErr      error
	sourcesSelected int

	// playlists
	playlists                    []alx.Playlist
	playlistsLoaded              bool
	playlistsLoading             bool
	playlistImportRefreshPending bool
	playlistsErr                 error
	playlistSelected             int
	playlistSongs                []alx.Song
	playlistName                 string
	playlistSongsLoading         bool
	playlistSongsErr             error
	playlistSongSelected         int
	playlistScroll               int
	playlistListScroll           int
	playlistFocusRight           bool
	playlistSeq                  int
	focusPlaylistName            string

	// player
	state             alx.PlayerState
	stateAt           time.Time
	stateErr          error
	stateFetching     bool
	stateSeq          int
	pollScheduled     bool
	daemonKnown       bool
	queue             []alx.Song
	queueIndex        int
	queueLoaded       bool
	queueLoading      bool
	queueSeq          int
	queueManualUntil  time.Time
	queueScroll       int
	playPending       bool
	pendingSong       alx.Song
	playSeq           int
	playlistPending   bool
	lyrics            alx.Lyrics
	lyricsSource      string
	lyricsFor         string
	lyricsLoading     bool
	lyricsErr         string
	lyricsSeq         int
	lyricsScroll      int
	lyricsManualUntil time.Time

	// search
	searchInput           textinput.Model
	searchResults         []alx.Song
	searchSubmitted       string
	searchSubmittedSource string
	searchPage            int
	searchSelected        int
	searchScroll          int
	searchLoading         bool
	searchErr             error
	searchFocus           bool
	searchSeq             int
	searchLimit           int

	// settings
	settingsScroll int

	// overlays
	menu      *menuState
	dlg       *dialogState
	dialogSeq int
	toast     toastState

	// view/layout scratch, refreshed by View
	hits              []Hit
	contentRect       Rect
	queueArea         Rect
	lyricsArea        Rect
	searchResultsArea Rect
	playlistListArea  Rect
	playlistSongsArea Rect
	settingsArea      Rect
	clock             time.Time
	spinner           int
}

// New creates the application model using the Lx-Cli backend.
func New(opts Options) (*App, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	manager, err := profile.New(executable)
	if err != nil {
		return nil, err
	}
	selected, err := manager.Selected(opts.CLIPath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	prepared, err := manager.Prepare(ctx, selected, opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	baseDark := resolveDark(opts.Theme)
	themePath, themeErr := themeConfigPath()
	var prefs ThemePrefs
	if themeErr == nil {
		var loadErr error
		prefs, loadErr = loadThemePrefs(themePath)
		if loadErr != nil {
			themeErr = loadErr
		}
	}
	th := buildTheme(baseDark, prefs)
	input := textinput.New()
	input.Prompt = "⌕ "
	input.Placeholder = ""
	input.CharLimit = 120
	input.Width = 48
	styleSearchInput(&input, th)

	now := time.Now()
	app := &App{
		client:           prepared.Client,
		profile:          manager,
		alxVersion:       prepared.Backend.Version,
		th:               th,
		baseDark:         baseDark,
		themePrefs:       prefs,
		themePath:        themePath,
		themeConfigErr:   themeErr,
		page:             PagePlayback,
		searchInput:      input,
		searchSource:     "all",
		searchLimit:      searchLimit,
		quality:          "320k",
		downloadDir:      "~/Music/rust-lx",
		stateAt:          now,
		clock:            now,
		playlistSelected: -1,
		sourcesSelected:  -1,
	}
	app.ensureBackendLifecycle()
	return app, nil
}

// Init starts background data loading and the UI clock.
func (a *App) Init() tea.Cmd {
	a.ensureBackendLifecycle()
	a.cfgLoading = true
	a.sourcesLoading = true
	a.playlistsLoading = true
	a.stateFetching = true
	a.stateSeq++
	a.queueLoading = true
	a.queueSeq++
	a.pollScheduled = false
	return a.backendCommand(tea.Batch(
		cmdInfo(a.client),
		cmdLoadConfig(a.client),
		cmdLoadSources(a.client),
		cmdLoadPlaylists(a.client),
		cmdLoadState(a.client, a.stateSeq),
		cmdLoadQueue(a.client, a.queueSeq),
		tickCmd(),
	))
}

// contentHeight returns the number of rows available to a page.
func (a *App) contentHeight() int {
	return max(0, a.height-headerRows-footerRows)
}

func (a *App) contentRectForPage() Rect {
	return Rect{X: 0, Y: headerRows, W: a.width, H: a.contentHeight()}
}

func (a *App) currentSong() *alx.Song {
	return a.state.CurrentSong
}

func (a *App) currentSongID() string {
	if a.state.CurrentSong == nil {
		return ""
	}
	return a.state.CurrentSong.CLIID
}

func (a *App) trackDuration() float64 {
	if a.state.Duration > 0 {
		return a.state.Duration
	}
	if s := a.currentSong(); s != nil {
		return s.Duration().Seconds()
	}
	return 0
}

// currentPosition interpolates between state polls for smooth progress and
// lyric highlighting.
func (a *App) currentPosition() float64 {
	pos := a.state.Position
	if a.state.Status == "playing" && !a.stateAt.IsZero() {
		pos += time.Since(a.stateAt).Seconds()
	}
	dur := a.trackDuration()
	if dur > 0 && pos > dur {
		pos = dur
	}
	if pos < 0 {
		pos = 0
	}
	return pos
}

func (a *App) progressRatio() float64 {
	dur := a.trackDuration()
	if dur <= 0 {
		return 0
	}
	r := a.currentPosition() / dur
	if r < 0 {
		return 0
	}
	if r > 1 {
		return 1
	}
	return r
}

func (a *App) selectedPlaylistName() string {
	if a.playlistSelected < 0 || a.playlistSelected >= len(a.playlists) {
		return ""
	}
	return a.playlists[a.playlistSelected].Name
}

func (a *App) toastActive() bool {
	return a.toast.text != "" && !a.toast.until.IsZero() && a.clock.Before(a.toast.until)
}

func (a *App) showToast(text string, kind toastKind) {
	if strings.TrimSpace(text) == "" {
		return
	}
	d := 5 * time.Second
	if kind == toastError {
		d = 8 * time.Second
	}
	a.toast = toastState{text: prettyText(text), kind: kind, until: time.Now().Add(d)}
}

func (a *App) toastString() string {
	if !a.toastActive() {
		return ""
	}
	switch a.toast.kind {
	case toastSuccess:
		return a.th.GoodText("✓ " + a.toast.text)
	case toastError:
		return a.th.BadText("✕ " + a.toast.text)
	default:
		return a.th.AccentText("• " + a.toast.text)
	}
}

func (a *App) playIcon() string {
	switch a.state.Status {
	case "playing":
		return "⏸"
	case "paused":
		return "▶"
	default:
		return "▶"
	}
}

func (a *App) playbackStateLabel() string {
	if a.playPending {
		return "加载中"
	}
	switch a.state.Status {
	case "playing":
		return "播放中"
	case "paused":
		return "已暂停"
	case "stopped":
		return "已停止"
	default:
		if a.state.CurrentSong == nil {
			return "未播放"
		}
		return a.state.Status
	}
}

// View renders the whole screen. It also refreshes the hit map used for mouse
// interaction, which is why it is the only place that registers clicks.
func (a *App) View() string {
	a.hits = a.hits[:0]
	a.contentRect = a.contentRectForPage()
	a.queueArea = Rect{}
	a.lyricsArea = Rect{}
	a.searchResultsArea = Rect{}
	a.playlistListArea = Rect{}
	a.playlistSongsArea = Rect{}
	a.settingsArea = Rect{}

	w, h := a.width, a.height
	if w <= 0 || h <= 0 {
		return " 正在启动 ALX TUI… "
	}
	if w < minWidth || h < minHeight {
		msg := a.th.Bold(a.th.Accent, "ALX TUI") + "\n\n" +
			a.th.Dim(fmt.Sprintf("终端窗口太小：%d×%d", w, h)) + "\n" +
			a.th.FaintText(fmt.Sprintf("请调整到至少 %d×%d", minWidth, minHeight))
		lines := make([]string, h)
		content := strings.Split(msg, "\n")
		start := max(0, (h-len(content))/2)
		for i := range lines {
			lines[i] = spaces(w)
		}
		for i, line := range content {
			if start+i < h {
				lines[start+i] = spaces(max(0, (w-displayWidth(line))/2)) + line
			}
		}
		return strings.Join(normalizeLines(lines, h, w), "\n")
	}

	header := a.renderHeader()
	content := a.renderPage(a.contentRect)
	footer := a.renderFooter()
	lines := make([]string, 0, h)
	lines = append(lines, header...)
	lines = append(lines, content...)
	lines = append(lines, footer...)
	lines = normalizeLines(lines, h, w)

	if a.menu != nil {
		lines = a.renderMenuOverlay(lines)
	}
	if a.dlg != nil {
		lines = a.renderDialogOverlay(lines)
	}
	return strings.Join(lines, "\n")
}

func (a *App) renderHeader() []string {
	w := a.width
	t := a.th
	line := spaces(w)
	var b strings.Builder
	b.WriteString("  ")
	b.WriteString(t.AccentText("♪"))
	b.WriteString(" ")
	b.WriteString(t.Brand("ALX"))
	b.WriteString(t.Dim(" TUI"))
	b.WriteString("  ")
	leftWidth := displayWidth(b.String())

	status := ""
	if a.stateErr != nil {
		status = t.BadText("○ 未连接")
	} else if a.state.DaemonActive {
		status = t.GoodText("● 在线")
	} else {
		status = t.FaintText("○ 离线")
	}
	version := ""
	if a.alxVersion != "" {
		version = t.FaintText("Lx-Cli " + a.alxVersion)
	}
	statusFull := status
	if version != "" {
		statusFull += "  " + version
	}
	statusW := displayWidth(statusFull)
	showStatus := statusW > 0 && leftWidth+40+statusW+3 <= w
	if !showStatus {
		statusFull = ""
		statusW = 0
	}
	if statusW > 0 && w > 90 && version == "" {
		// no-op, kept for clarity
	}

	x := leftWidth
	for i, title := range pageTitles {
		if i > 0 {
			b.WriteString(" ")
			x++
		}
		active := Page(i) == a.page
		chip := t.ActiveChip(title, active)
		chipW := displayWidth(chip)
		b.WriteString(chip)
		if x+chipW <= w {
			a.addHit(HitTab, Rect{X: x, Y: 0, W: chipW, H: 1}, "", i, alx.Song{})
		}
		x += chipW
	}
	row := padVisible(b.String(), w)
	if statusW > 0 && displayWidth(b.String())+2+statusW <= w {
		row = padVisible(b.String(), w-statusW) + statusFull
	}
	line = row
	return []string{line, t.Sep(w)}
}

func (a *App) renderFooter() []string {
	w := a.width
	t := a.th
	y := a.height - footerRows
	lines := make([]string, 0, footerRows)
	lines = append(lines, t.Sep(w))

	// Second row: compact now-playing bar with controls.
	innerX := 2
	innerW := max(0, w-4)
	row := spaces(w)
	if innerW > 0 {
		row = spaces(innerX) + a.renderMiniPlayer(innerX, y+1, innerW) + spaces(2)
		row = padVisible(row, w)
	}
	lines = append(lines, row)

	// Third row: contextual help and toast.
	hint := pageHelpForWidth(a.page, innerW)
	toast := a.toastString()
	helpLine := spaces(innerX)
	if innerW > 0 {
		var content string
		if toast != "" {
			// Prefer showing the toast; keep a shortened hint when possible.
			if displayWidth(toast)+displayWidth(hint)+4 <= innerW {
				content = hint + spaces(2) + toast
			} else {
				content = toast
			}
		} else {
			content = t.Dim(hint)
		}
		helpLine = spaces(innerX) + padVisible(content, innerW)
	}
	lines = append(lines, padVisible(helpLine, w))
	return lines
}

func (a *App) renderMiniPlayer(x, y, width int) string {
	t := a.th
	song := a.currentSong()
	pending := a.playPending
	if pending {
		song = &a.pendingSong
	}
	if song == nil {
		if a.playlistPending {
			msg := t.Spinner(a.spinner) + " " + t.AccentText("正在加载歌单…") + t.FaintText("  请稍候")
			return padVisible(msg, width)
		}
		msg := t.FaintText("♪ 未在播放") + t.FaintText("  去「搜索」找一首歌开始吧")
		return padVisible(msg, width)
	}

	ctrlW := 7
	timeW := 13
	hasVolume := width >= 86
	volW := 0
	if hasVolume {
		volW = 9
	}
	desiredLeft := clamp(width/3, 16, 46)
	barW := width - desiredLeft - ctrlW - volW - timeW - 5
	if barW < 8 {
		desiredLeft = max(10, width-ctrlW-volW-timeW-5-8)
		barW = width - desiredLeft - ctrlW - volW - timeW - 5
	}
	if barW < 6 {
		hasVolume = false
		volW = 0
		barW = width - desiredLeft - ctrlW - timeW - 5
	}
	barW = clamp(barW, 4, 30)
	leftW := width - ctrlW - volW - timeW - barW - 5
	if leftW < 8 {
		leftW = 8
	}

	// Left: state icon + title + artist.
	icon := "▶"
	if pending {
		icon = t.Spinner(a.spinner)
	} else {
		switch a.state.Status {
		case "playing":
			icon = "▶"
		case "paused":
			icon = "⏸"
		case "stopped":
			icon = "■"
		}
	}
	title := t.Bold(t.Text, truncate(prettyText(song.Name), max(4, leftW-8)))
	artist := t.FaintText(" · " + truncate(prettyText(song.Singer), max(0, leftW-displayWidth(title)-5)))
	left := t.AccentText(icon) + " " + title + artist
	if pending {
		left = icon + " " + t.AccentText("正在加载 ") + title
	}
	left = padVisible(left, leftW)
	a.addHit(HitMiniInfo, Rect{X: x, Y: y, W: leftW, H: 1}, "", 0, *song)

	cursor := x + leftW + 1
	ratio := a.progressRatio()
	if pending {
		ratio = 0
	}
	bar := a.renderBar(ratio, barW)
	a.addHit(HitMiniProgress, Rect{X: cursor, Y: y, W: barW, H: 1}, "", 0, *song)

	dur := a.trackDuration()
	timeText := t.FaintText(fmt.Sprintf("%s / %s", alx.FormatSeconds(a.currentPosition()), alx.FormatSeconds(dur)))
	if pending {
		timeText = t.FaintText("00:00 / --:--")
	}

	controlsX := x + width - ctrlW
	volText := ""
	if hasVolume {
		vol := a.state.Volume
		if vol <= 0 {
			vol = a.cfg.Player.DefaultVolume
		}
		volText = t.FaintText(fmt.Sprintf("VOL %d%%", vol))
	}
	playLabel := a.playIcon()
	if pending {
		playLabel = "·"
	}
	controls := t.FaintText("⏮") + "  " + t.AccentText(playLabel) + "  " + t.FaintText("⏭")
	a.addHit(HitMiniPrev, Rect{X: controlsX, Y: y, W: 2, H: 1}, "", 0, *song)
	a.addHit(HitMiniPlayPause, Rect{X: controlsX + 3, Y: y, W: 2, H: 1}, "", 0, *song)
	a.addHit(HitMiniNext, Rect{X: controlsX + 6, Y: y, W: 1, H: 1}, "", 0, *song)

	// Assemble: left | bar | time | volume | controls
	var sb strings.Builder
	sb.WriteString(left)
	sb.WriteString(" ")
	sb.WriteString(bar)
	sb.WriteString(" ")
	sb.WriteString(padVisible(timeText, timeW))
	if hasVolume {
		volX := controlsX - 2 - volW
		cur := x + leftW + 1 + barW + 1 + timeW
		if volX > cur {
			sb.WriteString(spaces(volX - cur))
			sb.WriteString(padVisible(volText, volW))
			sb.WriteString(spaces(2))
		} else {
			sb.WriteString(spaces(2))
		}
	} else {
		cur := x + leftW + 1 + barW + 1 + timeW
		if controlsX > cur {
			sb.WriteString(spaces(controlsX - cur))
		} else {
			sb.WriteString(spaces(2))
		}
	}
	sb.WriteString(controls)
	out := sb.String()
	return padVisible(out, width)
}

func (a *App) renderBar(ratio float64, width int) string {
	if width <= 0 {
		return ""
	}
	ratio = clampFloat(ratio, 0, 1)
	filled := int(ratio*float64(width) + 0.5)
	filled = clamp(filled, 0, width)
	if filled == 0 {
		return a.th.FaintText(strings.Repeat("─", width))
	}
	return a.th.AccentText(strings.Repeat("━", filled)) + a.th.FaintText(strings.Repeat("─", width-filled))
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (a *App) renderPage(r Rect) []string {
	switch a.page {
	case PagePlayback:
		return a.renderPlaybackPage(r)
	case PageSearch:
		return a.renderSearchPage(r)
	case PagePlaylists:
		return a.renderPlaylistPage(r)
	case PageSettings:
		return a.renderSettingsPage(r)
	default:
		return nil
	}
}

func pageHelpForWidth(p Page, width int) string {
	if width < 88 {
		switch p {
		case PagePlayback:
			return "Space 播放  ←→ 快进  ↑↓ 音量  右键 菜单"
		case PageSearch:
			return "Enter 搜索/播放  / 输入  ↑↓ 选择  右键 菜单"
		case PagePlaylists:
			return "↑↓ 选择  Enter 播放  n 新建  i 导入"
		case PageSettings:
			return "↑↓ 滚动  a 添加音源  r 刷新  q 退出"
		}
	}
	return pageHelp(p)
}

func pageHelp(p Page) string {
	switch p {
	case PagePlayback:
		return "Space 播放/暂停   ←/→ 快退/快进   ↑/↓ 音量   右键正在播放 更多   Tab 切页"
	case PageSearch:
		return "Enter 搜索/播放   / 输入   ↑/↓ 选择   PgUp/PgDn 翻页   右键 更多   Tab 切页"
	case PagePlaylists:
		return "↑/↓ 选择   Enter 打开/播放   n 新建   i 导入链接   d 删除   右键歌曲 更多   Tab 切页"
	case PageSettings:
		return "鼠标点击设置项   ↑/↓ 滚动   1-4 切页   q 退出"
	default:
		return "Tab 切页   q 退出"
	}
}

// selectedSourceIndex returns the selected custom source, or nil.
func (a *App) selectedSource() *alx.Source {
	if a.sourcesSelected < 0 || a.sourcesSelected >= len(a.sources) {
		return nil
	}
	return &a.sources[a.sourcesSelected]
}

// playlistSongContext returns the playlist name encoded in a hit value.
func playlistSongContext(value string) string {
	if strings.HasPrefix(value, "playlist:") {
		return strings.TrimPrefix(value, "playlist:")
	}
	return ""
}

// Shutdown stops the alx daemon and any mpv process it owns. It is called
// after the Bubble Tea program returns, including normal q/Ctrl+C exits and
// handled termination signals.
func (a *App) Shutdown() error {
	if a == nil || a.client == nil {
		return nil
	}
	a.ensureBackendLifecycle()
	life := a.lifecycle
	life.mu.Lock()
	life.closed = true
	life.epoch.Add(1)
	if life.sessionCancel != nil {
		life.sessionCancel()
	}
	oldClient := life.client
	client := *oldClient
	client.SessionContext = nil
	life.mu.Unlock()
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	waitErr := oldClient.WaitIdle(waitCtx)
	waitCancel()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return errors.Join(waitErr, client.Quit(ctx))
}
