package ui

import (
	"errors"
	"strings"
	"testing"

	"alxtui/internal/alx"
	tea "github.com/charmbracelet/bubbletea"
)

func playlistSearchApp() *App {
	a := testApp(120, 36, PageSearch)
	a.searchInput.SetValue("night")
	a.setSearchMode(searchPlaylists)
	a.startSearch(1)
	a.handlePlaylistSearchResult(playlistSearchMsg{seq: a.searchSeq, keyword: "night", source: "all", page: 1, result: alx.PlaylistSearchResult{Playlists: []alx.RemotePlaylist{{ID: "1", Name: "Night", Author: "DJ", Source: "wy", Link: "https://music.163.com/playlist?id=1", Count: 12}}, Page: 1, Limit: 50, HasMore: true, Warnings: []string{"tx unavailable"}}})
	return a
}

func TestPlaylistSearchModeAndPagination(t *testing.T) {
	a := playlistSearchApp()
	if a.searchSource != "wy" || a.playlistSearchSource != "all" {
		t.Fatal("song config source changed")
	}
	renderView(t, a)
	if len(findHit(a, HitSearchMode)) != 2 || len(findHit(a, HitSearchSource)) != 3 || len(findHit(a, HitRemotePlaylist)) != 1 || len(findHit(a, HitSearchNext)) != 1 {
		t.Fatal("missing mode, source, row or has_more pagination hits")
	}
	if !strings.Contains(a.View(), "tx unavailable") {
		t.Fatal("partial source warning hidden")
	}
	a.handleSearchKey(tea.KeyMsg{Type: tea.KeyPgDown}, false)
	if a.searchPage != 2 || !a.searchLoading {
		t.Fatal("has_more did not allow next page")
	}
	a.handlePlaylistSearchResult(playlistSearchMsg{seq: a.searchSeq, keyword: "night", source: "all", page: 2, result: alx.PlaylistSearchResult{Playlists: make([]alx.RemotePlaylist, 50), Page: 2}})
	if a.canSearchNext() {
		t.Fatal("ignored has_more=false")
	}
	a.setSearchMode(searchSongs)
	if a.searchSource != "wy" {
		t.Fatal("song source not restored")
	}
}

func TestPlaylistPreviewBackImportAndStaleReplies(t *testing.T) {
	a := playlistSearchApp()
	a.openSearchPlaylist(0)
	seq := a.previewSeq
	a.handlePlaylistPreview(playlistPreviewMsg{seq: seq, link: a.previewLink, result: alx.ImportedPlaylist{Name: "Night", Songs: []alx.Song{sampleSong()}, Warnings: []string{"skipped"}}})
	if !a.playlistPreview || a.previewLoading || len(a.preview.Songs) != 1 {
		t.Fatal("preview not loaded")
	}
	renderView(t, a)
	if len(findHit(a, HitSearchResult)) != 1 || len(findHit(a, HitSearchBack)) != 1 || len(findHit(a, HitSearchImport)) != 1 {
		t.Fatal("preview controls missing")
	}
	a.openSearchPlaylistImport()
	if a.dlg == nil || a.dlg.kind != dialogImportPlaylistName || a.dlg.target != a.previewLink || a.dlg.input.Value() != "Night" {
		t.Fatal("import not prefilled")
	}
	a.dlg = nil
	a.backSearchPlaylists()
	a.handlePlaylistPreview(playlistPreviewMsg{seq: seq, link: "https://music.163.com/playlist?id=1", err: errors.New("stale")})
	if a.playlistPreview || a.previewErr != nil {
		t.Fatal("back accepted stale preview")
	}
	old := a.searchSeq
	a.searchInput.SetValue("new")
	a.handlePlaylistSearchResult(playlistSearchMsg{seq: old, keyword: "night", source: "all", page: 1, err: errors.New("stale")})
	if a.searchErr != nil {
		t.Fatal("changed query accepted stale reply")
	}
	a.setSearchMode(searchSongs)
	a.handlePlaylistSearchResult(playlistSearchMsg{seq: a.searchSeq, keyword: "new", source: "all", err: errors.New("stale")})
	if a.searchErr != nil {
		t.Fatal("song mode accepted playlist result")
	}
}

func TestPlaylistSearchViewsFit(t *testing.T) {
	for _, w := range []int{48, 80, 120} {
		a := playlistSearchApp()
		a.width = w
		renderView(t, a)
		a.openSearchPlaylist(0)
		renderView(t, a)
		a.handlePlaylistPreview(playlistPreviewMsg{seq: a.previewSeq, link: a.previewLink, err: errors.New("inspect failed")})
		renderView(t, a)
	}
}

func TestPlaylistPreviewRejectsChangedSelectionAndQuery(t *testing.T) {
	for _, change := range []string{"selection", "query"} {
		a := playlistSearchApp()
		a.remotePlaylists = append(a.remotePlaylists, alx.RemotePlaylist{ID: "2", Link: "other"})
		a.openSearchPlaylist(0)
		if change == "selection" {
			a.searchSelected = 1
		} else {
			a.searchInput.SetValue("changed")
		}
		a.handlePlaylistPreview(playlistPreviewMsg{seq: a.previewSeq, link: a.previewLink, result: alx.ImportedPlaylist{Name: "stale", Songs: []alx.Song{sampleSong()}}})
		if a.preview.Name == "stale" {
			t.Fatalf("accepted preview after changed %s", change)
		}
	}
}

func TestPlaylistModeStaleSongReplyDoesNotFocusInput(t *testing.T) {
	a := playlistSearchApp()
	a.blurSearch()
	_, cmd := a.update(searchMsg{seq: a.searchSeq, keyword: "night", source: "wy", page: 1, songs: []alx.Song{sampleSong()}})
	if cmd != nil {
		t.Fatal("stale song reply refocused playlist input")
	}
}

func TestPlaylistKeyboardMouseAndSongActions(t *testing.T) {
	a := testApp(120, 36, PageSearch)
	a.handleKey(tea.KeyMsg{Type: tea.KeyCtrlT})
	if a.searchMode != searchPlaylists {
		t.Fatal("Ctrl+T did not switch mode")
	}
	a = playlistSearchApp()
	a.chooseSearchSource("kg")
	if a.activeSearchSource() != "all" {
		t.Fatal("unsupported playlist source selected")
	}
	renderView(t, a)
	row := findHit(a, HitRemotePlaylist)[0]
	a.handleMouse(tea.MouseMsg{X: row.Rect.X, Y: row.Rect.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if !a.previewLoading {
		t.Fatal("click did not inspect")
	}
	a.Update(playlistPreviewMsg{seq: a.previewSeq, link: a.previewLink, result: alx.ImportedPlaylist{Name: "Night", Songs: []alx.Song{sampleSong()}}})
	renderView(t, a)
	song := findHit(a, HitSearchResult)[0]
	a.handleMouse(tea.MouseMsg{X: song.Rect.X, Y: song.Rect.Y, Button: tea.MouseButtonRight, Action: tea.MouseActionPress})
	if a.menu == nil || a.menu.song.CLIID != sampleSong().CLIID {
		t.Fatal("preview did not reuse song menu/cache identity")
	}
	for _, item := range a.menu.main {
		if item.action == menuRemoveFromPlaylist {
			t.Fatal("preview exposes remove-from-playlist")
		}
	}
	a.menu = nil
	a.handleKey(tea.KeyMsg{Type: tea.KeyCtrlO})
	if a.dlg == nil || a.dlg.kind != dialogImportPlaylistName {
		t.Fatal("Ctrl+O did not open import name")
	}
	a.dlg = nil
	renderView(t, a)
	back := findHit(a, HitSearchBack)[0]
	a.handleMouse(tea.MouseMsg{X: back.Rect.X, Y: back.Rect.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if a.playlistPreview || len(a.remotePlaylists) != 1 {
		t.Fatal("back did not preserve search results")
	}
	renderView(t, a)
	imp := findHit(a, HitSearchImport)[0]
	a.handleMouse(tea.MouseMsg{X: imp.Rect.X, Y: imp.Rect.Y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if a.dlg == nil || a.dlg.input.Value() != "Night" {
		t.Fatal("selected row import mouse failed")
	}
}

func TestPlaylistSearchErrorEmptyAndRetry(t *testing.T) {
	a := playlistSearchApp()
	a.startSearch(1)
	a.Update(playlistSearchMsg{seq: a.searchSeq, keyword: "night", source: "all", page: 1, err: errors.New("catalog failed")})
	if a.searchLoading || a.searchErr == nil {
		t.Fatal("search error not handled")
	}
	if !strings.Contains(a.View(), "catalog failed") {
		t.Fatal("search error hidden")
	}
	a.blurSearch()
	if cmd := a.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}}, false); cmd == nil || !a.searchLoading {
		t.Fatal("retry missing")
	}
	a.Update(playlistSearchMsg{seq: a.searchSeq, keyword: "night", source: "all", page: 1, result: alx.PlaylistSearchResult{Playlists: []alx.RemotePlaylist{}}})
	if a.searchLoading || a.searchErr != nil || len(a.remotePlaylists) != 0 {
		t.Fatal("empty search not applied")
	}
	if !strings.Contains(a.View(), "没有找到歌单") {
		t.Fatal("empty state hidden")
	}
}

func TestPlaylistQueryEditClearsOutdatedResults(t *testing.T) {
	a := playlistSearchApp()
	a.searchFocus = true
	a.searchInput.Focus()
	a.handleSearchKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, true)
	if len(a.remotePlaylists) != 0 || a.searchSubmitted != "" || a.playlistSearchHasMore || len(a.searchWarnings) != 0 {
		t.Fatal("query edit retained outdated playlist results or pagination")
	}
	if cmd := a.openSearchPlaylist(0); cmd != nil || a.playlistPreview {
		t.Fatal("query edit allowed previewing an old result")
	}
	renderView(t, a)
	if len(findHit(a, HitRemotePlaylist)) != 0 || len(findHit(a, HitSearchNext)) != 0 {
		t.Fatal("outdated results remain clickable")
	}
	a.searchInput.SetValue("night")
	if cmd := a.handleSearchKey(tea.KeyMsg{Type: tea.KeyEnter}, true); cmd == nil || !a.searchLoading || a.searchSubmitted != "night" {
		t.Fatal("reverting edited query did not start a fresh search")
	}
}
