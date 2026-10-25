package ui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"alxtui/internal/alx"
)

// Update is the central Bubble Tea message dispatcher.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	a.ensureBackendLifecycle()
	if envelope, ok := msg.(backendEnvelope); ok {
		if envelope.epoch != a.lifecycle.epoch.Load() {
			return a, nil
		}
		msg = envelope.msg
	}
	model, cmd := a.update(msg)
	return model, a.backendCommand(cmd)
}

func (a *App) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		return a, nil

	case clockMsg:
		a.clock = time.Time(msg)
		a.spinner = (a.spinner + 1) % len(spinnerFrames)
		if a.toast.text != "" && !a.clock.Before(a.toast.until) {
			a.toast = toastState{}
		}
		return a, tickCmd()

	case pollMsg:
		a.pollScheduled = false
		if a.stateFetching {
			return a, nil
		}
		a.stateFetching = true
		a.stateSeq++
		return a, cmdLoadState(a.client, a.stateSeq)

	case tea.KeyMsg:
		return a, a.handleKey(msg)

	case tea.MouseMsg:
		return a, a.handleMouse(msg)

	case infoMsg:
		a.handleInfo(msg)
		return a, nil

	case configMsg:
		cmd := a.handleConfig(msg)
		if a.page == PageSettings {
			// Source/config data changes the number of lines in the
			// scrollable settings view. A full repaint avoids stale
			// fragments (and an occasional terminal scroll) when the
			// section grows while the user is already on this page.
			return a, tea.Batch(tea.ClearScreen, cmd)
		}
		return a, cmd

	case sourcesMsg:
		a.handleSources(msg)
		if a.page == PageSettings {
			return a, tea.ClearScreen
		}
		return a, nil

	case playlistsMsg:
		return a, a.handlePlaylists(msg)

	case playlistSongsMsg:
		a.handlePlaylistSongs(msg)
		return a, nil

	case stateMsg:
		return a, a.handleState(msg)

	case queueMsg:
		a.handleQueue(msg)
		return a, nil

	case lyricsMsg:
		a.handleLyrics(msg)
		return a, nil

	case searchMsg:
		a.handleSearchResult(msg)
		if msg.err == nil && a.page == PageSearch {
			return a, a.searchInput.Focus()
		}
		return a, nil

	case playSongMsg:
		return a, a.handlePlaySong(msg)

	case playerActionMsg:
		return a, a.handlePlayerAction(msg)

	case playlistPlayMsg:
		return a, a.handlePlaylistPlay(msg)

	case addToPlaylistMsg:
		return a, a.handleAddToPlaylist(msg)

	case removeFromPlaylistMsg:
		return a, a.handleRemoveFromPlaylist(msg)

	case downloadAddMsg:
		a.handleDownloadAdd(msg)
		return a, nil

	case queueAddMsg:
		return a, a.handleQueueAdd(msg)

	case configSetMsg:
		return a, a.handleConfigSet(msg)

	case addSourceMsg:
		return a, a.handleAddSource(msg)

	case removeSourceMsg:
		return a, a.handleRemoveSource(msg)

	case createPlaylistMsg:
		return a, a.handleCreatePlaylist(msg)
	case importPlaylistMsg:
		return a, a.handleImportPlaylist(msg)

	case renamePlaylistMsg:
		return a, a.handleRenamePlaylist(msg)

	case deletePlaylistMsg:
		return a, a.handleDeletePlaylist(msg)

	case themeSavedMsg:
		if msg.err != nil {
			a.themeConfigErr = msg.err
			a.showToast("颜色已应用，但保存失败："+msg.err.Error(), toastError)
			return a, nil
		}
		a.themeConfigErr = nil
		a.showToast("颜色设置已保存", toastSuccess)
		return a, nil
	}

	// Cursor blink and clipboard messages from textinput.
	var cmd tea.Cmd
	if a.dlg != nil && a.dlg.hasInput {
		a.dlg.input, cmd = a.dlg.input.Update(msg)
	} else if a.page == PageSearch && a.searchFocus {
		a.searchInput, cmd = a.searchInput.Update(msg)
	}
	return a, cmd
}

func (a *App) handleKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	if key == "ctrl+c" {
		return tea.Quit
	}
	if a.dlg != nil {
		return a.handleDialogKey(msg)
	}
	if a.menu != nil {
		return a.handleMenuKey(msg)
	}

	// Page navigation is global, except while typing? Tab is always available.
	switch key {
	case "tab":
		return a.switchPage((a.page + 1) % 4)
	case "shift+tab":
		return a.switchPage((a.page + 3) % 4)
	}

	typing := a.page == PageSearch && a.searchFocus
	if !typing {
		if key == "/" && a.page != PageSearch {
			return a.switchPage(PageSearch)
		}
		if len(key) == 1 && key[0] >= '1' && key[0] <= '4' {
			return a.switchPage(Page(key[0] - '1'))
		}
		if key == "q" {
			return tea.Quit
		}
		if key == " " {
			return a.togglePause()
		}
	}

	switch a.page {
	case PageSearch:
		return a.handleSearchKey(msg, typing)
	case PagePlayback:
		return a.handlePlaybackKey(key)
	case PagePlaylists:
		return a.handlePlaylistKey(key)
	case PageSettings:
		return a.handleSettingsKey(key)
	}
	return nil
}

func (a *App) handleSearchKey(msg tea.KeyMsg, typing bool) tea.Cmd {
	key := msg.String()
	if !typing {
		switch key {
		case "/", "i":
			return a.focusSearch()
		case "enter":
			if a.searchSubmitted == "" {
				return a.focusSearch()
			}
			return a.playSelectedSong()
		case "up":
			a.searchSelect(-1)
			return nil
		case "down":
			a.searchSelect(1)
			return nil
		case "[", "pgup":
			if a.searchPage > 1 && !a.searchLoading {
				return a.startSearch(a.searchPage - 1)
			}
			return nil
		case "]", "pgdown":
			if !a.searchLoading && len(a.searchResults) >= a.searchLimit {
				return a.startSearch(a.searchPage + 1)
			}
			return nil
		case "r":
			if a.searchSubmitted != "" {
				return a.startSearch(a.searchPage)
			}
			return nil
		case "esc":
			a.searchFocus = false
			a.searchInput.Blur()
			return nil
		}
		return nil
	}

	// Typing in the search field.
	switch key {
	case "enter":
		dirty := strings.TrimSpace(a.searchInput.Value()) != a.searchSubmitted ||
			a.searchSubmittedSource != a.searchSource
		if dirty || a.searchErr != nil {
			return a.startSearch(1)
		}
		if a.searchLoading {
			return nil
		}
		return a.playSelectedSong()
	case "esc":
		a.blurSearch()
		return nil
	case "up":
		a.searchSelect(-1)
		return nil
	case "down":
		a.searchSelect(1)
		return nil
	case "pgup":
		if a.searchPage > 1 && !a.searchLoading {
			return a.startSearch(a.searchPage - 1)
		}
		return nil
	case "pgdown":
		if !a.searchLoading && len(a.searchResults) >= a.searchLimit {
			return a.startSearch(a.searchPage + 1)
		}
		return nil
	default:
		var cmd tea.Cmd
		a.searchInput, cmd = a.searchInput.Update(msg)
		return cmd
	}
}

func (a *App) handlePlaybackKey(key string) tea.Cmd {
	switch key {
	case "left":
		return a.seekBy(-5)
	case "right":
		return a.seekBy(5)
	case "up":
		return a.adjustVolume(5)
	case "down":
		return a.adjustVolume(-5)
	case "n":
		return a.nextSong()
	case "b":
		return a.previousSong()
	case "s":
		return a.stopPlayback()
	case "L":
		return a.retryLyrics()
	}
	return nil
}

func (a *App) handlePlaylistKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		if a.playlistFocusRight {
			a.playlistSongSelect(-1)
			return nil
		}
		if len(a.playlists) == 0 {
			return nil
		}
		a.playlistSelected = clamp(a.playlistSelected-1, 0, len(a.playlists)-1)
		ensureVisible(a.playlistSelected, &a.playlistListScroll, max(1, a.playlistListArea.H), len(a.playlists))
		return a.selectPlaylist(a.playlistSelected)
	case "down", "j":
		if a.playlistFocusRight {
			a.playlistSongSelect(1)
			return nil
		}
		if len(a.playlists) == 0 {
			return nil
		}
		a.playlistSelected = clamp(a.playlistSelected+1, 0, len(a.playlists)-1)
		ensureVisible(a.playlistSelected, &a.playlistListScroll, max(1, a.playlistListArea.H), len(a.playlists))
		return a.selectPlaylist(a.playlistSelected)
	case "left", "h":
		a.playlistFocusRight = false
	case "right", "l":
		a.playlistFocusRight = true
	case "enter":
		if a.playlistFocusRight {
			return a.playSelectedSong()
		}
		if a.playlistSelected >= 0 && a.playlistSelected < len(a.playlists) {
			a.playlistFocusRight = true
			return a.selectPlaylist(a.playlistSelected)
		}
	case "n":
		return a.openNewPlaylistDialog(nil)
	case "i":
		return a.openImportPlaylistDialog()
	case "d":
		return a.openDeletePlaylistConfirm()
	case "p":
		return a.playPlaylist(a.selectedPlaylistName())
	case "r":
		return a.reloadPlaylists()
	}
	return nil
}

func (a *App) handleSettingsKey(key string) tea.Cmd {
	switch key {
	case "up", "k":
		a.settingsScroll = max(0, a.settingsScroll-1)
	case "down", "j":
		a.settingsScroll++
	case "pgup":
		a.settingsScroll = max(0, a.settingsScroll-8)
	case "pgdown":
		a.settingsScroll += 8
	case "home":
		a.settingsScroll = 0
	case "end":
		a.settingsScroll = 1 << 20
	case "r":
		return a.refreshSettingsData()
	case "a":
		return a.openAddSourceDialog()
	}
	return nil
}

func (a *App) handleMouse(msg tea.MouseMsg) tea.Cmd {
	if a.dlg != nil {
		return a.handleDialogMouse(msg)
	}
	if a.menu != nil {
		return a.handleMenuMouse(msg)
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		return a.handleWheel(msg)
	}
	if msg.Action != tea.MouseActionPress {
		return nil
	}
	hit, ok := a.hitAt(msg.X, msg.Y)
	switch msg.Button {
	case tea.MouseButtonRight:
		if ok {
			switch hit.Kind {
			case HitSearchResult:
				a.searchSelected = hit.Index
				a.openSongMenu(hit.Song, "search", msg.X, msg.Y)
			case HitPlaylistSong:
				a.playlistSongSelected = hit.Index
				a.playlistFocusRight = true
				a.openSongMenu(hit.Song, hit.Value, msg.X, msg.Y)
			case HitMiniInfo, HitNowPlayingInfo:
				if s := a.currentSong(); s != nil {
					a.openSongMenu(*s, "playback", msg.X, msg.Y)
				}
			}
		}
		return nil
	case tea.MouseButtonLeft:
		if !ok {
			return nil
		}
		return a.dispatchHit(hit, msg.X, msg.Y)
	}
	return nil
}

func (a *App) dispatchHit(hit Hit, mouseX, mouseY int) tea.Cmd {
	switch hit.Kind {
	case HitTab:
		return a.switchPage(Page(hit.Index))
	case HitMiniInfo:
		return a.switchPage(PagePlayback)
	case HitNowPlayingInfo:
		return nil
	case HitMiniProgress:
		return a.seekFromBar(hit.Rect, mouseX)
	case HitMiniPrev:
		return a.previousSong()
	case HitMiniPlayPause:
		return a.togglePause()
	case HitMiniNext:
		return a.nextSong()
	case HitPlaybackProgress:
		return a.seekFromBar(hit.Rect, mouseX)
	case HitPlaybackPrev:
		return a.previousSong()
	case HitPlaybackPlayPause:
		return a.togglePause()
	case HitPlaybackNext:
		return a.nextSong()
	case HitPlaybackStop:
		return a.stopPlayback()
	case HitPlaybackVolumeMinus:
		return a.adjustVolume(-5)
	case HitPlaybackVolumePlus:
		return a.adjustVolume(5)
	case HitLyricsRetry:
		return a.retryLyrics()
	case HitSearchInput:
		a.page = PageSearch
		a.searchFocus = true
		return a.searchInput.Focus()
	case HitSearchSource:
		return a.chooseSearchSource(hit.Value)
	case HitSearchResult:
		a.searchSelected = hit.Index
		return a.startPlay(hit.Song)
	case HitSearchPrev:
		if a.searchPage > 1 && !a.searchLoading {
			return a.startSearch(a.searchPage - 1)
		}
	case HitSearchNext:
		if !a.searchLoading {
			return a.startSearch(a.searchPage + 1)
		}
	case HitPlaylistRow:
		a.playlistFocusRight = false
		if hit.Index >= 0 && hit.Index < len(a.playlists) {
			a.playlistSelected = hit.Index
			return a.selectPlaylist(hit.Index)
		}
	case HitPlaylistSong:
		a.playlistSongSelected = hit.Index
		a.playlistFocusRight = true
		return a.startPlay(hit.Song)
	case HitPlaylistNew:
		return a.openNewPlaylistDialog(nil)
	case HitPlaylistImport:
		return a.openImportPlaylistDialog()
	case HitPlaylistPlayAll:
		return a.playPlaylist(a.selectedPlaylistName())
	case HitPlaylistDelete:
		return a.openDeletePlaylistConfirm()
	case HitPlaylistRename:
		return a.openRenamePlaylistDialog()
	case HitPlaylistRefresh:
		return a.reloadPlaylists()
	case HitSettingsSourceChip:
		return a.chooseSearchSource(hit.Value)
	case HitSettingsQualityChip:
		return a.saveQuality(hit.Value)
	case HitSettingsAddSource:
		return a.openAddSourceDialog()
	case HitSettingsRemoveSource:
		return a.openRemoveSourceConfirm()
	case HitSettingsSourceRow:
		if hit.Index >= 0 && hit.Index < len(a.sources) {
			a.sourcesSelected = hit.Index
		}
		return nil
	case HitSettingsRefresh:
		return a.refreshSettingsData()
	case HitSettingsAccentColor:
		return a.setAccentColor(hit.Value)
	case HitSettingsAccentCustom:
		return a.openAccentColorDialog()
	case HitSettingsTextColor:
		return a.setTextColor(hit.Value)
	case HitSettingsTextCustom:
		return a.openTextColorDialog()
	case HitSettingsResetColors:
		return a.resetThemeColors()
	case HitSettingsDownloadDir:
		return a.openDownloadDirDialog()
	case HitSettingsDirPreset:
		return a.saveDownloadDir(hit.Value)
	case HitSettingsRepeat:
		return a.setRepeat(hit.Value)
	case HitSettingsShuffle:
		return a.setShuffle(hit.Value == "on")
	case HitSettingsVolumeMinus:
		return a.adjustVolume(-5)
	case HitSettingsVolumePlus:
		return a.adjustVolume(5)
	}
	return nil
}

func (a *App) seekFromBar(bar Rect, mouseX int) tea.Cmd {
	if bar.W <= 0 {
		return nil
	}
	dur := a.trackDuration()
	if dur <= 0 {
		a.showToast("当前歌曲时长未知，暂时无法跳转", toastInfo)
		return nil
	}
	ratio := (float64(mouseX-bar.X) + 0.5) / float64(bar.W)
	ratio = clampFloat(ratio, 0, 1)
	target := int(ratio * dur)
	if target < 0 {
		target = 0
	}
	a.state.Position = float64(target)
	a.stateAt = time.Now()
	return cmdPlayerAction(a.client, fmt.Sprintf("seek:%d", target))
}

func (a *App) chooseSearchSource(code string) tea.Cmd {
	if code == "" {
		return nil
	}
	changed := a.searchSource != code
	a.searchSource = code
	cmdCfg := cmdSetConfig(a.client, "source.default_source", code, 0)
	if changed && a.searchSubmitted != "" {
		return tea.Batch(cmdCfg, a.startSearch(1))
	}
	return cmdCfg
}

func (a *App) handleWheel(msg tea.MouseMsg) tea.Cmd {
	delta := 3
	if msg.Button == tea.MouseButtonWheelUp {
		delta = -3
	}
	switch a.page {
	case PageSearch:
		if a.searchResultsArea.Contains(msg.X, msg.Y) {
			h := max(1, a.searchResultsArea.H)
			a.searchScroll = clamp(a.searchScroll+delta, 0, max(0, len(a.searchResults)-h))
		}
	case PagePlayback:
		if a.lyricsArea.Contains(msg.X, msg.Y) && len(a.lyrics.Lines) > 0 {
			h := max(1, a.lyricsArea.H)
			if !a.lyrics.Synced {
				a.lyricsScroll = clamp(a.lyricsScroll+delta, 0, max(0, len(a.lyrics.Lines)-h))
			} else {
				if !time.Now().Before(a.lyricsManualUntil) {
					active := a.lyrics.IndexAt(time.Duration(a.currentPosition() * float64(time.Second)))
					if active < 0 {
						active = 0
					}
					a.lyricsScroll = active - h/2
				}
				a.lyricsScroll = clamp(a.lyricsScroll+delta, 0, max(0, len(a.lyrics.Lines)-h))
			}
			a.lyricsManualUntil = time.Now().Add(8 * time.Second)
		} else if a.queueArea.Contains(msg.X, msg.Y) && len(a.queue) > 0 {
			h := max(1, a.queueArea.H)
			a.queueManualUntil = time.Now().Add(8 * time.Second)
			a.queueScroll = clamp(a.queueScroll+delta, 0, max(0, len(a.queue)-h))
		}
	case PagePlaylists:
		if a.playlistListArea.Contains(msg.X, msg.Y) && len(a.playlists) > 0 {
			h := max(1, a.playlistListArea.H)
			a.playlistListScroll = clamp(a.playlistListScroll+delta, 0, max(0, len(a.playlists)-h))
		} else if a.playlistSongsArea.Contains(msg.X, msg.Y) && len(a.playlistSongs) > 0 {
			h := max(1, a.playlistSongsArea.H)
			a.playlistScroll = clamp(a.playlistScroll+delta, 0, max(0, len(a.playlistSongs)-h))
		}
	case PageSettings:
		a.settingsScroll = max(0, a.settingsScroll+delta)
	}
	return nil
}

func (a *App) handleInfo(msg infoMsg) {
	a.alxVersion = prettyText(msg.version)
	a.configFile = prettyText(msg.configFile)
	a.backendErr = msg.err
}

func (a *App) handleConfig(msg configMsg) tea.Cmd {
	a.cfgLoading = false
	if msg.err != nil {
		a.cfgErr = msg.err
		if !a.cfgLoaded {
			a.showToast("读取配置失败："+msg.err.Error(), toastError)
		}
		return nil
	}
	a.cfgErr = nil
	a.cfg = msg.cfg
	a.cfgLoaded = true
	a.searchSource = normalizePlatform(a.cfg.Source.DefaultSource)
	if q := strings.TrimSpace(a.cfg.Source.DefaultQuality); q != "" {
		a.quality = q
	} else if a.quality == "" {
		a.quality = "320k"
	}
	if dir := strings.TrimSpace(a.cfg.Download.OutputDir); dir != "" {
		a.downloadDir = dir
	}
	return nil
}

func normalizePlatform(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	for _, p := range searchPlatforms {
		if v == p {
			return v
		}
	}
	return "all"
}

func (a *App) handleSources(msg sourcesMsg) {
	a.sourcesLoading = false
	if msg.err != nil {
		a.sourcesErr = msg.err
		return
	}
	a.sourcesErr = nil
	oldID := ""
	if src := a.selectedSource(); src != nil {
		oldID = src.ID
	}
	a.sources = msg.sources
	a.sourcesLoaded = true
	found := -1
	if oldID != "" {
		for i := range a.sources {
			if a.sources[i].ID == oldID {
				found = i
				break
			}
		}
	}
	if found < 0 {
		if len(a.sources) > 0 {
			found = 0
		}
	}
	a.sourcesSelected = found
}

func (a *App) handlePlaylists(msg playlistsMsg) tea.Cmd {
	a.playlistsLoading = false
	if a.playlistImportRefreshPending {
		a.playlistImportRefreshPending = false
		return a.reloadPlaylists()
	}
	if msg.err != nil {
		a.playlistsErr = msg.err
		a.showToast("歌单加载失败："+prettyText(msg.err.Error()), toastError)
		return nil
	}
	a.playlistsErr = nil
	a.playlistsLoaded = true
	oldName := a.selectedPlaylistName()
	preferred := a.focusPlaylistName
	a.focusPlaylistName = ""
	a.playlists = msg.playlists
	target := -1
	if preferred != "" {
		for i := range a.playlists {
			if a.playlists[i].Name == preferred {
				target = i
				break
			}
		}
	}
	if target < 0 && oldName != "" {
		for i := range a.playlists {
			if a.playlists[i].Name == oldName {
				target = i
				break
			}
		}
	}
	if target < 0 && len(a.playlists) > 0 {
		for i := range a.playlists {
			if a.playlists[i].Name == "Favorites" {
				target = i
				break
			}
		}
	}
	if target < 0 && len(a.playlists) > 0 {
		target = 0
	}
	a.playlistSelected = target
	clampScroll(&a.playlistListScroll, len(a.playlists), max(1, a.playlistListArea.H))
	if target < 0 {
		a.playlistName = ""
		a.playlistSongs = nil
		a.playlistSongsErr = nil
		a.playlistSongsLoading = false
		return nil
	}
	return a.selectPlaylist(target)
}

func (a *App) handlePlaylistSongs(msg playlistSongsMsg) {
	if msg.seq != a.playlistSeq {
		return
	}
	if msg.name != a.selectedPlaylistName() {
		return
	}
	a.playlistSongsLoading = false
	if msg.err != nil {
		a.playlistSongsErr = msg.err
		a.playlistSongs = nil
		a.playlistSongSelected = 0
		a.playlistScroll = 0
		return
	}
	a.playlistSongsErr = nil
	a.playlistSongs = msg.songs
	a.playlistSongSelected = clamp(a.playlistSongSelected, 0, max(0, len(msg.songs)-1))
	clampScroll(&a.playlistScroll, len(msg.songs), max(1, a.playlistSongsArea.H))
}

func (a *App) handleState(msg stateMsg) tea.Cmd {
	if msg.seq != a.stateSeq {
		return nil
	}
	a.stateFetching = false
	if msg.err != nil {
		a.stateErr = msg.err
		if !a.daemonKnown {
			a.showToast("无法连接播放后台："+msg.err.Error(), toastError)
		}
		a.daemonKnown = true
		return pollTickCmd()
	}
	a.stateErr = nil
	a.daemonKnown = true
	oldID := a.currentSongID()
	oldQueueLen := len(a.queue)
	wasLoaded := a.queueLoaded
	a.state = msg.state
	a.stateAt = time.Now()
	newID := a.currentSongID()
	if a.playlistPending && (msg.state.CurrentSong != nil || msg.state.Status == "playing") {
		a.playlistPending = false
	}

	var cmds []tea.Cmd
	if oldID != newID {
		a.queueManualUntil = time.Time{}
		if msg.state.CurrentSong != nil {
			cmds = append(cmds, a.loadLyrics(msg.state.CurrentSong, false))
		} else {
			cmds = append(cmds, a.loadLyrics(nil, false))
		}
		cmds = append(cmds, a.refreshQueue())
	} else if newID != "" && a.lyricsFor != newID {
		cmds = append(cmds, a.loadLyrics(msg.state.CurrentSong, false))
	}
	if !wasLoaded || msg.state.QueueLength != oldQueueLen {
		cmds = append(cmds, a.refreshQueue())
	}
	if !a.pollScheduled {
		a.pollScheduled = true
		cmds = append(cmds, pollTickCmd())
	}
	return tea.Batch(cmds...)
}

func (a *App) handleQueue(msg queueMsg) {
	if msg.seq != a.queueSeq {
		return
	}
	a.queueLoading = false
	if msg.err != nil {
		a.queueLoaded = false
		return
	}
	a.queue = msg.queue.Songs
	a.queueIndex = msg.queue.CurrentIndex
	a.queueLoaded = true
	clampScroll(&a.queueScroll, len(a.queue), max(1, a.queueArea.H))
}

func (a *App) handleLyrics(msg lyricsMsg) {
	if msg.seq != a.lyricsSeq {
		return
	}
	a.lyricsLoading = false
	if msg.err != nil {
		a.lyrics = alx.Lyrics{}
		a.lyricsSource = ""
		a.lyricsErr = msg.err.Error()
		return
	}
	a.lyrics = msg.lyrics
	a.lyricsSource = msg.source
	a.lyricsErr = ""
	a.lyricsScroll = 0
	a.lyricsManualUntil = time.Time{}
}

func (a *App) handleSearchResult(msg searchMsg) {
	if msg.seq != a.searchSeq {
		return
	}
	a.searchLoading = false
	if msg.err != nil {
		a.searchErr = msg.err
		a.searchSubmitted = msg.keyword
		a.searchSubmittedSource = msg.source
		a.searchPage = msg.page
		a.searchSelected = 0
		a.searchScroll = 0
		return
	}
	a.searchErr = nil
	a.searchResults = msg.songs
	a.searchSubmitted = msg.keyword
	a.searchSubmittedSource = msg.source
	a.searchPage = msg.page
	a.searchSelected = 0
	a.searchScroll = 0
	a.searchFocus = true
}

func (a *App) handlePlaySong(msg playSongMsg) tea.Cmd {
	if msg.seq != a.playSeq {
		return nil
	}
	a.playPending = false
	if msg.err != nil {
		a.showToast("播放失败："+msg.err.Error(), toastError)
		return a.refreshState()
	}
	song := msg.song
	a.state.CurrentSong = &song
	a.state.Status = "playing"
	a.state.DaemonActive = true
	a.state.Position = 0
	a.state.Duration = song.Duration().Seconds()
	a.state.QueueLength = 1
	a.stateAt = time.Now()
	a.queue = nil
	a.queueLoaded = false
	a.invalidateState()
	a.invalidateQueue()
	cmds := []tea.Cmd{
		a.loadLyrics(&song, true),
		a.refreshState(),
		a.refreshQueue(),
	}
	a.showToast("正在播放："+truncate(song.Label(), 40), toastSuccess)
	return tea.Batch(cmds...)
}

func (a *App) handlePlayerAction(msg playerActionMsg) tea.Cmd {
	if msg.err != nil {
		a.showToast("播放器操作失败："+msg.err.Error(), toastError)
		return a.refreshState()
	}
	if strings.HasPrefix(msg.action, "next") || strings.HasPrefix(msg.action, "prev") {
		return a.refreshState()
	}
	if msg.action == "stop" {
		a.state.CurrentSong = nil
		a.state.Status = "stopped"
		a.state.QueueLength = 0
		a.queue = nil
		a.queueLoaded = false
		a.loadLyrics(nil, false)
		return tea.Batch(a.refreshState(), a.refreshQueue())
	}
	return nil
}

func (a *App) handlePlaylistPlay(msg playlistPlayMsg) tea.Cmd {
	a.playlistPending = false
	if msg.err != nil {
		a.showToast("播放歌单失败："+msg.err.Error(), toastError)
		return a.refreshState()
	}
	a.showToast("正在播放歌单「"+truncate(msg.name, 30)+"」", toastSuccess)
	a.invalidateState()
	a.invalidateQueue()
	a.queueLoaded = false
	return tea.Batch(
		a.switchPage(PagePlayback),
		a.refreshState(),
		a.refreshQueue(),
	)
}

func (a *App) handleAddToPlaylist(msg addToPlaylistMsg) tea.Cmd {
	if msg.err != nil {
		a.showToast("添加到歌单失败："+msg.err.Error(), toastError)
		return nil
	}
	a.showToast("已添加到歌单「"+msg.playlist+"」", toastSuccess)
	return a.reloadPlaylists()
}

func (a *App) handleRemoveFromPlaylist(msg removeFromPlaylistMsg) tea.Cmd {
	if msg.err != nil {
		a.showToast("从歌单移除失败："+msg.err.Error(), toastError)
		return nil
	}
	a.showToast("已从歌单「"+msg.playlist+"」移除", toastSuccess)
	return a.reloadPlaylists()
}

func (a *App) handleDownloadAdd(msg downloadAddMsg) {
	if msg.err != nil {
		a.showToast("加入下载队列失败："+msg.err.Error(), toastError)
		return
	}
	names := msg.res.Songs
	if len(names) == 0 {
		for _, s := range msg.songs {
			names = append(names, s.Name)
		}
	}
	label := ""
	if len(names) > 0 {
		if len(names) > 3 {
			label = prettyText(strings.Join(names[:3], "、")) + "…"
		} else {
			label = prettyText(strings.Join(names, "、"))
		}
	}
	if label == "" {
		a.showToast("已加入下载队列", toastSuccess)
	} else {
		a.showToast("已加入下载队列："+label, toastSuccess)
	}
}

func (a *App) handleQueueAdd(msg queueAddMsg) tea.Cmd {
	if msg.err != nil {
		a.showToast("加入播放队列失败："+msg.err.Error(), toastError)
		return nil
	}
	a.showToast("已加入播放队列", toastSuccess)
	return tea.Batch(a.refreshQueue(), a.refreshState())
}

func (a *App) handleConfigSet(msg configSetMsg) tea.Cmd {
	if msg.err != nil {
		if a.dlg != nil && a.dlg.seq == msg.seq {
			a.dlg.busy = false
			a.dlg.err = msg.err.Error()
			return nil
		}
		a.showToast("保存设置失败："+msg.err.Error(), toastError)
		a.cfgLoading = true
		return cmdLoadConfig(a.client)
	}
	switch msg.key {
	case "source.default_source":
		a.searchSource = normalizePlatform(msg.value)
	case "source.default_quality":
		a.quality = msg.value
	case "download.output_dir":
		a.downloadDir = msg.value
	}
	if a.dlg != nil && a.dlg.seq == msg.seq {
		a.closeDialog()
		a.showToast("设置已保存", toastSuccess)
		return nil
	}
	a.showToast("设置已保存", toastSuccess)
	if msg.key == "source.default_source" &&
		a.searchSubmitted != "" && a.searchSubmittedSource != a.searchSource {
		return a.startSearch(1)
	}
	return nil
}

func (a *App) handleAddSource(msg addSourceMsg) tea.Cmd {
	if msg.err != nil {
		if a.dlg != nil && a.dlg.seq == msg.seq {
			a.dlg.busy = false
			a.dlg.err = msg.err.Error()
			return nil
		}
		a.showToast("添加音源失败："+msg.err.Error(), toastError)
		return nil
	}
	if a.dlg != nil && a.dlg.seq == msg.seq {
		a.closeDialog()
	}
	a.showToast("音源已添加", toastSuccess)
	a.sourcesLoading = true
	a.cfgLoading = true
	return tea.Batch(cmdLoadSources(a.client), cmdLoadConfig(a.client))
}

func (a *App) handleRemoveSource(msg removeSourceMsg) tea.Cmd {
	if msg.err != nil {
		if a.dlg != nil && a.dlg.seq == msg.seq {
			a.dlg.busy = false
			a.dlg.err = msg.err.Error()
			return nil
		}
		a.showToast("移除音源失败："+msg.err.Error(), toastError)
		return nil
	}
	if a.dlg != nil && a.dlg.seq == msg.seq {
		a.closeDialog()
	}
	a.sourcesSelected = -1
	a.showToast("音源已移除", toastSuccess)
	a.sourcesLoading = true
	return cmdLoadSources(a.client)
}

func (a *App) handleCreatePlaylist(msg createPlaylistMsg) tea.Cmd {
	if msg.err != nil {
		if a.dlg != nil && a.dlg.seq == msg.seq {
			a.dlg.busy = false
			a.dlg.err = msg.err.Error()
			return nil
		}
		a.showToast("创建歌单失败："+msg.err.Error(), toastError)
		return nil
	}
	if a.dlg != nil && a.dlg.seq == msg.seq {
		a.closeDialog()
	}
	a.focusPlaylistName = msg.name
	if msg.addErr != nil {
		a.showToast("歌单已创建，但添加歌曲失败："+msg.addErr.Error(), toastError)
	} else {
		a.showToast("歌单「"+msg.name+"」已创建", toastSuccess)
	}
	return a.reloadPlaylists()
}

func (a *App) handleRenamePlaylist(msg renamePlaylistMsg) tea.Cmd {
	if msg.err != nil {
		if a.dlg != nil && a.dlg.seq == msg.seq {
			a.dlg.busy = false
			a.dlg.err = msg.err.Error()
			return nil
		}
		a.showToast("重命名失败："+msg.err.Error(), toastError)
		return nil
	}
	if a.dlg != nil && a.dlg.seq == msg.seq {
		a.closeDialog()
	}
	a.focusPlaylistName = msg.newName
	a.showToast("歌单已重命名为「"+msg.newName+"」", toastSuccess)
	return a.reloadPlaylists()
}

func (a *App) handleImportPlaylist(msg importPlaylistMsg) tea.Cmd {
	if msg.err != nil {
		if a.dlg != nil && a.dlg.seq == msg.seq {
			a.dlg.busy = false
			a.dlg.err = msg.err.Error()
			return nil
		}
		a.showToast("导入歌单失败："+msg.err.Error(), toastError)
		return nil
	}
	if a.dlg != nil && a.dlg.seq == msg.seq {
		a.closeDialog()
	}
	a.focusPlaylistName = msg.res.Name
	message := fmt.Sprintf("已导入歌单「%s」· %d 首", msg.res.Name, msg.res.Count)
	if len(msg.res.Warnings) > 0 {
		message += "；" + strings.Join(msg.res.Warnings, "；")
	}
	a.showToast(message, toastSuccess)
	if a.playlistsLoading {
		// The in-flight request may have read the database before import.
		// Load once more when it finishes and keep the preferred name intact.
		a.playlistImportRefreshPending = true
		return nil
	}
	return a.reloadPlaylists()
}

func (a *App) handleDeletePlaylist(msg deletePlaylistMsg) tea.Cmd {
	if msg.err != nil {
		if a.dlg != nil && a.dlg.seq == msg.seq {
			a.dlg.busy = false
			a.dlg.err = msg.err.Error()
			return nil
		}
		a.showToast("删除歌单失败："+msg.err.Error(), toastError)
		return nil
	}
	if a.dlg != nil && a.dlg.seq == msg.seq {
		a.closeDialog()
	}
	if a.selectedPlaylistName() == msg.name {
		a.focusPlaylistName = "Favorites"
	}
	a.showToast("歌单「"+msg.name+"」已删除", toastSuccess)
	return a.reloadPlaylists()
}
