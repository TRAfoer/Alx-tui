package ui

import (
	"alxtui/internal/alx"
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) renderPlaylistPage(r Rect) []string {
	t := a.th
	if r.H <= 0 || r.W <= 0 {
		return nil
	}
	const pad = 2
	innerX := r.X + pad
	innerW := r.W - pad*2
	if innerW < 24 {
		return []string{padVisible(t.Dim("窗口太窄"), r.W)}
	}

	lines := make([]string, 0, r.H)
	right := t.FaintText(fmt.Sprintf("共 %d 个歌单", len(a.playlists)))
	if a.playlistsLoading {
		right = t.AccentText(a.th.Spinner(a.spinner) + " 同步中")
	}
	heading := t.Bold(t.Accent, "歌单") + "  " + t.FaintText("PLAYLISTS")
	lines = append(lines, spaces(innerX)+padVisible(composeLR(heading, right, innerW), innerW))
	lines = append(lines, spaces(innerX)+t.Sep(innerW))

	if len(lines) >= r.H {
		return lines[:r.H]
	}
	bodyH := r.H - len(lines)
	bodyY := r.Y + len(lines)

	wide := innerW >= 78
	leftW := 0
	rightX := innerX
	rightW := innerW
	if wide {
		leftW = clamp(innerW*30/100, 26, 40)
		rightW = innerW - leftW - 5
		if rightW < 32 {
			wide = false
			leftW = 0
			rightW = innerW
		} else {
			rightX = innerX + leftW + 5
		}
	}

	if !wide {
		narrowLines := a.renderNarrowPlaylistBody(innerX, bodyY, innerW, bodyH)
		for _, line := range narrowLines {
			lines = append(lines, spaces(innerX)+line)
		}
		if len(lines) > r.H {
			lines = lines[:r.H]
		}
		return lines
	}

	leftLines := make([]string, bodyH)
	rightLines := make([]string, bodyH)
	for i := range leftLines {
		leftLines[i] = spaces(leftW)
		rightLines[i] = spaces(rightW)
	}

	// Left heading and playlist rows.
	leftHead := t.Bold(t.Accent, "我的歌单")
	if bodyH > 0 {
		leftLines[0] = padVisible(leftHead, leftW)
	}
	listH := max(0, bodyH-2)
	listStartRow := 1
	if listH > 0 {
		if a.playlistsErr != nil {
			leftLines[listStartRow] = padVisible(t.BadText("加载失败"), leftW)
		} else if len(a.playlists) == 0 {
			leftLines[listStartRow] = padVisible(t.FaintText("还没有歌单"), leftW)
		} else {
			clampScroll(&a.playlistListScroll, len(a.playlists), listH)
			start, end := windowBounds(len(a.playlists), a.playlistListScroll, listH)
			for i := start; i < end; i++ {
				row := a.renderPlaylistListRow(a.playlists[i], i == a.playlistSelected, leftW)
				leftLines[listStartRow+i-start] = row
				a.addRowHit(HitPlaylistRow, innerX, bodyY+listStartRow+i-start, leftW, "", i, alx.Song{})
			}
			a.playlistListArea = Rect{X: innerX, Y: bodyY + listStartRow, W: leftW, H: listH}
		}
	}
	// Playlist creation and import actions on the last row of the left column.
	if bodyH > 0 {
		newY := bodyY + bodyH - 1
		newLabel := a.renderButtons(innerX, newY, []actionButton{
			{label: "＋ 新建歌单", kind: HitPlaylistNew, primary: true},
			{label: "导入链接", kind: HitPlaylistImport},
		})
		leftLines[bodyH-1] = padVisible(newLabel, leftW)
	}

	// Right header with actions.
	name := a.selectedPlaylistName()
	if name == "" {
		name = "未选择歌单"
	}
	songCount := len(a.playlistSongs)
	if name != a.playlistName {
		songCount = 0
	}
	actions := []actionButton{
		{label: "▶ 播放", kind: HitPlaylistPlayAll, primary: true, disabled: name == "未选择歌单" || songCount == 0},
		{label: "重命名", kind: HitPlaylistRename, disabled: name == "未选择歌单"},
		{label: "删除", kind: HitPlaylistDelete, danger: true, disabled: name == "未选择歌单" || name == "Favorites"},
		{label: "⟳", kind: HitPlaylistRefresh},
	}
	if buttonWidths(actions) > rightW-10 {
		actions = []actionButton{
			{label: "▶ 播放", kind: HitPlaylistPlayAll, primary: true, disabled: name == "未选择歌单" || songCount == 0},
			{label: "⟳", kind: HitPlaylistRefresh},
		}
	}
	actionsW := buttonWidths(actions)
	actionsX := rightX + rightW - actionsW
	actionsStr := a.renderButtons(actionsX, bodyY, actions)
	title := t.Bold(t.Accent, truncate(prettyText(name), max(4, rightW-actionsW-8)))
	if name != "未选择歌单" {
		title += t.FaintText(fmt.Sprintf(" · %d 首", songCount))
	}
	if bodyH > 0 {
		leftPart := padVisible(title, max(0, rightW-actionsW-1))
		rightLines[0] = padVisible(leftPart+" "+actionsStr, rightW)
	}
	if bodyH > 1 {
		rightLines[1] = t.Sep(rightW)
	}

	rowsH := max(0, bodyH-3)
	if bodyH > 2 {
		cols := newSongColumns(rightW, true)
		rightLines[2] = padVisible(cols.header(t), rightW)
	}
	if rowsH > 0 {
		var rows []string
		switch {
		case a.playlistSongsLoading:
			rows = blankState(rowsH, rightW, a.th.Spinner(a.spinner)+"  "+t.AccentText("加载歌单…"), t.FaintText("正在读取歌曲列表"))
		case a.playlistSongsErr != nil:
			lines2 := []string{t.BadText("✕ 加载失败")}
			for _, l := range wrapText(prettyText(a.playlistSongsErr.Error()), max(10, rightW-4)) {
				lines2 = append(lines2, t.FaintText(truncate(l, rightW)))
			}
			rows = verticalCenter(lines2, rowsH, rightW)
		case name == "未选择歌单":
			rows = blankState(rowsH, rightW, t.FaintText("请选择左侧歌单"))
		case len(a.playlistSongs) == 0:
			rows = blankState(rowsH, rightW, t.FaintText("♪  这个歌单还是空的"), t.FaintText("在搜索结果上右键，选择「添加到歌单」"))
		default:
			if a.playlistSongSelected >= len(a.playlistSongs) {
				a.playlistSongSelected = max(0, len(a.playlistSongs)-1)
			}
			rows = a.renderSongRows(rightX, bodyY+3, rightW, rowsH, a.playlistSongs, a.playlistSongSelected, &a.playlistScroll, a.currentSongID(), HitPlaylistSong, "playlist:"+a.playlistName)
			a.playlistSongsArea = Rect{X: rightX, Y: bodyY + 3, W: rightW, H: rowsH}
		}
		if len(rows) < rowsH {
			rows = append(rows, make([]string, rowsH-len(rows))...)
		}
		for i := 0; i < rowsH && 3+i < bodyH; i++ {
			rightLines[3+i] = rows[i]
		}
	}

	for i := 0; i < bodyH; i++ {
		row := padVisible(leftLines[i], leftW) + "  " + t.FaintText("│") + "  " + padVisible(rightLines[i], rightW)
		lines = append(lines, spaces(innerX)+padVisible(row, innerW))
	}
	if len(lines) > r.H {
		lines = lines[:r.H]
	}
	return lines
}

func (a *App) renderPlaylistListRow(p alx.Playlist, selected bool, width int) string {
	t := a.th
	marker := " "
	if selected {
		marker = "▌"
	}
	icon := "♪"
	if p.Name == "Favorites" {
		icon = "★"
	}
	count := fmt.Sprintf("%d", p.Count)
	leftW := max(2, width-displayWidth(count)-1)
	label := marker + icon + " " + prettyText(p.Name)
	if selected {
		label = t.AccentText(marker) + t.AccentText(icon) + " " + t.Bold(t.Accent, truncate(prettyText(p.Name), leftW-2))
	} else {
		label = t.FaintText(marker) + t.Dim(icon) + " " + t.Dim(truncate(prettyText(p.Name), leftW-2))
	}
	label = padVisible(label, leftW)
	return padVisible(label+" "+t.FaintText(count), width)
}

func (a *App) renderNarrowPlaylistBody(x, y, width, height int) []string {
	t := a.th
	if height <= 0 {
		return nil
	}
	lines := make([]string, 0, height)
	name := a.selectedPlaylistName()
	if name == "" {
		name = "未选择歌单"
	}
	prevLabel := t.FaintText("‹")
	nextLabel := t.FaintText("›")
	if a.playlistSelected > 0 {
		prevLabel = t.AccentText("‹")
	}
	if a.playlistSelected >= 0 && a.playlistSelected < len(a.playlists)-1 {
		nextLabel = t.AccentText("›")
	}
	selector := spaces(1) + prevLabel + "  " + t.Bold(t.Accent, truncate(prettyText(name), max(4, width-20))) + "  " + nextLabel
	if a.playlistSelected > 0 {
		a.addHit(HitPlaylistRow, Rect{X: x + 1, Y: y, W: 1, H: 1}, "", a.playlistSelected-1, alx.Song{})
	}
	if a.playlistSelected >= 0 && a.playlistSelected < len(a.playlists)-1 {
		selW := displayWidth(t.Bold(t.Accent, truncate(prettyText(name), max(4, width-20))))
		a.addHit(HitPlaylistRow, Rect{X: x + 3 + selW + 2, Y: y, W: 1, H: 1}, "", a.playlistSelected+1, alx.Song{})
	}
	newW := displayWidth("＋ 新建")
	if displayWidth(selector) < width-newW {
		selector = padVisible(selector, width-newW) + t.AccentText("＋ 新建")
		a.addHit(HitPlaylistNew, Rect{X: x + width - newW, Y: y, W: newW, H: 1}, "", 0, alx.Song{})
	}
	lines = append(lines, padVisible(selector, width))

	actions := []actionButton{
		{label: "▶ 播放", kind: HitPlaylistPlayAll, primary: true, disabled: name == "未选择歌单" || len(a.playlistSongs) == 0},
		{label: "导入链接", kind: HitPlaylistImport},
		{label: "重命名", kind: HitPlaylistRename, disabled: name == "未选择歌单"},
		{label: "删除", kind: HitPlaylistDelete, danger: true, disabled: name == "未选择歌单" || name == "Favorites"},
		{label: "⟳", kind: HitPlaylistRefresh},
	}
	lines = append(lines, a.renderButtons(x, y+len(lines), actions))
	lines = append(lines, t.Sep(width))
	cols := newSongColumns(width, true)
	lines = append(lines, padVisible(cols.header(t), width))
	rowsH := height - len(lines)
	if rowsH <= 0 {
		return lines
	}
	var rows []string
	switch {
	case a.playlistSongsLoading:
		rows = blankState(rowsH, width, a.th.Spinner(a.spinner)+"  "+t.AccentText("加载歌单…"))
	case a.playlistSongsErr != nil:
		rows = verticalCenter([]string{t.BadText("✕ 加载失败"), t.FaintText(truncate(prettyText(a.playlistSongsErr.Error()), width))}, rowsH, width)
	case name == "未选择歌单":
		rows = blankState(rowsH, width, t.FaintText("还没有选择歌单"))
	case len(a.playlistSongs) == 0:
		rows = blankState(rowsH, width, t.FaintText("♪  这个歌单还是空的"))
	default:
		a.playlistSongSelected = clamp(a.playlistSongSelected, 0, len(a.playlistSongs)-1)
		rows = a.renderSongRows(x, y+len(lines), width, rowsH, a.playlistSongs, a.playlistSongSelected, &a.playlistScroll, a.currentSongID(), HitPlaylistSong, "playlist:"+a.playlistName)
		a.playlistSongsArea = Rect{X: x, Y: y + len(lines), W: width, H: rowsH}
	}
	lines = append(lines, rows...)
	if len(lines) > height {
		lines = lines[:height]
	}
	return lines
}

func (a *App) selectPlaylist(index int) tea.Cmd {
	if index < 0 || index >= len(a.playlists) {
		return nil
	}
	a.playlistSelected = index
	name := a.playlists[index].Name
	if name == a.playlistName && a.playlistSongsLoading {
		return nil
	}
	a.playlistName = name
	a.playlistSeq++
	seq := a.playlistSeq
	a.playlistSongsLoading = true
	a.playlistSongsErr = nil
	a.playlistSongSelected = 0
	a.playlistScroll = 0
	return cmdLoadPlaylistSongs(a.client, name, seq)
}

func (a *App) playSelectedSong() tea.Cmd {
	var s *alx.Song
	if a.page == PageSearch {
		if a.searchSelected >= 0 && a.searchSelected < len(a.searchResults) {
			s = &a.searchResults[a.searchSelected]
		}
	} else if a.page == PagePlaylists {
		if a.playlistSongSelected >= 0 && a.playlistSongSelected < len(a.playlistSongs) {
			s = &a.playlistSongs[a.playlistSongSelected]
		}
	}
	if s == nil {
		return nil
	}
	a.blurSearch()
	return a.startPlay(*s)
}

func (a *App) keyboardPlaylistSelection(delta int, toRight bool) tea.Cmd {
	if a.playlistFocusRight {
		if len(a.playlistSongs) == 0 {
			a.playlistFocusRight = false
			return nil
		}
		a.playlistSongSelected = clamp(a.playlistSongSelected+delta, 0, len(a.playlistSongs)-1)
		ensureVisible(a.playlistSongSelected, &a.playlistScroll, max(1, a.playlistSongsArea.H), len(a.playlistSongs))
		return nil
	}
	if len(a.playlists) == 0 {
		return nil
	}
	a.playlistSelected = clamp(a.playlistSelected+delta, 0, len(a.playlists)-1)
	ensureVisible(a.playlistSelected, &a.playlistListScroll, max(1, a.playlistListArea.H), len(a.playlists))
	if toRight {
		a.playlistFocusRight = true
	}
	return a.selectPlaylist(a.playlistSelected)
}

func (a *App) requestPlaylistSongMenu(song alx.Song, playlist string, x, y int) {
	a.openSongMenu(song, "playlist:"+playlist, x, y)
}
