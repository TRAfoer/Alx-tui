package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"alxtui/internal/alx"
)

func renderView(t *testing.T, app *App) []string {
	t.Helper()
	app.Update(tea.WindowSizeMsg{Width: app.width, Height: app.height})
	view := app.View()
	if strings.ContainsRune(view, '\x00') {
		t.Fatalf("view contains a NUL byte")
	}
	lines := strings.Split(view, "\n")
	if len(lines) != app.height {
		t.Fatalf("view has %d lines, want %d", len(lines), app.height)
	}
	for i, line := range lines {
		if w := ansi.StringWidth(line); w > app.width {
			t.Fatalf("line %d width %d > terminal width %d", i, w, app.width)
		}
	}
	return lines
}

func findHit(app *App, kind HitKind) []Hit {
	var out []Hit
	for _, h := range app.hits {
		if h.Kind == kind {
			out = append(out, h)
		}
	}
	return out
}

func TestPlaybackHitsAndOverlays(t *testing.T) {
	app := testApp(120, 36, PagePlayback)
	s := sampleSong()
	app.state = alx.PlayerState{CurrentSong: &s, DaemonActive: true, Duration: 326, Position: 11, QueueLength: 2, Status: "playing", Volume: 65}
	app.queue = []alx.Song{s, sampleSong()}
	app.queueLoaded = true
	app.lyrics = alx.ParseLRC("[00:00.00]这一路上走走停停\n[00:10.00]顺着少年漂流的痕迹\n")
	app.lyricsFor = "abc"
	renderView(t, app)

	for _, kind := range []HitKind{
		HitPlaybackProgress, HitPlaybackPlayPause, HitPlaybackPrev, HitPlaybackNext,
		HitPlaybackStop, HitPlaybackVolumeMinus, HitPlaybackVolumePlus, HitNowPlayingInfo,
	} {
		if len(findHit(app, kind)) == 0 {
			t.Errorf("missing hit kind %v", kind)
		}
	}
	volume := findHit(app, HitPlaybackVolumeMinus)
	if len(volume) == 0 || volume[0].Rect.Y != 7 {
		t.Fatalf("volume hit at unexpected place: %+v", volume)
	}

	// Context menu overlay must still fit and not emit NUL bytes.
	app.openSongMenu(s, "search", 20, 9)
	app.menu.subOpen = true
	app.menu.subIdx = 0
	renderView(t, app)
	if app.menu == nil || !app.menu.subOpen {
		t.Fatal("menu did not open")
	}
}

func TestDialogMouseCancel(t *testing.T) {
	app := testApp(100, 24, PageSettings)
	app.sources = []alx.Source{{ID: "_9", Name: "全豆要[聚合音源]", Platforms: `["wy"]`, Enabled: true}}
	app.sourcesLoaded = true
	app.sourcesSelected = 0
	renderView(t, app)
	app.openRemoveSourceConfirm()
	renderView(t, app)
	if app.dlg == nil {
		t.Fatal("confirm dialog did not open")
	}
	var cancel *dialogHit
	for i := range app.dlg.hits {
		if app.dlg.hits[i].action == dialogHitCancel {
			cancel = &app.dlg.hits[i]
		}
	}
	if cancel == nil {
		t.Fatal("no cancel hit registered")
	}
	cmd := app.handleMouse(tea.MouseMsg{
		X: cancel.rect.X + 1, Y: cancel.rect.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if cmd != nil {
		t.Fatalf("unexpected command from cancel: %v", cmd)
	}
	if app.dlg != nil {
		t.Fatal("dialog did not close on cancel click")
	}
}

func TestTabClickSwitchesPage(t *testing.T) {
	app := testApp(100, 24, PagePlayback)
	renderView(t, app)
	tabs := findHit(app, HitTab)
	if len(tabs) != 4 {
		t.Fatalf("got %d tabs, want 4", len(tabs))
	}
	app.handleMouse(tea.MouseMsg{
		X: tabs[2].Rect.X + 1, Y: tabs[2].Rect.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if app.page != PagePlaylists {
		t.Fatalf("page = %v, want playlists", app.page)
	}
}

func TestPlaylistPlaySwitchesPage(t *testing.T) {
	app := testApp(120, 30, PagePlaylists)
	app.Update(playlistPlayMsg{name: "TestDesc"})
	if app.page != PagePlayback {
		t.Fatalf("page = %v, want playback", app.page)
	}
}

func TestSearchFlowWithoutBackend(t *testing.T) {
	app := testApp(120, 36, PageSearch)
	app.searchFocus = true
	app.searchInput.SetValue("周杰伦")
	app.startSearch(1)
	if !app.searchLoading || app.searchPage != 1 {
		t.Fatalf("search did not enter loading state: loading=%v page=%d", app.searchLoading, app.searchPage)
	}
	seq := app.searchSeq
	songs := []alx.Song{sampleSong()}
	app.Update(searchMsg{seq: seq, keyword: "周杰伦", source: "wy", page: 1, songs: songs})
	if app.searchLoading || len(app.searchResults) != 1 || app.searchErr != nil {
		t.Fatalf("search result not applied: loading=%v results=%d err=%v", app.searchLoading, len(app.searchResults), app.searchErr)
	}
	renderView(t, app)
	if len(findHit(app, HitSearchResult)) == 0 {
		t.Fatal("search result row has no click target")
	}
}

func TestStateMessageLoadsCurrentSong(t *testing.T) {
	app := testApp(120, 36, PagePlayback)
	app.stateFetching = true
	app.stateSeq = 3
	s := sampleSong()
	app.Update(stateMsg{seq: 3, state: alx.PlayerState{
		CurrentSong: &s, DaemonActive: true, Status: "playing", Duration: 326, Position: 12, QueueLength: 1, Volume: 70,
	}})
	if app.stateFetching {
		t.Fatal("stateFetching flag was not cleared")
	}
	if app.currentSongID() != "abc" {
		t.Fatalf("current song = %q, want abc", app.currentSongID())
	}
	if !app.pollScheduled {
		t.Fatal("poll was not scheduled")
	}
}
