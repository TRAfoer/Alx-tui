package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"alxtui/internal/alx"
)

func (a *App) switchPage(p Page) tea.Cmd {
	if p < PagePlayback || p > PageSettings {
		return nil
	}
	if p == a.page {
		if p == PageSearch && !a.searchFocus {
			return a.focusSearch()
		}
		return nil
	}
	a.page = p
	var cmds []tea.Cmd
	if p == PageSearch {
		cmds = append(cmds, a.focusSearch())
	} else {
		a.blurSearch()
	}
	switch p {
	case PagePlaylists:
		if !a.playlistsLoading {
			a.playlistsLoading = true
			cmds = append(cmds, cmdLoadPlaylists(a.client))
		}
	case PageSettings:
		cmds = append(cmds, a.refreshSettingsData())
	}
	return tea.Batch(cmds...)
}

func (a *App) refreshState() tea.Cmd {
	if a.stateFetching {
		return nil
	}
	a.stateFetching = true
	a.stateSeq++
	return cmdLoadState(a.client, a.stateSeq)
}

func (a *App) invalidateState() {
	a.stateFetching = false
	a.stateSeq++
}

func (a *App) refreshQueue() tea.Cmd {
	if a.queueLoading {
		return nil
	}
	a.queueLoading = true
	a.queueSeq++
	return cmdLoadQueue(a.client, a.queueSeq)
}

func (a *App) invalidateQueue() {
	a.queueLoading = false
	a.queueSeq++
}

func (a *App) loadLyrics(song *alx.Song, force bool) tea.Cmd {
	if song == nil || song.CLIID == "" {
		a.lyricsSeq++
		a.lyrics = alx.Lyrics{}
		a.lyricsSource = ""
		a.lyricsFor = ""
		a.lyricsErr = ""
		a.lyricsLoading = false
		a.lyricsScroll = 0
		return nil
	}
	if !force && a.lyricsFor == song.CLIID {
		return nil
	}
	a.lyricsSeq++
	seq := a.lyricsSeq
	a.lyricsFor = song.CLIID
	a.lyricsLoading = true
	a.lyrics = alx.Lyrics{}
	a.lyricsSource = ""
	a.lyricsErr = ""
	a.lyricsScroll = 0
	a.lyricsManualUntil = time.Time{}
	return cmdLoadLyrics(a.client, *song, seq)
}

func (a *App) startPlay(song alx.Song) tea.Cmd {
	if song.CLIID == "" {
		a.showToast("这首歌缺少 CLI ID，无法播放", toastError)
		return nil
	}
	a.playSeq++
	a.playPending = true
	a.playlistPending = false
	a.pendingSong = song
	a.blurSearch()
	return cmdPlaySong(a.client, song, a.playSeq)
}

func (a *App) togglePause() tea.Cmd {
	if a.playPending {
		return nil
	}
	song := a.currentSong()
	if song == nil {
		a.showToast("还没有正在播放的歌曲", toastInfo)
		return nil
	}
	if !a.state.DaemonActive || a.state.Status == "stopped" {
		return a.startPlay(*song)
	}
	now := time.Now()
	switch a.state.Status {
	case "playing":
		a.state.Position = a.currentPosition()
		a.state.Status = "paused"
		a.stateAt = now
		return tea.Batch(cmdPlayerAction(a.client, "pause"), a.refreshState())
	case "paused":
		a.state.Status = "playing"
		a.stateAt = now
		return tea.Batch(cmdPlayerAction(a.client, "resume"), a.refreshState())
	default:
		return a.startPlay(*song)
	}
}

func (a *App) seekNow(seconds int) tea.Cmd {
	dur := a.trackDuration()
	if dur <= 0 {
		a.showToast("当前歌曲时长未知，暂时无法跳转", toastInfo)
		return nil
	}
	if seconds < 0 {
		seconds = 0
	}
	if float64(seconds) > dur {
		seconds = int(dur)
	}
	a.state.Position = float64(seconds)
	a.stateAt = time.Now()
	return cmdPlayerAction(a.client, fmt.Sprintf("seek:%d", seconds))
}

func (a *App) seekBy(delta float64) tea.Cmd {
	dur := a.trackDuration()
	if dur <= 0 {
		return nil
	}
	pos := a.currentPosition() + delta
	if pos < 0 {
		pos = 0
	}
	if pos > dur {
		pos = dur
	}
	a.state.Position = pos
	a.stateAt = time.Now()
	return cmdPlayerAction(a.client, fmt.Sprintf("seek:%d", int(pos)))
}

func (a *App) adjustVolume(delta int) tea.Cmd {
	vol := a.state.Volume
	if vol <= 0 {
		vol = a.cfg.Player.DefaultVolume
	}
	vol = clamp(vol+delta, 0, 100)
	a.state.Volume = vol
	return cmdPlayerAction(a.client, fmt.Sprintf("volume:%d", vol))
}

func (a *App) nextSong() tea.Cmd {
	if a.playPending || a.currentSong() == nil {
		return nil
	}
	return tea.Batch(cmdPlayerAction(a.client, "next"), a.refreshState())
}

func (a *App) previousSong() tea.Cmd {
	if a.playPending || a.currentSong() == nil {
		return nil
	}
	return tea.Batch(cmdPlayerAction(a.client, "prev"), a.refreshState())
}

func (a *App) stopPlayback() tea.Cmd {
	if a.currentSong() == nil {
		return nil
	}
	a.state.Status = "stopped"
	return tea.Batch(cmdPlayerAction(a.client, "stop"), a.refreshState())
}

func (a *App) setRepeat(mode string) tea.Cmd {
	a.state.Repeat = mode
	return cmdPlayerAction(a.client, "repeat:"+mode)
}

func (a *App) setShuffle(on bool) tea.Cmd {
	a.state.Shuffle = on
	value := "off"
	if on {
		value = "on"
	}
	return cmdPlayerAction(a.client, "shuffle:"+value)
}

func (a *App) downloadSongs(songs []alx.Song) tea.Cmd {
	if len(songs) == 0 {
		a.showToast("没有要下载的歌曲", toastInfo)
		return nil
	}
	return cmdDownload(a.client, songs)
}

func (a *App) addSongsToQueue(songs []alx.Song) tea.Cmd {
	if len(songs) == 0 {
		a.showToast("没有要加入队列的歌曲", toastInfo)
		return nil
	}
	return cmdQueueAdd(a.client, songs)
}

func (a *App) addSongToPlaylist(playlist string, song alx.Song) tea.Cmd {
	if strings.TrimSpace(playlist) == "" {
		a.showToast("请选择歌单", toastInfo)
		return nil
	}
	return cmdAddToPlaylist(a.client, playlist, song)
}

func (a *App) removeSongFromPlaylist(playlist string, song alx.Song) tea.Cmd {
	if playlist == "" || song.CLIID == "" {
		return nil
	}
	return cmdRemoveFromPlaylist(a.client, playlist, song)
}

func (a *App) playPlaylist(name string) tea.Cmd {
	if name == "" {
		a.showToast("请先选择一个歌单", toastInfo)
		return nil
	}
	if len(a.playlistSongs) == 0 {
		a.showToast("这个歌单还没有歌曲", toastInfo)
		return nil
	}
	a.playlistPending = true
	a.playPending = false
	a.showToast("正在开始播放歌单…", toastInfo)
	return tea.Batch(a.switchPage(PagePlayback), cmdPlayPlaylist(a.client, name))
}

func (a *App) reloadPlaylists() tea.Cmd {
	if a.playlistsLoading {
		return nil
	}
	a.playlistsLoading = true
	return cmdLoadPlaylists(a.client)
}

func (a *App) reloadSelectedPlaylistSongs() tea.Cmd {
	name := a.selectedPlaylistName()
	if name == "" {
		return nil
	}
	a.playlistSeq++
	seq := a.playlistSeq
	a.playlistName = name
	a.playlistSongsLoading = true
	a.playlistSongsErr = nil
	return cmdLoadPlaylistSongs(a.client, name, seq)
}

func (a *App) retryLyrics() tea.Cmd {
	return a.loadLyrics(a.currentSong(), true)
}

func (a *App) refreshSettingsData() tea.Cmd {
	if !a.sourcesLoading {
		a.sourcesLoading = true
	}
	if !a.cfgLoading {
		a.cfgLoading = true
	}
	return tea.Batch(cmdLoadConfig(a.client), cmdLoadSources(a.client))
}

func (a *App) refreshSearchPage(page int) tea.Cmd {
	if page < 1 {
		page = 1
	}
	return a.startSearch(page)
}

func (a *App) searchSelect(delta int) {
	if len(a.searchResults) == 0 {
		return
	}
	a.searchSelected = clamp(a.searchSelected+delta, 0, len(a.searchResults)-1)
	ensureVisible(a.searchSelected, &a.searchScroll, max(1, a.searchResultsArea.H), len(a.searchResults))
}

func (a *App) playlistSongSelect(delta int) {
	if len(a.playlistSongs) == 0 {
		return
	}
	a.playlistSongSelected = clamp(a.playlistSongSelected+delta, 0, len(a.playlistSongs)-1)
	ensureVisible(a.playlistSongSelected, &a.playlistScroll, max(1, a.playlistSongsArea.H), len(a.playlistSongs))
}
