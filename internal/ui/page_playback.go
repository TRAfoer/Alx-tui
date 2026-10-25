package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"alxtui/internal/alx"
)

func (a *App) renderPlaybackPage(r Rect) []string {
	t := a.th
	if r.H <= 0 || r.W <= 0 {
		return nil
	}
	const pad = 2
	innerX := r.X + pad
	innerW := r.W - pad*2
	if innerW < 20 {
		return []string{padVisible(t.Dim("窗口太窄"), r.W)}
	}

	lines := make([]string, 0, r.H)
	cardH := 7
	if r.H < cardH+2 {
		cardH = r.H
	}
	cardLines := a.renderNowPlayingCard(innerX, r.Y, innerW, cardH)
	lines = append(lines, cardLines...)
	if r.H < cardH+2 {
		return lines
	}
	lines = append(lines, "")

	lowerH := r.H - len(lines) - 1 // heading consumes one row
	if lowerH <= 0 {
		return lines
	}
	// heading
	leftW := 0
	rightW := innerW
	columns := innerW >= 86 && lowerH >= 3
	if columns {
		leftW = clamp(innerW*30/100, 26, 42)
		rightW = innerW - leftW - 5
		if rightW < 28 {
			columns = false
			leftW = 0
			rightW = innerW
		}
	}

	lyricsHeading := t.Bold(t.Accent, "歌词")
	if a.lyricsSource != "" && a.lyricsSource != "alx" {
		lyricsHeading += "  " + t.FaintText("· "+a.lyricsSource)
	}
	rightHeading := lyricsHeading
	if a.lyricsManualUntil.After(a.clock) {
		rightHeading += "  " + t.FaintText("手动滚动")
	}
	retry := t.FaintText("↻ 重试")
	if a.lyricsLoading {
		retry = a.th.Spinner(a.spinner) + t.FaintText(" 获取中")
	}
	retryW := displayWidth(retry)
	rightHeading = composeLR(rightHeading, retry, rightW)
	if columns {
		leftHeading := t.Bold(t.Accent, "播放队列")
		if len(a.queue) > 0 {
			leftHeading += " " + t.FaintText(fmt.Sprintf("%d 首", len(a.queue)))
		}
		heading := padVisible(leftHeading, leftW) + "  " + t.FaintText("│") + "  " + padVisible(rightHeading, rightW)
		lines = append(lines, padVisible(heading, innerW))
		retryX := innerX + leftW + 5 + rightW - retryW
		if !a.lyricsLoading {
			a.addHit(HitLyricsRetry, Rect{X: retryX, Y: r.Y + len(lines) - 1, W: retryW, H: 1}, "", 0, alx.Song{})
		}
	} else {
		heading := lyricsHeading
		heading = composeLR(heading, retry, innerW)
		lines = append(lines, padVisible(heading, innerW))
		retryX := innerX + innerW - retryW
		if !a.lyricsLoading {
			a.addHit(HitLyricsRetry, Rect{X: retryX, Y: r.Y + len(lines) - 1, W: retryW, H: 1}, "", 0, alx.Song{})
		}
	}

	bodyH := r.H - len(lines)
	if bodyH <= 0 {
		return lines
	}
	queueLines := make([]string, bodyH)
	lyricsLines := make([]string, bodyH)
	if columns {
		queueLines = a.renderQueueBody(leftW, bodyH)
		lyricsLines = a.renderLyricsBody(rightW, bodyH)
		a.queueArea = Rect{X: innerX, Y: r.Y + len(lines), W: leftW, H: bodyH}
		a.lyricsArea = Rect{X: innerX + leftW + 5, Y: r.Y + len(lines), W: rightW, H: bodyH}
	} else {
		lyricsLines = a.renderLyricsBody(innerW, bodyH)
		a.lyricsArea = Rect{X: innerX, Y: r.Y + len(lines), W: innerW, H: bodyH}
	}

	for i := 0; i < bodyH; i++ {
		if columns {
			row := padVisible(queueLines[i], leftW) + "  " + t.FaintText("│") + "  " + padVisible(lyricsLines[i], rightW)
			lines = append(lines, padVisible(row, innerW))
		} else {
			lines = append(lines, padVisible(lyricsLines[i], innerW))
		}
	}
	// Prefix page padding.
	for i := range lines {
		lines[i] = spaces(innerX) + lines[i]
	}
	if len(lines) > r.H {
		lines = lines[:r.H]
	}
	return lines
}

func (a *App) renderNowPlayingCard(x, y, width, height int) []string {
	t := a.th
	if width < 10 || height < 3 {
		return []string{" " + t.Dim("窗口太小")}
	}
	contentW := width - 4
	lines := make([]string, 0, height)
	top := t.FrameTop("正在播放", width, t.Border)
	lines = append(lines, top)

	song := a.currentSong()
	pending := a.playPending
	if pending {
		song = &a.pendingSong
	}
	loadingPlaylist := a.playlistPending && song == nil
	status := ""
	if pending {
		status = t.AccentText(a.th.Spinner(a.spinner) + " 加载中")
	} else if loadingPlaylist {
		status = t.AccentText(a.th.Spinner(a.spinner) + " 加载歌单")
	} else if song == nil {
		status = t.FaintText("未播放")
	} else {
		switch a.state.Status {
		case "playing":
			status = t.GoodText("● 播放中")
		case "paused":
			status = t.WarnText("● 已暂停")
		default:
			status = t.FaintText("● 已停止")
		}
	}

	var titleLine, artistLine string
	if loadingPlaylist {
		titleLine = a.th.Spinner(a.spinner) + "  " + t.Bold(t.Text, "正在加载歌单…")
		artistLine = t.FaintText("正在解析第一首歌曲，请稍候")
	} else if song == nil {
		titleLine = t.AccentText("♫") + "  " + t.Bold(t.Text, "还没有播放歌曲")
		artistLine = t.FaintText("去「搜索」页找一首歌，左键点击即可播放")
	} else {
		title := prettyText(song.Name)
		titleLine = t.AccentText("♫") + "  " + t.Bold(t.Text, truncate(title, max(6, contentW-16)))
		if pending {
			titleLine = a.th.Spinner(a.spinner) + "  " + t.Bold(t.Text, truncate(title, max(6, contentW-16)))
		}
		album := prettyText(song.AlbumName)
		if album != "" {
			album = "  ·  《" + album + "》"
		}
		artistLine = t.Dim(truncate(prettyText(song.Singer), max(0, contentW/2))) + t.FaintText(truncate(album, max(0, contentW/2)))
		if song.Singer == "" {
			artistLine = t.FaintText(truncate(album, contentW))
		}
	}
	titleLine = composeLR(titleLine, status, contentW)
	lines = append(lines, t.FrameLine(titleLine, width, t.Border))
	if song != nil {
		a.addHit(HitNowPlayingInfo, Rect{X: x + 2, Y: y + 1, W: width - 4, H: 3}, "", 0, *song)
	}

	sourcePart := ""
	if song != nil {
		sourcePart = lipgloss.NewStyle().Foreground(sourceAccent(t, song.Source)).Bold(true).Render(sourceCodeLabel(song.Source))
		quality := a.quality
		if quality == "" {
			quality = "320k"
		}
		sourcePart += t.FaintText("  " + strings.ToUpper(quality))
	}
	artistLine = composeLR(artistLine, sourcePart, contentW)
	lines = append(lines, t.FrameLine(artistLine, width, t.Border))
	lines = append(lines, t.FrameLine("", width, t.Border))

	// Progress row.
	progressY := y + len(lines)
	contentX := x + 2
	pos := a.currentPosition()
	dur := a.trackDuration()
	timeW := 5
	barW := clamp(contentW-timeW*2-6, 8, 54)
	bar := a.renderBar(a.progressRatio(), barW)
	timeLabel := t.FaintText(alx.FormatSeconds(pos))
	totalLabel := t.FaintText(alx.FormatSeconds(dur))
	if dur <= 0 {
		totalLabel = t.FaintText("--:--")
	}
	progress := timeLabel + "  " + bar + "  " + totalLabel
	lines = append(lines, t.FrameLine(progress, width, t.Border))
	if song != nil {
		barX := contentX + timeW + 2
		a.addHit(HitPlaybackProgress, Rect{X: barX, Y: progressY, W: barW, H: 1}, "", 0, *song)
	}

	// Controls row.
	controlsY := y + len(lines)
	controls := a.renderPlaybackControls(song, pending, contentX, controlsY, contentW)
	lines = append(lines, t.FrameLine(controls, width, t.Border))

	for len(lines) < height-1 {
		lines = append(lines, t.FrameLine("", width, t.Border))
	}
	if len(lines) >= height {
		lines = lines[:max(0, height-1)]
	}
	lines = append(lines, t.FrameBottom(width, t.Border))
	return lines
}

func (a *App) renderPlaybackControls(song *alx.Song, pending bool, x, y, width int) string {
	t := a.th
	enabled := song != nil
	playLabel := a.playIcon()
	if pending {
		playLabel = "·"
	}
	if !enabled {
		playLabel = "▶"
	}
	buttons := []struct {
		label   string
		kind    HitKind
		primary bool
	}{
		{"⏮", HitPlaybackPrev, false},
		{playLabel, HitPlaybackPlayPause, true},
		{"⏭", HitPlaybackNext, false},
		{"⏹", HitPlaybackStop, false},
	}
	var b strings.Builder
	cursor := x
	for i, btn := range buttons {
		style := lipgloss.NewStyle().Foreground(t.Muted).Background(t.PanelBg).Padding(0, 2)
		if btn.primary {
			style = lipgloss.NewStyle().Foreground(t.OnAccent).Background(t.AccentStrong).Bold(true).Padding(0, 2)
		}
		if !enabled {
			style = lipgloss.NewStyle().Foreground(t.Faint).Padding(0, 2)
		}
		rendered := style.Render(btn.label)
		b.WriteString(rendered)
		if enabled {
			a.addHit(btn.kind, Rect{X: cursor, Y: y, W: displayWidth(rendered), H: 1}, "", 0, *song)
		}
		cursor += displayWidth(rendered)
		if i < len(buttons)-1 {
			b.WriteString("  ")
			cursor += 2
		}
	}

	vol := a.state.Volume
	if vol <= 0 {
		vol = a.cfg.Player.DefaultVolume
	}
	if width >= 58 {
		b.WriteString(spaces(2))
		cursor += 2
		volText := t.FaintText(fmt.Sprintf("VOL %d%%", vol))
		b.WriteString(volText)
		cursor += displayWidth(volText)
		b.WriteString("  ")
		cursor += 2
		minus := lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 1).Render("−")
		plus := lipgloss.NewStyle().Foreground(t.Muted).Padding(0, 1).Render("+")
		b.WriteString(minus)
		a.addHit(HitPlaybackVolumeMinus, Rect{X: cursor, Y: y, W: displayWidth(minus), H: 1}, "", 0, alx.Song{})
		cursor += displayWidth(minus)
		b.WriteString(plus)
		a.addHit(HitPlaybackVolumePlus, Rect{X: cursor, Y: y, W: displayWidth(plus), H: 1}, "", 0, alx.Song{})
		cursor += displayWidth(plus)
	}
	return truncate(b.String(), width)
}

func (a *App) renderQueueBody(width, height int) []string {
	t := a.th
	if height <= 0 {
		return nil
	}
	out := make([]string, height)
	if !a.queueLoaded {
		out[0] = t.FaintText("加载队列中…")
		return out
	}
	if len(a.queue) == 0 {
		out[0] = t.FaintText("队列为空")
		return out
	}
	if a.clock.After(a.queueManualUntil) && a.queueIndex >= 0 {
		a.queueScroll = a.queueIndex - height/2
	}
	clampScroll(&a.queueScroll, len(a.queue), height)
	start, end := windowBounds(len(a.queue), a.queueScroll, height)
	for i := start; i < end; i++ {
		s := a.queue[i]
		marked := i == a.queueIndex
		prefix := "  "
		style := t.Dim
		if marked {
			prefix = t.AccentText("▸ ")
			style = func(s string) string { return t.AccentText(s) }
		}
		line := prefix + style(fmt.Sprintf("%d. ", i+1)) + style(truncate(prettyText(s.Name), max(2, width-16)))
		if s.Singer != "" && width > 32 {
			line += t.FaintText(" · " + truncate(prettyText(s.Singer), 10))
		}
		out[i-start] = padVisible(line, width)
	}
	return out
}

func (a *App) renderLyricsBody(width, height int) []string {
	t := a.th
	if height <= 0 {
		return nil
	}
	out := make([]string, height)
	if a.lyricsLoading {
		out[0] = t.FaintText(a.th.Spinner(a.spinner) + " 正在加载歌词…")
		return out
	}
	if len(a.lyrics.Lines) == 0 {
		if a.playlistPending && a.currentSong() == nil {
			out[0] = t.FaintText(a.th.Spinner(a.spinner) + " 正在加载歌单…")
			return out
		}
		if a.currentSong() == nil {
			out[0] = t.FaintText("♪  暂无播放内容")
			if height > 1 {
				out[1] = t.FaintText(truncate("播放歌曲后，有歌词就会自动同步显示", width))
			}
			return out
		}
		out[0] = t.FaintText("♪  暂无歌词")
		if height > 1 {
			hint := "有歌词时会自动同步显示"
			if a.lyricsErr != "" {
				lower := strings.ToLower(a.lyricsErr)
				if strings.Contains(lower, "all sources failed") || strings.Contains(lower, "not found") || strings.Contains(lower, "no lyric") {
					hint = "该歌曲没有可用的 LRC 歌词"
				} else {
					hint = "歌词获取失败，可点击「重试」"
				}
			}
			out[1] = t.FaintText(truncate(hint, width))
		}
		return out
	}

	lines := a.lyrics.Lines
	active := -1
	if a.lyrics.Synced {
		active = a.lyrics.IndexAt(time.Duration(a.currentPosition() * float64(time.Second)))
		if active < 0 {
			active = 0
		}
	}
	offset := a.lyricsScroll
	if a.lyrics.Synced && !a.clock.Before(a.lyricsManualUntil) {
		offset = active - height/2
		a.lyricsScroll = offset
	}
	clampScroll(&offset, len(lines), height)
	a.lyricsScroll = offset
	start, end := windowBounds(len(lines), offset, height)
	for i := start; i < end; i++ {
		text := prettyText(lines[i].Text)
		if text == "" {
			text = "♪"
		}
		if i == active {
			out[i-start] = padVisible(t.AccentText("▌ ")+t.Bold(t.Accent, truncate(text, max(2, width-2))), width)
		} else {
			out[i-start] = padVisible("  "+t.FaintText(truncate(text, max(2, width-2))), width)
		}
	}
	return out
}
