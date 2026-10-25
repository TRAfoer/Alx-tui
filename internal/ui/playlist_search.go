package ui

import (
	"fmt"
	"strings"

	"alxtui/internal/alx"
	tea "github.com/charmbracelet/bubbletea"
)

type searchMode int

const (
	searchSongs searchMode = iota
	searchPlaylists
)

// updateSearchInput handles both keys and asynchronous clipboard messages so
// every edit invalidates results and requests for the previous playlist query.
func (a *App) updateSearchInput(msg tea.Msg) tea.Cmd {
	before := a.searchInput.Value()
	var cmd tea.Cmd
	a.searchInput, cmd = a.searchInput.Update(msg)
	if a.searchMode == searchPlaylists && before != a.searchInput.Value() {
		a.searchSeq++
		a.previewSeq++
		a.searchLoading = false
		a.searchErr = nil
		a.searchSubmitted = ""
		a.searchSubmittedSource = ""
		a.remotePlaylists = nil
		a.playlistSearchHasMore = false
		a.playlistSearchTotal = 0
		a.searchWarnings = nil
		a.searchSelected = 0
		a.searchScroll = 0
		a.searchPage = 1
	}
	return cmd
}

func (a *App) activeSearchSource() string {
	if a.searchMode == searchPlaylists {
		if a.playlistSearchSource == "" {
			return "all"
		}
		return a.playlistSearchSource
	}
	return a.searchSource
}

func (a *App) setSearchMode(mode searchMode) tea.Cmd {
	if mode == a.searchMode {
		return nil
	}
	a.searchMode = mode
	if mode == searchPlaylists && a.playlistSearchSource == "" {
		a.playlistSearchSource = "all"
	}
	a.searchSeq++
	a.backSearchPlaylists()
	a.searchLoading = false
	a.searchErr = nil
	a.searchWarnings = nil
	a.searchResults = nil
	a.remotePlaylists = nil
	a.searchSubmitted = ""
	a.searchSubmittedSource = ""
	a.searchSelected = 0
	a.searchScroll = 0
	a.searchPage = 1
	a.playlistSearchHasMore = false
	return a.focusSearch()
}

func (a *App) canSearchNext() bool {
	if a.searchLoading || a.searchSubmitted == "" || a.playlistPreview {
		return false
	}
	if a.searchMode == searchPlaylists {
		return a.playlistSearchHasMore
	}
	return len(a.searchResults) >= a.searchLimit
}

func (a *App) searchCount() int {
	if a.searchMode == searchPlaylists {
		return len(a.remotePlaylists)
	}
	return len(a.searchResults)
}

func (a *App) handlePlaylistSearchResult(msg playlistSearchMsg) {
	if a.searchMode != searchPlaylists || a.playlistPreview || msg.seq != a.searchSeq || msg.keyword != strings.TrimSpace(a.searchInput.Value()) || msg.source != a.activeSearchSource() || msg.page != a.searchPage {
		return
	}
	a.searchLoading = false
	a.searchErr = msg.err
	a.searchWarnings = msg.result.Warnings
	if msg.err != nil {
		a.remotePlaylists = nil
		a.playlistSearchHasMore = false
		return
	}
	a.remotePlaylists = msg.result.Playlists
	a.playlistSearchHasMore = msg.result.HasMore
	a.playlistSearchTotal = msg.result.Total
	a.searchSelected = 0
	a.searchScroll = 0
}

func (a *App) openSearchPlaylist(index int) tea.Cmd {
	if a.searchMode != searchPlaylists || a.searchLoading || index < 0 || index >= len(a.remotePlaylists) {
		return nil
	}
	a.searchSelected = index
	a.searchSeq++
	a.previewSeq++
	a.playlistPreview = true
	a.previewLink = a.remotePlaylists[index].Link
	a.preview = alx.ImportedPlaylist{Name: a.remotePlaylists[index].Name}
	a.previewLoading = true
	a.previewErr = nil
	a.previewSelected = 0
	a.previewScroll = 0
	a.blurSearch()
	return cmdInspectPlaylist(a.client, a.previewSeq, a.previewLink)
}

func (a *App) handlePlaylistPreview(msg playlistPreviewMsg) {
	if a.searchMode != searchPlaylists || !a.playlistPreview || msg.seq != a.previewSeq || msg.link != a.previewLink || strings.TrimSpace(a.searchInput.Value()) != a.searchSubmitted || a.searchSelected < 0 || a.searchSelected >= len(a.remotePlaylists) || a.remotePlaylists[a.searchSelected].Link != msg.link {
		return
	}
	a.previewLoading = false
	a.previewErr = msg.err
	if msg.err == nil {
		a.preview = msg.result
	}
}

func (a *App) backSearchPlaylists() {
	a.previewSeq++
	a.playlistPreview = false
	a.previewLoading = false
	a.previewErr = nil
	a.preview = alx.ImportedPlaylist{}
	a.previewLink = ""
	a.previewSelected = 0
	a.previewScroll = 0
	a.blurSearch()
}

func (a *App) openSearchPlaylistImport() tea.Cmd {
	var link, name string
	if a.playlistPreview {
		link, name = a.previewLink, a.preview.Name
	} else if a.searchSelected >= 0 && a.searchSelected < len(a.remotePlaylists) && !a.searchLoading {
		p := a.remotePlaylists[a.searchSelected]
		link, name = p.Link, p.Name
	}
	if link == "" {
		return nil
	}
	a.menu = nil
	a.blurSearch()
	a.dialogSeq++
	a.dlg = &dialogState{kind: dialogImportPlaylistName, seq: a.dialogSeq, title: "导入在线歌单", label: "本地歌单名称", target: link, hint: "Enter 导入；如有同名歌单，可修改名称后重试。", input: a.newDialogInput(name), hasInput: true}
	return a.dlg.input.Focus()
}

func (a *App) renderSearchModes(x, y, width int) string {
	line := ""
	for i, label := range []string{"歌曲", "歌单"} {
		chip := a.th.ActiveChip(label, a.searchMode == searchMode(i))
		a.addHit(HitSearchMode, Rect{X: x + displayWidth(line), Y: y, W: displayWidth(chip), H: 1}, "", i, alx.Song{})
		line += chip + " "
	}
	return padVisible(line+a.th.FaintText("Ctrl+T 切换"), width)
}

func (a *App) remotePlaylistBody(x, y, width, height int) []string {
	if height <= 0 {
		return nil
	}
	if a.searchLoading || a.searchErr != nil || a.searchSubmitted == "" {
		return a.searchBodyState(width, height)
	}
	if len(a.remotePlaylists) == 0 {
		return blankState(height, width, a.th.Bold(a.th.Text, "没有找到歌单"), a.th.FaintText("试试更换关键词或平台"))
	}
	out := make([]string, height)
	clampScroll(&a.searchScroll, len(a.remotePlaylists), height)
	start, end := windowBounds(len(a.remotePlaylists), a.searchScroll, height)
	for i := start; i < end; i++ {
		p := a.remotePlaylists[i]
		marker := "  "
		if i == a.searchSelected {
			marker = "▸ "
		}
		meta := fmt.Sprintf("%s · %d首 · %d次", sourceCodeLabel(p.Source), p.Count, p.PlayCount)
		nameWidth := max(8, width-displayWidth(meta)-3)
		name := p.Name
		if p.Author != "" {
			name += " / " + p.Author
		}
		out[i-start] = padVisible(marker+a.th.AccentText(truncate(prettyText(name), nameWidth)), width-displayWidth(meta)) + a.th.FaintText(meta)
		out[i-start] = padVisible(out[i-start], width)
		a.addRowHit(HitRemotePlaylist, x, y+i-start, width, p.Link, i, alx.Song{})
	}
	return out
}

func (a *App) searchBodyState(width, height int) []string {
	if a.searchLoading {
		return blankState(height, width, a.th.AccentText("正在搜索歌单…"))
	}
	if a.searchErr != nil {
		return verticalCenter([]string{a.th.BadText("搜索失败"), a.th.FaintText(truncate(prettyText(a.searchErr.Error()), width)), a.th.FaintText("按 r 重试，或更换平台")}, height, width)
	}
	return blankState(height, width, a.th.AccentText("搜索在线歌单"), a.th.FaintText("输入关键词，Enter 搜索 · 网易云 / QQ"))
}

func (a *App) renderPlaylistPreview(r Rect) []string {
	x, w := r.X+2, r.W-4
	if w < 24 {
		return []string{padVisible(a.th.Dim("窗口太窄"), r.W)}
	}
	lines := []string{a.th.Bold(a.th.Accent, "歌单预览 · "+truncate(prettyText(a.preview.Name), max(4, w-14)))}
	controls := a.th.AccentText("‹ 返回 (Esc)") + "  " + a.th.AccentText("导入 (Ctrl+O)")
	a.addHit(HitSearchBack, Rect{X: x, Y: r.Y + 1, W: displayWidth("‹ 返回 (Esc)"), H: 1}, "", 0, alx.Song{})
	a.addHit(HitSearchImport, Rect{X: x + displayWidth("‹ 返回 (Esc)") + 2, Y: r.Y + 1, W: displayWidth("导入 (Ctrl+O)"), H: 1}, "", 0, alx.Song{})
	lines = append(lines, padVisible(controls, w))
	lines = append(lines, a.th.FaintText(truncate("Enter 播放 · 右键 / m 更多 · 预览不保存歌单", w)))
	if len(a.preview.Warnings) > 0 {
		lines = append(lines, a.th.FaintText(truncate(strings.Join(a.preview.Warnings, "; "), w)))
	}
	lines = append(lines, newSongColumns(w, true).header(a.th))
	h := max(0, r.H-len(lines))
	a.searchResultsArea = Rect{X: x, Y: r.Y + len(lines), W: w, H: h}
	var body []string
	switch {
	case a.previewLoading:
		body = blankState(h, w, a.th.AccentText("正在读取歌单…"))
	case a.previewErr != nil:
		body = blankState(h, w, a.th.BadText("预览失败"), a.th.FaintText(truncate(prettyText(a.previewErr.Error()), w)), a.th.FaintText("r 重试 · Esc 返回"))
	case len(a.preview.Songs) == 0:
		body = blankState(h, w, a.th.FaintText("歌单暂无可用歌曲"))
	default:
		body = a.renderSongRows(x, r.Y+len(lines), w, h, a.preview.Songs, a.previewSelected, &a.previewScroll, a.currentSongID(), HitSearchResult, "preview")
	}
	lines = append(lines, body...)
	for i := range lines {
		lines[i] = spaces(x) + padVisible(lines[i], w)
	}
	if len(lines) > r.H {
		lines = lines[:r.H]
	}
	return lines
}

func (a *App) handlePlaylistPreviewKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc", "backspace":
		a.backSearchPlaylists()
	case "ctrl+o":
		return a.openSearchPlaylistImport()
	case "up":
		a.previewSelected = clamp(a.previewSelected-1, 0, max(0, len(a.preview.Songs)-1))
		ensureVisible(a.previewSelected, &a.previewScroll, max(1, a.searchResultsArea.H), len(a.preview.Songs))
	case "down":
		a.previewSelected = clamp(a.previewSelected+1, 0, max(0, len(a.preview.Songs)-1))
		ensureVisible(a.previewSelected, &a.previewScroll, max(1, a.searchResultsArea.H), len(a.preview.Songs))
	case "enter":
		if !a.previewLoading && a.previewErr == nil && a.previewSelected < len(a.preview.Songs) {
			return a.startPlay(a.preview.Songs[a.previewSelected])
		}
	case "r":
		a.previewSeq++
		a.previewLoading = true
		a.previewErr = nil
		return cmdInspectPlaylist(a.client, a.previewSeq, a.previewLink)
	case "m":
		if a.previewSelected < len(a.preview.Songs) {
			a.openSongMenu(a.preview.Songs[a.previewSelected], "preview", a.searchResultsArea.X, a.searchResultsArea.Y)
		}
	}
	return nil
}

func (a *App) activateSearchSelection() tea.Cmd {
	if a.searchMode == searchPlaylists {
		return a.openSearchPlaylist(a.searchSelected)
	}
	return a.playSelectedSong()
}
