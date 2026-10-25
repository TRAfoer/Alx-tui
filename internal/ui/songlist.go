package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"alxtui/internal/alx"
)

type songColumns struct {
	prefix     int
	title      int
	artist     int
	album      int
	source     int
	duration   int
	showArtist bool
	showAlbum  bool
}

func newSongColumns(width int, allowAlbum bool) songColumns {
	c := songColumns{prefix: 4, source: 5, duration: 5, showArtist: true, showAlbum: allowAlbum}
	if width < 64 {
		c.showArtist = false
	}
	if width < 92 {
		c.showAlbum = false
	}
	for {
		fixed := c.prefix + 1 // prefix + gap
		if c.showArtist {
			fixed += 1 + 13
		}
		if c.showAlbum {
			fixed += 1 + 16
		}
		fixed += 1 + c.source + 1 + c.duration
		c.title = width - fixed
		if c.title >= 14 {
			break
		}
		if c.showAlbum {
			c.showAlbum = false
			continue
		}
		if c.showArtist {
			c.showArtist = false
			continue
		}
		c.title = max(6, width-fixed)
		break
	}
	if c.showArtist {
		c.artist = 13
	}
	if c.showAlbum {
		c.album = 16
	}
	return c
}

func (c songColumns) header(t Theme) string {
	var b strings.Builder
	b.WriteString(padVisible("  # ", c.prefix))
	b.WriteString(" ")
	b.WriteString(padVisible("歌曲", c.title))
	if c.showArtist {
		b.WriteString(" ")
		b.WriteString(padVisible("歌手", c.artist))
	}
	if c.showAlbum {
		b.WriteString(" ")
		b.WriteString(padVisible("专辑", c.album))
	}
	b.WriteString(" ")
	b.WriteString(padVisible("来源", c.source))
	b.WriteString(" ")
	b.WriteString(lipgloss.NewStyle().Width(c.duration).Align(lipgloss.Right).Render("时长"))
	return t.FaintText(b.String())
}

func (c songColumns) row(t Theme, s alx.Song, index int, selected, playing bool) string {
	marker := " "
	if playing {
		marker = "♪"
	}
	if selected {
		marker = "▸"
	}
	numStyle := func(v string) string { return t.FaintText(v) }
	if playing {
		numStyle = t.AccentText
	}
	if selected {
		numStyle = t.AccentText
	}
	prefix := numStyle(marker+" ") + numStyle(fmt.Sprintf("%2d", index+1))
	prefix = padVisible(prefix, c.prefix)

	titleStyle := lipgloss.NewStyle().Foreground(t.Text)
	if playing {
		titleStyle = titleStyle.Foreground(t.Accent2).Bold(true)
	}
	if selected {
		titleStyle = titleStyle.Foreground(t.Accent).Bold(true)
	}
	title := titleStyle.Render(truncate(prettyText(s.Name), c.title))

	var b strings.Builder
	b.WriteString(prefix)
	b.WriteString(" ")
	b.WriteString(padVisible(title, c.title))
	if c.showArtist {
		b.WriteString(" ")
		b.WriteString(padVisible(t.MutedText(truncate(prettyText(s.Singer), c.artist)), c.artist))
	}
	if c.showAlbum {
		b.WriteString(" ")
		b.WriteString(padVisible(t.FaintText(truncate(prettyText(s.AlbumName), c.album)), c.album))
	}
	b.WriteString(" ")
	src := lipgloss.NewStyle().Foreground(sourceAccent(t, s.Source)).Bold(true).Render(sourceCodeLabel(s.Source))
	b.WriteString(padVisible(src, c.source))
	b.WriteString(" ")
	b.WriteString(padVisible(t.FaintText(s.Interval), c.duration))
	return b.String()
}

// renderSongRows draws a window of songs and registers one hit per row.
func (a *App) renderSongRows(x, y, width, height int, songs []alx.Song, selected int, scroll *int, currentID string, kind HitKind, contextValue string) []string {
	if height <= 0 || width <= 0 {
		return nil
	}
	cols := newSongColumns(width, true)
	out := make([]string, height)
	clampScroll(scroll, len(songs), height)
	start, end := windowBounds(len(songs), *scroll, height)
	for i := start; i < end; i++ {
		s := songs[i]
		row := cols.row(a.th, s, i, i == selected, s.CLIID != "" && s.CLIID == currentID)
		out[i-start] = padVisible(row, width)
		a.addRowHit(kind, x, y+i-start, width, contextValue, i, s)
	}
	return out
}
