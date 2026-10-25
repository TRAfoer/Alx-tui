package ui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"alxtui/internal/alx"
)

func TestPlaylistImportKeyboardOpensInput(t *testing.T) {
	app := testApp(120, 30, PagePlaylists)
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if app.dlg == nil || !app.dlg.hasInput {
		t.Fatal("i did not open a playlist link input")
	}
	if !strings.Contains(app.dlg.label, "链接") {
		t.Fatalf("dialog does not ask for a link: %q", app.dlg.label)
	}
}

func TestPlaylistImportSuccessRefreshesAndFocusesPlaylist(t *testing.T) {
	app := testApp(120, 30, PagePlaylists)
	client, logPath := testBackend(t, `{"name":"通勤","count":12,"source":"wy","warnings":["有一首歌曲不可用"]}`, 0)
	app.client = client
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if app.dlg == nil {
		t.Fatal("import dialog did not open")
	}
	link := "分享歌单 https://music.163.com/playlist?id=42"
	app.dlg.input.SetValue(link)
	app.handleDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if app.dlg == nil || !strings.Contains(app.dlg.label, "名称") {
		t.Fatal("link entry did not advance to the optional playlist name")
	}
	cmd := app.handleDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil || !app.dlg.busy {
		t.Fatal("blank optional name did not start the import")
	}
	if duplicate := app.handleDialogKey(tea.KeyMsg{Type: tea.KeyEnter}); duplicate != nil {
		t.Fatal("busy dialog allowed a duplicate import")
	}
	_, refresh := app.Update(cmd())
	if app.dlg != nil || refresh == nil || !app.playlistsLoading {
		t.Fatalf("success did not close and refresh: dialog=%v refresh=%v loading=%v", app.dlg != nil, refresh != nil, app.playlistsLoading)
	}
	if app.focusPlaylistName != "通勤" || !strings.Contains(app.toast.text, "12") || !strings.Contains(app.toast.text, "有一首歌曲不可用") {
		t.Fatalf("import outcome not shown: focus=%q toast=%q", app.focusPlaylistName, app.toast.text)
	}
	want := []string{"playlist", "import", link, "--json", "--no-color"}
	if got := testBackendArgs(t, logPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("import args = %q, want %q", got, want)
	}
	app.Update(playlistsMsg{playlists: []alx.Playlist{{Name: "Favorites"}, {Name: "通勤", Count: 12}}})
	if app.selectedPlaylistName() != "通勤" || !app.playlistSongsLoading {
		t.Fatal("refreshed list did not select and load the imported playlist")
	}
}

func TestPlaylistImportBackendErrorAllowsRenameAndRetry(t *testing.T) {
	app := testApp(120, 30, PagePlaylists)
	client, logPath := testBackend(t, `{"error":"歌单已存在，请指定其它名称"}`, 1)
	app.client = client
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if app.dlg == nil {
		t.Fatal("import dialog did not open")
	}
	app.dlg.input.SetValue("https://y.qq.com/n/ryqq/playlist/42")
	app.handleDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	app.dlg.input.SetValue("旧名称")
	cmd := app.handleDialogKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("named import did not start")
	}
	app.Update(cmd())
	if app.dlg == nil || app.dlg.busy || !strings.Contains(app.dlg.err, "歌单已存在") {
		t.Fatalf("failed import left input unavailable: %+v", app.dlg)
	}
	want := []string{"playlist", "import", "https://y.qq.com/n/ryqq/playlist/42", "--name", "旧名称", "--json", "--no-color"}
	if got := testBackendArgs(t, logPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("named import args = %q, want %q", got, want)
	}
	app.dlg.input.SetValue("新名称")
	if retry := app.handleDialogKey(tea.KeyMsg{Type: tea.KeyEnter}); retry == nil || !app.dlg.busy {
		t.Fatal("failed import could not retry with another name")
	}
}

func TestPlaylistImportButtonVisibleAtSupportedWidths(t *testing.T) {
	for _, width := range []int{56, 70, 120} {
		height := 24
		if width == 56 {
			height = 16
		}
		app := testApp(width, height, PagePlaylists)
		view := strings.Join(renderView(t, app), "\n")
		if !strings.Contains(ansi.Strip(view), "导入链接") {
			t.Errorf("width %d has no visible import link button", width)
		}
		hits := findHit(app, HitPlaylistImport)
		if len(hits) != 1 {
			t.Fatalf("width %d has %d import click targets, want 1", width, len(hits))
		}
		hit := hits[0]
		app.handleMouse(tea.MouseMsg{X: hit.Rect.X, Y: hit.Rect.Y, Action: tea.MouseActionPress, Button: tea.MouseButtonLeft})
		if app.dlg == nil || app.dlg.kind != dialogImportPlaylist {
			t.Errorf("width %d import button did not open link input", width)
		}
		renderView(t, app)
		app.dlg.input.SetValue("https://music.163.com/playlist?id=42")
		app.submitDialog()
		renderView(t, app)
	}
}

func TestPlaylistImportTimeoutUnlocksDialogAndIgnoresStaleResults(t *testing.T) {
	app := testApp(120, 30, PagePlaylists)
	app.openImportPlaylistDialog()
	app.dlg.input.SetValue("https://music.163.com/playlist?id=42")
	app.submitDialog()
	app.submitDialog()
	seq := app.dlg.seq
	app.Update(importPlaylistMsg{seq: seq, err: context.DeadlineExceeded})
	if app.dlg == nil || app.dlg.busy || app.dlg.err == "" {
		t.Fatalf("timeout did not unlock import name input: %+v", app.dlg)
	}
	app.closeDialog()
	app.openNewPlaylistDialog(nil)
	newSeq := app.dlg.seq
	app.Update(importPlaylistMsg{seq: seq, res: alx.PlaylistImportResult{Name: "迟到的歌单", Count: 1, Source: "wy"}})
	if app.dlg == nil || app.dlg.seq != newSeq || app.dlg.kind != dialogNewPlaylist {
		t.Fatal("late import result closed an unrelated dialog")
	}
}

func TestPlaylistImportRefreshWaitsForOlderListRequest(t *testing.T) {
	app := testApp(120, 30, PagePlaylists)
	app.playlistsLoading = true
	app.Update(importPlaylistMsg{res: alx.PlaylistImportResult{Name: "新歌单", Count: 1, Source: "wy"}})
	_, refresh := app.Update(playlistsMsg{playlists: []alx.Playlist{{Name: "Favorites"}}})
	if refresh == nil || !app.playlistsLoading || app.focusPlaylistName != "新歌单" {
		t.Fatalf("in-flight refresh lost the imported playlist: refresh=%v loading=%v focus=%q", refresh != nil, app.playlistsLoading, app.focusPlaylistName)
	}
	app.Update(playlistsMsg{playlists: []alx.Playlist{{Name: "Favorites"}, {Name: "新歌单", Count: 1}}})
	if app.selectedPlaylistName() != "新歌单" {
		t.Fatalf("imported playlist was not selected after refresh: %q", app.selectedPlaylistName())
	}
}
