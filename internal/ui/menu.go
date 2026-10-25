package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"alxtui/internal/alx"
)

type menuAction int

const (
	menuPlay menuAction = iota
	menuAddToPlaylist
	menuDownload
	menuQueue
	menuRemoveFromPlaylist
	menuNewPlaylist
	menuPlaylistTarget
	menuClose
)

type menuItem struct {
	icon     string
	label    string
	action   menuAction
	playlist string
	disabled bool
}

type menuHit struct {
	rect Rect
	item int
	sub  bool
}

type menuState struct {
	song      alx.Song
	context   string
	anchorX   int
	anchorY   int
	main      []menuItem
	sub       []menuItem
	mainIdx   int
	subIdx    int
	subScroll int
	subOpen   bool
	x, y, w   int
	subX      int
	subY      int
	subW      int
	mainHits  []menuHit
	subHits   []menuHit
}

func (a *App) openSongMenu(song alx.Song, context string, x, y int) {
	if a.dlg != nil {
		return
	}
	a.blurSearch()
	main := []menuItem{
		{icon: "▶", label: "播放", action: menuPlay},
		{icon: "＋", label: "添加到歌单  ›", action: menuAddToPlaylist},
		{icon: "↓", label: "下载", action: menuDownload},
		{icon: "≡", label: "加入播放队列", action: menuQueue},
	}
	if playlistSongContext(context) != "" {
		main = append(main, menuItem{icon: "−", label: "从歌单移除", action: menuRemoveFromPlaylist})
	}
	main = append(main, menuItem{icon: "✕", label: "关闭", action: menuClose})

	current := playlistSongContext(context)
	sub := make([]menuItem, 0, len(a.playlists)+1)
	for _, p := range a.playlists {
		label := fmt.Sprintf("%s  %d", prettyText(p.Name), p.Count)
		disabled := current != "" && p.Name == current
		if disabled {
			label += "  (当前)"
		}
		sub = append(sub, menuItem{
			icon:     "♪",
			label:    label,
			action:   menuPlaylistTarget,
			playlist: p.Name,
			disabled: disabled,
		})
	}
	sub = append(sub, menuItem{icon: "＋", label: "新建歌单…", action: menuNewPlaylist})

	a.menu = &menuState{
		song:    song,
		context: context,
		anchorX: x,
		anchorY: y,
		main:    main,
		sub:     sub,
	}
}

func (m *menuState) currentSubItem() *menuItem {
	if m.subIdx >= 0 && m.subIdx < len(m.sub) {
		return &m.sub[m.subIdx]
	}
	return nil
}

func (m *menuState) currentMainItem() *menuItem {
	if m.mainIdx >= 0 && m.mainIdx < len(m.main) {
		return &m.main[m.mainIdx]
	}
	return nil
}

func (a *App) closeMenu() {
	a.menu = nil
}

func nextEnabled(items []menuItem, from, delta int) int {
	if len(items) == 0 {
		return -1
	}
	for step := 0; step < len(items); step++ {
		from = (from + delta + len(items)) % len(items)
		if !items[from].disabled {
			return from
		}
	}
	return -1
}

func (a *App) handleMenuKey(msg tea.KeyMsg) tea.Cmd {
	m := a.menu
	if m == nil {
		return nil
	}
	key := msg.String()
	if m.subOpen {
		switch key {
		case "esc", "left", "h":
			m.subOpen = false
			return nil
		case "up", "k":
			if idx := nextEnabled(m.sub, m.subIdx, -1); idx >= 0 {
				m.subIdx = idx
			}
			return nil
		case "down", "j":
			if idx := nextEnabled(m.sub, m.subIdx, 1); idx >= 0 {
				m.subIdx = idx
			}
			return nil
		case "enter":
			item := m.currentSubItem()
			if item == nil || item.disabled {
				return nil
			}
			return a.activateMenuItem(*item, true)
		}
		return nil
	}
	switch key {
	case "esc", "q":
		a.closeMenu()
	case "up", "k":
		if idx := nextEnabled(m.main, m.mainIdx, -1); idx >= 0 {
			m.mainIdx = idx
		}
	case "down", "j":
		if idx := nextEnabled(m.main, m.mainIdx, 1); idx >= 0 {
			m.mainIdx = idx
		}
	case "right", "l":
		if item := m.currentMainItem(); item != nil && item.action == menuAddToPlaylist {
			m.subOpen = true
			m.subIdx = firstEnabled(m.sub)
		}
	case "enter":
		item := m.currentMainItem()
		if item == nil || item.disabled {
			return nil
		}
		if item.action == menuAddToPlaylist {
			m.subOpen = true
			m.subIdx = firstEnabled(m.sub)
			return nil
		}
		return a.activateMenuItem(*item, false)
	}
	return nil
}

func firstEnabled(items []menuItem) int {
	for i := range items {
		if !items[i].disabled {
			return i
		}
	}
	return -1
}

func (a *App) handleMenuMouse(msg tea.MouseMsg) tea.Cmd {
	m := a.menu
	if m == nil {
		return nil
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		delta := 3
		if msg.Button == tea.MouseButtonWheelUp {
			delta = -3
		}
		if m.subOpen {
			m.subIdx = clamp(m.subIdx+delta, 0, max(0, len(m.sub)-1))
			if m.sub[m.subIdx].disabled {
				if idx := nextEnabled(m.sub, m.subIdx, delta); idx >= 0 {
					m.subIdx = idx
				}
			}
		} else if idx := nextEnabled(m.main, m.mainIdx, delta); idx >= 0 {
			m.mainIdx = idx
		}
		return nil
	}
	switch msg.Action {
	case tea.MouseActionMotion:
		for _, h := range m.subHits {
			if h.rect.Contains(msg.X, msg.Y) {
				if !m.sub[h.item].disabled {
					m.subIdx = h.item
				}
				return nil
			}
		}
		for _, h := range m.mainHits {
			if h.rect.Contains(msg.X, msg.Y) {
				if !m.main[h.item].disabled {
					m.mainIdx = h.item
				}
				return nil
			}
		}
		return nil
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			return nil
		}
		if msg.Button == tea.MouseButtonRight {
			a.closeMenu()
			return nil
		}
		if msg.Button != tea.MouseButtonLeft {
			return nil
		}
		for _, h := range m.subHits {
			if h.rect.Contains(msg.X, msg.Y) {
				m.subIdx = h.item
				item := m.currentSubItem()
				if item == nil || item.disabled {
					return nil
				}
				return a.activateMenuItem(*item, true)
			}
		}
		for _, h := range m.mainHits {
			if h.rect.Contains(msg.X, msg.Y) {
				m.mainIdx = h.item
				item := m.currentMainItem()
				if item == nil || item.disabled {
					return nil
				}
				if item.action == menuAddToPlaylist {
					m.subOpen = true
					m.subIdx = firstEnabled(m.sub)
					return nil
				}
				return a.activateMenuItem(*item, false)
			}
		}
		a.closeMenu()
		return nil
	}
	return nil
}

func (a *App) activateMenuItem(item menuItem, fromSub bool) tea.Cmd {
	song := a.menu.song
	context := a.menu.context
	switch item.action {
	case menuPlay:
		a.closeMenu()
		return a.startPlay(song)
	case menuAddToPlaylist:
		if m := a.menu; m != nil {
			m.subOpen = true
			m.subIdx = firstEnabled(m.sub)
		}
		return nil
	case menuDownload:
		a.closeMenu()
		return a.downloadSongs([]alx.Song{song})
	case menuQueue:
		a.closeMenu()
		return a.addSongsToQueue([]alx.Song{song})
	case menuRemoveFromPlaylist:
		a.closeMenu()
		playlist := playlistSongContext(context)
		if playlist == "" {
			a.showToast("当前上下文不属于歌单", toastError)
			return nil
		}
		return a.removeSongFromPlaylist(playlist, song)
	case menuNewPlaylist:
		a.closeMenu()
		return a.openNewPlaylistDialog(&song)
	case menuPlaylistTarget:
		a.closeMenu()
		return a.addSongToPlaylist(item.playlist, song)
	case menuClose:
		a.closeMenu()
	}
	return nil
}

func (a *App) renderMenuOverlay(base []string) []string {
	m := a.menu
	if m == nil {
		return base
	}
	t := a.th
	screenW, screenH := a.width, a.height
	mainW := m.mainWidth(screenW)
	mainLines, mainHits := a.buildMenuPanel(prettyText(m.song.Name), m.main, m.mainIdx, mainW)
	mainH := len(mainLines)
	x := clamp(m.anchorX, 0, max(0, screenW-mainW))
	y := clamp(m.anchorY, 0, max(0, screenH-mainH))
	if m.anchorX+mainW > screenW-1 {
		x = clamp(m.anchorX-mainW, 0, max(0, screenW-mainW))
	}
	if m.anchorY+mainH > screenH-1 {
		y = clamp(m.anchorY-mainH, 0, max(0, screenH-mainH))
	}
	m.x, m.y, m.w = x, y, mainW
	m.mainHits = translateHits(mainHits, x, y)
	out := overlayLines(base, mainLines, x, y, screenW)

	if m.subOpen {
		subW := m.subWidth(screenW)
		m.subIdx = clamp(m.subIdx, 0, max(0, len(m.sub)-1))
		maxItems := max(1, screenH-4)
		if m.subIdx < m.subScroll {
			m.subScroll = m.subIdx
		}
		if m.subIdx >= m.subScroll+maxItems {
			m.subScroll = m.subIdx - maxItems + 1
		}
		m.subScroll = clamp(m.subScroll, 0, max(0, len(m.sub)-maxItems))
		end := min(len(m.sub), m.subScroll+maxItems)
		visible := m.sub[m.subScroll:end]
		selectedRel := m.subIdx - m.subScroll
		title := "添加到歌单"
		if m.subScroll > 0 {
			title = "↑ " + title
		}
		if end < len(m.sub) {
			title += " ↓"
		}
		subLines, subLocalHits := a.buildMenuPanel(title, visible, selectedRel, subW)
		for i := range subLocalHits {
			subLocalHits[i].item += m.subScroll
		}
		subH := len(subLines)
		subX := x + mainW + 1
		if subX+subW > screenW {
			subX = x - subW - 1
		}
		subX = clamp(subX, 0, max(0, screenW-subW))
		subY := y + 1 + m.mainIdx
		if subY+subH > screenH {
			subY = screenH - subH
		}
		subY = clamp(subY, 0, max(0, screenH-subH))
		m.subX, m.subY, m.subW = subX, subY, subW
		m.subHits = translateHits(subLocalHits, subX, subY)
		out = overlayLines(out, subLines, subX, subY, screenW)
	} else {
		m.subHits = nil
	}
	_ = t
	return out
}

func (m *menuState) mainWidth(screenW int) int {
	w := displayWidth(m.song.Name) + 6
	for _, item := range m.main {
		cw := displayWidth(item.icon) + 2 + displayWidth(item.label) + 4
		if cw > w {
			w = cw
		}
	}
	return clamp(w, 24, min(46, screenW-2))
}

func (m *menuState) subWidth(screenW int) int {
	w := displayWidth("添加到歌单") + 6
	for _, item := range m.sub {
		cw := displayWidth(item.icon) + 2 + displayWidth(item.label) + 4
		if cw > w {
			w = cw
		}
	}
	return clamp(w, 26, min(48, screenW-2))
}

func (a *App) buildMenuPanel(title string, items []menuItem, selected, width int) ([]string, []menuHit) {
	t := a.th
	if width < 8 {
		return nil, nil
	}
	contentW := width - 4
	panelStyle := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.PanelFg)
	borderStyle := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Border)
	titleStyle := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Accent).Bold(true)
	selectedStyle := lipgloss.NewStyle().Background(t.SelBg).Foreground(t.SelFg).Bold(true)
	disabledStyle := lipgloss.NewStyle().Background(t.PanelBg).Foreground(t.Faint)

	lines := make([]string, 0, len(items)+2)
	label := "─ " + truncate(title, max(1, width-8)) + " "
	fill := width - 2 - displayWidth(label)
	if fill < 0 {
		fill = 0
	}
	lines = append(lines, titleStyle.Render("╭"+label+strings.Repeat("─", fill)+"╮"))

	hits := make([]menuHit, 0, len(items))
	for i, item := range items {
		marker := " "
		if i == selected {
			marker = ">"
		}
		text := marker + item.icon + " " + item.label
		text = truncate(text, contentW)
		style := panelStyle
		switch {
		case item.disabled:
			style = disabledStyle
		case i == selected:
			style = selectedStyle
		}
		// Style the border separately from the selected row so the menu
		// outline stays intact even when the row is highlighted.
		line := borderStyle.Render("│ ") + style.Render(padVisible(text, contentW)) + borderStyle.Render(" │")
		lines = append(lines, line)
		if !item.disabled {
			hits = append(hits, menuHit{rect: Rect{X: 0, Y: len(lines) - 1, W: width, H: 1}, item: i})
		}
	}
	lines = append(lines, borderStyle.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return lines, hits
}

func translateHits(hits []menuHit, dx, dy int) []menuHit {
	out := make([]menuHit, len(hits))
	for i, h := range hits {
		h.rect.X += dx
		h.rect.Y += dy
		out[i] = h
	}
	return out
}
