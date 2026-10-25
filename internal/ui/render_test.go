package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"alxtui/internal/alx"
)

func testApp(width, height int, page Page) *App {
	input := textinput.New()
	input.Prompt = "⌕ "
	input.Width = 40
	app := &App{
		client:           &alx.Client{Bin: "/tmp/alx"},
		th:               NewTheme(true),
		baseDark:         true,
		width:            width,
		height:           height,
		page:             page,
		searchInput:      input,
		searchSource:     "wy",
		quality:          "flac",
		downloadDir:      "~/Music/rust-lx",
		searchLimit:      50,
		clock:            time.Now(),
		stateAt:          time.Now().Add(-11 * time.Second),
		daemonKnown:      true,
		playlistSelected: 1,
		sourcesSelected:  0,
	}
	return app
}

func sampleSong() alx.Song {
	return alx.Song{CLIID: "abc", SongID: "1", Name: "起风了（毕业季特别版）", Singer: "星野念_Hoshi", Source: "wy", Interval: "05:26", AlbumName: "起风了（毕业季特别版）"}
}

func TestViewsFitTerminal(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
		page   Page
		setup  func(*App)
	}{
		{"playback", 120, 36, PagePlayback, func(a *App) {
			s := sampleSong()
			a.state = alx.PlayerState{CurrentSong: &s, DaemonActive: true, Duration: 326, Position: 11, QueueLength: 3, Status: "playing", Volume: 65, Repeat: "off"}
			a.queue = []alx.Song{s, sampleSong(), sampleSong()}
			a.queueLoaded = true
			a.queueIndex = 0
			a.lyrics = alx.ParseLRC("[00:00.00]这一路上走走停停\n[00:10.00]顺着少年漂流的痕迹\n[00:20.00]迈出车站的前一刻\n[00:24.00]竟有些犹豫\n")
			a.lyricsFor = "abc"
		}},
		{"playback-small", 70, 24, PagePlayback, func(a *App) {
			s := sampleSong()
			a.state = alx.PlayerState{CurrentSong: &s, DaemonActive: true, Duration: 326, Position: 11, QueueLength: 1, Status: "playing", Volume: 65}
			a.lyrics = alx.ParseLRC("[00:00.00]这一路上走走停停\n[00:10.00]顺着少年漂流的痕迹\n")
			a.lyricsFor = "abc"
		}},
		{"playback-min", 56, 16, PagePlayback, func(a *App) {
			s := sampleSong()
			a.state = alx.PlayerState{CurrentSong: &s, DaemonActive: true, Duration: 326, Position: 11, QueueLength: 1, Status: "playing", Volume: 65}
			a.lyrics = alx.ParseLRC("[00:00.00]这一路上走走停停\n[00:10.00]顺着少年漂流的痕迹\n")
			a.lyricsFor = "abc"
		}},
		{"search-min", 56, 16, PageSearch, func(a *App) {
			a.searchSubmitted = "周杰伦"
			a.searchSubmittedSource = "wy"
			a.searchPage = 1
			for i := 0; i < 5; i++ {
				s := sampleSong()
				s.CLIID = string(rune('a' + i))
				a.searchResults = append(a.searchResults, s)
			}
		}},
		{"playlists-min", 56, 16, PagePlaylists, func(a *App) {
			a.playlists = []alx.Playlist{{Name: "Favorites", Count: 0}, {Name: "测试歌单", Count: 1}}
			a.playlistName = "测试歌单"
			a.playlistSongs = []alx.Song{sampleSong()}
		}},
		{"settings-min", 56, 16, PageSettings, func(a *App) {
			a.sources = []alx.Source{{ID: "_9", Name: "全豆要[聚合音源]", Version: "9.3", Platforms: `["wy"]`, Enabled: true}}
			a.sourcesLoaded = true
			a.alxVersion = "0.4.1"
		}},
		{"search", 120, 36, PageSearch, func(a *App) {
			a.searchSubmitted = "周杰伦"
			a.searchSubmittedSource = "wy"
			a.searchPage = 1
			for i := 0; i < 20; i++ {
				s := sampleSong()
				s.CLIID = string(rune('a' + i))
				a.searchResults = append(a.searchResults, s)
			}
		}},
		{"playlists", 120, 30, PagePlaylists, func(a *App) {
			a.playlists = []alx.Playlist{{Name: "Favorites", Count: 0}, {Name: "测试歌单名称非常长非常长非常长", Count: 12}}
			a.playlistName = "测试歌单名称非常长非常长非常长"
			a.playlistSongs = []alx.Song{sampleSong(), sampleSong()}
		}},
		{"playlists-narrow", 70, 24, PagePlaylists, func(a *App) {
			a.playlists = []alx.Playlist{{Name: "Favorites", Count: 0}, {Name: "测试歌单", Count: 12}}
			a.playlistName = "测试歌单"
			a.playlistSongs = []alx.Song{sampleSong()}
		}},
		{"settings", 100, 24, PageSettings, func(a *App) {
			a.sources = []alx.Source{{ID: "_9", Name: "全豆要[聚合音源]", Version: "9.3 93特供版 DeepSeek优化", Platforms: `["kg","kw","mg","tx","wy"]`, Enabled: true}}
			a.sourcesLoaded = true
			a.sourcesSelected = 0
			a.alxVersion = "0.4.1"
			a.configFile = "/home/user/.config/agent-lx-music/config.toml"
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := testApp(tt.width, tt.height, tt.page)
			tt.setup(app)
			app.Update(tea.WindowSizeMsg{Width: tt.width, Height: tt.height})
			view := app.View()
			lines := strings.Split(view, "\n")
			if len(lines) != tt.height {
				t.Fatalf("got %d lines, want %d", len(lines), tt.height)
			}
			for i, line := range lines {
				if w := ansi.StringWidth(line); w > tt.width {
					t.Fatalf("line %d width %d > terminal width %d: %q", i, w, tt.width, ansi.Strip(line))
				}
			}
		})
	}
}
