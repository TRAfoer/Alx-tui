package ui

import "alxtui/internal/alx"

// Rect is a terminal-cell rectangle in absolute screen coordinates.
type Rect struct {
	X, Y, W, H int
}

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

func (r Rect) Empty() bool { return r.W <= 0 || r.H <= 0 }

// HitKind identifies an interactive region.
type HitKind int

const (
	HitNone HitKind = iota
	HitTab

	HitMiniInfo
	HitMiniProgress
	HitMiniPrev
	HitMiniPlayPause
	HitMiniNext

	HitNowPlayingInfo
	HitPlaybackProgress
	HitPlaybackPrev
	HitPlaybackPlayPause
	HitPlaybackNext
	HitPlaybackStop
	HitPlaybackVolumeMinus
	HitPlaybackVolumePlus
	HitPlaybackRepeat
	HitPlaybackShuffle
	HitLyricsRetry
	HitQueueSong

	HitSearchInput
	HitSearchSource
	HitSearchResult
	HitSearchPrev
	HitSearchNext

	HitPlaylistRow
	HitPlaylistSong
	HitPlaylistNew
	HitPlaylistImport
	HitPlaylistPlayAll
	HitPlaylistDelete
	HitPlaylistRename
	HitPlaylistRefresh

	HitSettingsSourceChip
	HitSettingsQualityChip
	HitSettingsAddSource
	HitSettingsRemoveSource
	HitSettingsSourceRow
	HitSettingsRefresh
	HitSettingsAccentColor
	HitSettingsAccentCustom
	HitSettingsTextColor
	HitSettingsTextCustom
	HitSettingsResetColors
	HitSettingsDownloadDir
	HitSettingsDirPreset
	HitSettingsRepeat
	HitSettingsShuffle
	HitSettingsVolumeMinus
	HitSettingsVolumePlus
)

// Hit is one clickable region registered while the view is rendered.
type Hit struct {
	Kind  HitKind
	Rect  Rect
	Index int
	Value string
	Song  alx.Song
}

func (a *App) addHit(kind HitKind, r Rect, value string, index int, song alx.Song) {
	if r.Empty() || kind == HitNone {
		return
	}
	r.H = max(1, r.H)
	r.W = max(1, r.W)
	a.hits = append(a.hits, Hit{Kind: kind, Rect: r, Value: value, Index: index, Song: song})
}

func (a *App) addRowHit(kind HitKind, x, y, w int, value string, index int, song alx.Song) {
	a.addHit(kind, Rect{X: x, Y: y, W: w, H: 1}, value, index, song)
}

func (a *App) hitAt(x, y int) (Hit, bool) {
	for i := len(a.hits) - 1; i >= 0; i-- {
		if a.hits[i].Rect.Contains(x, y) {
			return a.hits[i], true
		}
	}
	return Hit{}, false
}
