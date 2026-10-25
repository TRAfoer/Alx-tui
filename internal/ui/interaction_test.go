package ui

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

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

func TestPlaybackQueueClickPlaysVisibleSong(t *testing.T) {
	for _, tc := range []struct {
		name     string
		scroll   bool
		wantID   string
		wantName string
	}{
		{"initial rows", false, "queue-1", "Song 01"},
		{"scrolled rows", true, "queue-4", "Song 04"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := testApp(120, 36, PagePlayback)
			for i := 0; i < 40; i++ {
				song := sampleSong()
				song.CLIID = fmt.Sprintf("queue-%d", i)
				song.Name = fmt.Sprintf("Song %02d", i)
				app.queue = append(app.queue, song)
			}
			app.queueLoaded = true
			app.queueIndex = 0
			app.state = alx.PlayerState{CurrentSong: &app.queue[0], Status: "playing", QueueLength: 40}
			lines := renderView(t, app)
			if app.queueArea.Empty() {
				t.Fatal("playback queue is not visible")
			}
			if tc.scroll {
				app.Update(tea.MouseMsg{
					X: app.queueArea.X, Y: app.queueArea.Y,
					Action: tea.MouseActionPress, Button: tea.MouseButtonWheelDown,
				})
				lines = renderView(t, app)
			}
			x := app.queueArea.X + app.queueArea.W - 1
			y := app.queueArea.Y + 1
			if !strings.Contains(ansi.Strip(lines[y]), tc.wantName) {
				t.Fatalf("clicked row does not display %q: %q", tc.wantName, ansi.Strip(lines[y]))
			}
			_, cmd := app.Update(tea.MouseMsg{
				X: x, Y: y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
			})
			if cmd == nil || !app.playPending || app.pendingSong.CLIID != tc.wantID {
				t.Fatalf("click did not request visible song %q: pending=%v song=%+v command=%v", tc.wantID, app.playPending, app.pendingSong, cmd != nil)
			}
			if len(app.queue) != 40 || app.queueIndex != 0 {
				t.Fatalf("click changed queue before playback succeeded: length=%d index=%d", len(app.queue), app.queueIndex)
			}
		})
	}
}

func TestPlaybackQueueBlankRowsDoNotPlay(t *testing.T) {
	app := testApp(120, 36, PagePlayback)
	app.queue = []alx.Song{sampleSong()}
	app.queueLoaded = true
	renderView(t, app)
	_, cmd := app.Update(tea.MouseMsg{
		X: app.queueArea.X, Y: app.queueArea.Y + 1,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	})
	if cmd != nil || app.playPending {
		t.Fatal("clicking an empty queue row requested playback")
	}
}

func TestPlaybackQueueIgnoresClicksWhileLoading(t *testing.T) {
	app := testApp(120, 36, PagePlayback)
	first, second := sampleSong(), sampleSong()
	second.CLIID = "other"
	app.queue = []alx.Song{first, second}
	app.queueLoaded = true
	renderView(t, app)
	click := tea.MouseMsg{
		X: app.queueArea.X, Y: app.queueArea.Y,
		Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
	}
	if _, cmd := app.Update(click); cmd == nil || !app.playPending {
		t.Fatal("first click did not start loading")
	}
	seq := app.playSeq
	click.Y++
	_, cmd := app.Update(click)
	if cmd != nil || app.playSeq != seq || app.pendingSong.CLIID != first.CLIID {
		t.Fatal("a second queue click started a competing playback request")
	}
}

func TestPlaybackQueueClickPreservesQueue(t *testing.T) {
	for _, tc := range []struct {
		name     string
		response string
		exitCode int
		wantErr  bool
	}{
		{"success with duplicate song", `{"status":"playing"}`, 0, false},
		{"backend failure", `{"error":"cannot load song"}`, 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := testApp(120, 36, PagePlayback)
			client, logPath := testBackend(t, tc.response, tc.exitCode)
			app.client = client
			first := sampleSong()
			second := sampleSong()
			second.CLIID = "other"
			app.queue = []alx.Song{first, second, first}
			app.queueLoaded = true
			app.queueIndex = 0
			app.state = alx.PlayerState{CurrentSong: &first, Status: "paused", QueueLength: 3, Shuffle: true}
			app.queueManualUntil = app.clock.Add(8 * time.Second)
			wantQueue := append([]alx.Song(nil), app.queue...)
			renderView(t, app)
			_, cmd := app.Update(tea.MouseMsg{
				X: app.queueArea.X, Y: app.queueArea.Y + 2,
				Action: tea.MouseActionPress, Button: tea.MouseButtonLeft,
			})
			if cmd == nil {
				t.Fatal("queue click did not request playback")
			}
			app.Update(cmd())
			wantArgs := []string{"queue", "play", "3", "--json", "--no-color"}
			if args := testBackendArgs(t, logPath); !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("queue click command = %q, want %q", args, wantArgs)
			}
			if app.playPending || !app.queueLoaded || !reflect.DeepEqual(app.queue, wantQueue) {
				t.Fatalf("queue was replaced or playback remained pending: pending=%v loaded=%v queue=%+v", app.playPending, app.queueLoaded, app.queue)
			}
			if tc.wantErr {
				if app.queueIndex != 0 || app.state.Status != "paused" || !strings.Contains(app.toast.text, "cannot load song") {
					t.Fatalf("failed playback changed current item or hid error: index=%d status=%q toast=%q", app.queueIndex, app.state.Status, app.toast.text)
				}
				return
			}
			if app.queueIndex != 2 || app.state.QueueLength != 3 || app.state.Status != "playing" || !app.state.Shuffle {
				t.Fatalf("queue playback did not select the duplicate at index 2: index=%d state=%+v", app.queueIndex, app.state)
			}
			if !app.queueLoading || !app.queueManualUntil.IsZero() {
				t.Fatal("queue playback did not refresh queue and resume following current item")
			}
		})
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
