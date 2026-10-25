package ui

import (
	"fmt"
	"strings"

	"alxtui/internal/alx"
	tea "github.com/charmbracelet/bubbletea"
)

var searchPlatforms = []string{"all", "wy", "tx", "kg", "kw", "mg"}

func (a *App) renderSearchPage(r Rect) []string {
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

	// Heading.
	right := ""
	if a.searchLoading {
		right = t.AccentText(a.th.Spinner(a.spinner) + " 搜索中")
	} else if a.searchSubmitted != "" {
		right = t.FaintText(fmt.Sprintf("第 %d 页 · %d 首", max(1, a.searchPage), len(a.searchResults)))
	} else {
		right = t.FaintText("输入关键词开始搜索")
	}
	heading := t.Bold(t.Accent, "搜索") + "  " + t.FaintText("SEARCH")
	lines = append(lines, padVisible(composeLR(heading, right, innerW), innerW))

	// Input box (3 rows).
	borderColor := t.Border
	if a.searchFocus {
		borderColor = t.Accent
	}
	inputInner := innerW - 4
	promptW := displayWidth(a.searchInput.Prompt)
	a.searchInput.Width = max(8, inputInner-promptW-1)
	inputView := a.searchInput.View()
	if a.searchInput.Value() == "" {
		cursor := ""
		if a.searchFocus {
			cursor = a.fakeCursor()
		}
		inputView = t.AccentText(a.searchInput.Prompt) + cursor + t.FaintText("搜索歌曲、歌手或专辑…")
	}
	inputView = padVisible(inputView, inputInner)
	top := t.FrameTop("", innerW, borderColor)
	boxLine := t.FrameLine(inputView, innerW, borderColor)
	bottom := t.FrameBottom(innerW, borderColor)
	inputY := r.Y + len(lines) + 1
	lines = append(lines, top, boxLine, bottom)
	a.addHit(HitSearchInput, Rect{X: innerX + 2, Y: inputY, W: innerW - 4, H: 1}, "", 0, alx.Song{})

	// Source selector chips.
	chipLine := t.FaintText("音源") + " "
	if displayWidth(chipLine)+displayWidth("全部")+2 > innerW {
		chipLine = ""
	}
	x := innerX + displayWidth(chipLine)
	for i, code := range searchPlatforms {
		label := sourceLabel(code)
		if innerW < 84 {
			if code == "all" {
				label = "全部"
			} else {
				label = strings.ToUpper(code)
			}
		}
		active := a.searchSource == code
		chip := t.ActiveChip(label, active)
		chipW := displayWidth(chip)
		if x-innerX+chipW > innerW {
			break
		}
		chipLine += chip
		a.addHit(HitSearchSource, Rect{X: x, Y: r.Y + len(lines), W: chipW, H: 1}, code, i, alx.Song{})
		x += chipW
		if i < len(searchPlatforms)-1 {
			chipLine += " "
			x++
		}
	}
	lines = append(lines, padVisible(chipLine, innerW))
	lines = append(lines, t.Sep(innerW))

	// Table header.
	cols := newSongColumns(innerW, true)
	tableHeader := cols.header(t)
	lines = append(lines, padVisible(tableHeader, innerW))

	if len(lines) >= r.H {
		return lines[:r.H]
	}
	navH := 1
	bodyH := r.H - len(lines) - navH
	if bodyH < 0 {
		bodyH = 0
	}

	bodyY := r.Y + len(lines)
	a.searchResultsArea = Rect{X: innerX, Y: bodyY, W: innerW, H: bodyH}
	body := a.searchBody(innerX, bodyY, innerW, bodyH, cols)
	lines = append(lines, body...)

	// Pagination row.
	nav := a.renderSearchNav(innerX, r.Y+len(lines), innerW)
	lines = append(lines, nav)

	for i := range lines {
		lines[i] = spaces(innerX) + lines[i]
	}
	if len(lines) > r.H {
		lines = lines[:r.H]
	}
	return lines
}

func (a *App) searchBody(x, y, width, height int, cols songColumns) []string {
	t := a.th
	if height <= 0 {
		return nil
	}
	if a.searchLoading {
		return blankState(height, width,
			a.th.Spinner(a.spinner)+"  "+t.AccentText("正在搜索")+"  "+t.Dim(truncate(prettyText(a.searchInput.Value()), max(2, width-20))),
			t.FaintText("音乐源响应可能需要几秒，请稍候…"),
		)
	}
	if a.searchErr != nil {
		msg := t.BadText("✕ 搜索失败")
		errLines := wrapText(prettyText(a.searchErr.Error()), max(10, width-4))
		lines := []string{msg, ""}
		for _, l := range errLines {
			lines = append(lines, t.FaintText(truncate(l, width)))
		}
		lines = append(lines, "", t.FaintText("按 r 重试，或更换音源后再试"))
		return verticalCenter(lines, height, width)
	}
	if a.searchSubmitted == "" {
		return blankState(height, width,
			t.AccentText("⌕  ")+t.Bold(t.Text, "搜索你喜欢的音乐"),
			t.FaintText("输入歌曲、歌手或专辑名，按 Enter 开始搜索"),
		)
	}
	if len(a.searchResults) == 0 {
		return blankState(height, width,
			t.Bold(t.Text, "没有找到「"+truncate(a.searchSubmitted, max(4, width-12))+"」"),
			t.FaintText("试试更换关键词，或点击上方其他音源"),
		)
	}
	return a.renderSongRows(x, y, width, height, a.searchResults, a.searchSelected, &a.searchScroll, a.currentSongID(), HitSearchResult, "search")
}

func (a *App) renderSearchNav(x, y, width int) string {
	t := a.th
	prev := t.FaintText("‹ 上一页")
	next := t.FaintText("下一页 ›")
	prevHit := Rect{}
	nextHit := Rect{}
	if a.searchPage > 1 && !a.searchLoading {
		prev = t.AccentText("‹ 上一页")
		prevHit = Rect{X: x, Y: y, W: displayWidth(prev), H: 1}
	}
	if len(a.searchResults) >= a.searchLimit && !a.searchLoading {
		next = t.AccentText("下一页 ›")
		nextHit = Rect{X: x + width - displayWidth(next), Y: y, W: displayWidth(next), H: 1}
	}
	pageText := t.FaintText(fmt.Sprintf("第 %d 页", max(1, a.searchPage)))
	line := padVisible(prev, width)
	pageX := x + max(0, (width-displayWidth(pageText))/2)
	line = padVisible(prev, pageX-x) + pageText
	if displayWidth(line) < width-displayWidth(next) {
		line = padVisible(line, width-displayWidth(next)) + next
	} else {
		line = padVisible(line, width)
	}
	if !prevHit.Empty() {
		a.addHit(HitSearchPrev, prevHit, "", 0, alx.Song{})
	}
	if !nextHit.Empty() {
		a.addHit(HitSearchNext, nextHit, "", 0, alx.Song{})
	}
	return padVisible(line, width)
}

// blankState vertically centers a title/subtitle in a results area.
func blankState(height, width int, lines ...string) []string {
	if height <= 0 {
		return nil
	}
	out := make([]string, height)
	start := max(0, (height-len(lines))/2)
	for i := 0; i < height; i++ {
		if i >= start && i-start < len(lines) {
			out[i] = padVisible(centerText(lines[i-start], width), width)
		} else {
			out[i] = spaces(width)
		}
	}
	return out
}

func verticalCenter(lines []string, height, width int) []string {
	if height <= 0 {
		return nil
	}
	out := make([]string, height)
	start := max(0, (height-len(lines))/2)
	for i := 0; i < height; i++ {
		if i >= start && i-start < len(lines) {
			out[i] = padVisible(lines[i-start], width)
		} else {
			out[i] = spaces(width)
		}
	}
	return out
}

func (a *App) focusSearch() tea.Cmd {
	if a.page != PageSearch {
		return nil
	}
	a.searchFocus = true
	return a.searchInput.Focus()
}

func (a *App) blurSearch() {
	a.searchFocus = false
	a.searchInput.Blur()
}

func (a *App) startSearch(page int) tea.Cmd {
	keyword := strings.TrimSpace(a.searchInput.Value())
	if keyword == "" {
		a.showToast("请输入搜索关键词", toastInfo)
		a.searchFocus = true
		return a.searchInput.Focus()
	}
	if page < 1 {
		page = 1
	}
	a.searchSeq++
	seq := a.searchSeq
	a.searchLoading = true
	a.searchErr = nil
	a.searchPage = page
	a.searchSubmitted = keyword
	a.searchSubmittedSource = a.searchSource
	a.searchSelected = 0
	a.searchScroll = 0
	a.searchInput.CursorEnd()
	return cmdSearch(a.client, seq, keyword, a.searchSource, page, a.searchLimit)
}
