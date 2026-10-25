package ui

import (
	"context"
	"errors"
	"strconv"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"alxtui/internal/alx"
)

type clockMsg time.Time
type pollMsg time.Time

type infoMsg struct {
	version    string
	configFile string
	err        error
}

type configMsg struct {
	cfg alx.Config
	err error
}

type sourcesMsg struct {
	sources []alx.Source
	err     error
}

type playlistsMsg struct {
	playlists []alx.Playlist
	err       error
}

type playlistSongsMsg struct {
	name  string
	songs []alx.Song
	err   error
	seq   int
}

type stateMsg struct {
	state alx.PlayerState
	err   error
	seq   int
}

type queueMsg struct {
	queue alx.Queue
	err   error
	seq   int
}

type lyricsMsg struct {
	id     string
	lyrics alx.Lyrics
	source string
	err    error
	seq    int
}

type searchMsg struct {
	seq     int
	keyword string
	source  string
	page    int
	songs   []alx.Song
	err     error
}

type playSongMsg struct {
	song alx.Song
	res  alx.PlayResult
	err  error
	seq  int
}

type queuePlayMsg struct {
	song  alx.Song
	index int
	err   error
	seq   int
}

type playerActionMsg struct {
	action string
	err    error
}

type playlistPlayMsg struct {
	name string
	err  error
}

type addToPlaylistMsg struct {
	playlist string
	song     alx.Song
	err      error
}

type removeFromPlaylistMsg struct {
	playlist string
	song     alx.Song
	err      error
}

type downloadAddMsg struct {
	songs []alx.Song
	res   alx.DownloadAddResult
	err   error
}

type queueAddMsg struct {
	songs []alx.Song
	res   alx.QueueAddResult
	err   error
}

type configSetMsg struct {
	key, value string
	seq        int
	err        error
}

type addSourceMsg struct {
	path string
	seq  int
	err  error
}

type removeSourceMsg struct {
	id  string
	seq int
	err error
}

type createPlaylistMsg struct {
	name    string
	pending *alx.Song
	seq     int
	err     error
	addErr  error
}

type importPlaylistMsg struct {
	res alx.PlaylistImportResult
	seq int
	err error
}

type renamePlaylistMsg struct {
	oldName, newName string
	seq              int
	err              error
}

type deletePlaylistMsg struct {
	name string
	seq  int
	err  error
}

func tickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg { return clockMsg(t) })
}

func pollTickCmd() tea.Cmd {
	return tea.Tick(1200*time.Millisecond, func(t time.Time) tea.Msg { return pollMsg(t) })
}

func cmdInfo(c *alx.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		version, verr := c.Version(ctx)
		path, perr := c.ConfigFile(ctx)
		return infoMsg{version: version, configFile: path, err: errors.Join(verr, perr)}
	}
}

func cmdLoadConfig(c *alx.Client) tea.Cmd {
	return func() tea.Msg {
		cfg, err := c.Config(context.Background())
		return configMsg{cfg: cfg, err: err}
	}
}

func cmdLoadSources(c *alx.Client) tea.Cmd {
	return func() tea.Msg {
		sources, err := c.Sources(context.Background())
		return sourcesMsg{sources: sources, err: err}
	}
}

func cmdLoadPlaylists(c *alx.Client) tea.Cmd {
	return func() tea.Msg {
		lists, err := c.Playlists(context.Background())
		return playlistsMsg{playlists: lists, err: err}
	}
}

func cmdLoadPlaylistSongs(c *alx.Client, name string, seq int) tea.Cmd {
	return func() tea.Msg {
		songs, err := c.PlaylistSongs(context.Background(), name)
		return playlistSongsMsg{name: name, songs: songs, err: err, seq: seq}
	}
}

func cmdLoadState(c *alx.Client, seq int) tea.Cmd {
	return func() tea.Msg {
		st, err := c.State(context.Background())
		return stateMsg{state: st, err: err, seq: seq}
	}
}

func cmdLoadQueue(c *alx.Client, seq int) tea.Cmd {
	return func() tea.Msg {
		q, err := c.Queue(context.Background())
		return queueMsg{queue: q, err: err, seq: seq}
	}
}

func cmdLoadLyrics(c *alx.Client, song alx.Song, seq int) tea.Cmd {
	return func() tea.Msg {
		var backendErr error
		if song.CLIID != "" {
			res, err := c.Lyrics(context.Background(), song.CLIID)
			if err != nil {
				backendErr = err
			} else if !nullLyric(res.Lyric) {
				if parsed := alx.ParseLRC(res.Lyric); usableLyrics(parsed) {
					return lyricsMsg{id: song.CLIID, lyrics: parsed, source: "alx", seq: seq}
				}
			}
		} else if song.Name == "" {
			return lyricsMsg{id: song.Key(), err: errors.New("歌曲缺少 CLI ID 和名称"), seq: seq}
		}

		// alx 0.4.x can return Null"null" for every song when an installed JS
		// source has no lyric implementation. Fall back to LRCLIB rather than
		// showing that sentinel or an empty pane.
		parent := c.SessionContext
		if parent == nil {
			parent = context.Background()
		}
		ctx, cancel := context.WithTimeout(parent, 20*time.Second)
		defer cancel()
		lyrics, source, err := fetchFallbackLyrics(ctx, song)
		if usableLyrics(lyrics) {
			return lyricsMsg{id: song.CLIID, lyrics: lyrics, source: source, seq: seq}
		}
		if err == nil {
			err = backendErr
		}
		if err == nil {
			err = errors.New("未找到可用歌词")
		}
		return lyricsMsg{id: song.CLIID, err: err, seq: seq}
	}
}

func cmdSearch(c *alx.Client, seq int, keyword, source string, page, limit int) tea.Cmd {
	return func() tea.Msg {
		songs, err := c.Search(context.Background(), keyword, source, page, limit)
		return searchMsg{seq: seq, keyword: keyword, source: source, page: page, songs: songs, err: err}
	}
}

func cmdPlaySong(c *alx.Client, song alx.Song, seq int) tea.Cmd {
	return func() tea.Msg {
		res, err := c.Play(context.Background(), song.CLIID)
		return playSongMsg{song: song, res: res, err: err, seq: seq}
	}
}

func cmdPlayQueue(c *alx.Client, song alx.Song, index, seq int) tea.Cmd {
	return func() tea.Msg {
		err := c.QueuePlay(context.Background(), index)
		return queuePlayMsg{song: song, index: index, err: err, seq: seq}
	}
}

func cmdPlayPlaylist(c *alx.Client, name string) tea.Cmd {
	return func() tea.Msg {
		err := c.PlayPlaylist(context.Background(), name, false)
		return playlistPlayMsg{name: name, err: err}
	}
}

func cmdPlayerAction(c *alx.Client, action string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		var err error
		switch {
		case action == "pause":
			err = c.Pause(ctx)
		case action == "resume":
			err = c.Resume(ctx)
		case action == "stop":
			err = c.Stop(ctx)
		case action == "next":
			err = c.Next(ctx)
		case action == "prev":
			err = c.Previous(ctx)
		case len(action) > 5 && action[:5] == "seek:":
			n, _ := strconv.Atoi(action[5:])
			err = c.Seek(ctx, n)
		case len(action) > 7 && action[:7] == "volume:":
			n, _ := strconv.Atoi(action[7:])
			err = c.SetVolume(ctx, n)
		case len(action) > 7 && action[:7] == "repeat:":
			err = c.SetRepeat(ctx, action[7:])
		case len(action) > 8 && action[:8] == "shuffle:":
			err = c.SetShuffle(ctx, action[8:] == "on")
		default:
			err = errors.New("未知播放器操作：" + action)
		}
		return playerActionMsg{action: action, err: err}
	}
}

func cmdAddToPlaylist(c *alx.Client, playlist string, song alx.Song) tea.Cmd {
	return func() tea.Msg {
		err := c.PlaylistAdd(context.Background(), playlist, []alx.Song{song})
		return addToPlaylistMsg{playlist: playlist, song: song, err: err}
	}
}

func cmdRemoveFromPlaylist(c *alx.Client, playlist string, song alx.Song) tea.Cmd {
	return func() tea.Msg {
		err := c.PlaylistRemove(context.Background(), playlist, song.CLIID)
		return removeFromPlaylistMsg{playlist: playlist, song: song, err: err}
	}
}

func cmdDownload(c *alx.Client, songs []alx.Song) tea.Cmd {
	return func() tea.Msg {
		res, err := c.DownloadAdd(context.Background(), songs)
		return downloadAddMsg{songs: songs, res: res, err: err}
	}
}

func cmdQueueAdd(c *alx.Client, songs []alx.Song) tea.Cmd {
	return func() tea.Msg {
		res, err := c.QueueAdd(context.Background(), songs)
		return queueAddMsg{songs: songs, res: res, err: err}
	}
}

func cmdSetConfig(c *alx.Client, key, value string, seq int) tea.Cmd {
	return func() tea.Msg {
		err := c.SetConfig(context.Background(), key, value)
		return configSetMsg{key: key, value: value, seq: seq, err: err}
	}
}

func cmdAddSource(c *alx.Client, path string, seq int) tea.Cmd {
	return func() tea.Msg {
		err := c.SourceAdd(context.Background(), path)
		return addSourceMsg{path: path, seq: seq, err: err}
	}
}

func cmdRemoveSource(c *alx.Client, id string, seq int) tea.Cmd {
	return func() tea.Msg {
		err := c.SourceRemove(context.Background(), id)
		return removeSourceMsg{id: id, seq: seq, err: err}
	}
}

func cmdCreatePlaylist(c *alx.Client, name string, pending *alx.Song, seq int) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		err := c.PlaylistCreate(ctx, name, "")
		var addErr error
		if err == nil && pending != nil {
			addErr = c.PlaylistAdd(ctx, name, []alx.Song{*pending})
		}
		return createPlaylistMsg{name: name, pending: pending, seq: seq, err: err, addErr: addErr}
	}
}

func cmdRenamePlaylist(c *alx.Client, oldName, newName string, seq int) tea.Cmd {
	return func() tea.Msg {
		err := c.PlaylistRename(context.Background(), oldName, newName)
		return renamePlaylistMsg{oldName: oldName, newName: newName, seq: seq, err: err}
	}
}

func cmdImportPlaylist(c *alx.Client, link, name string, seq int) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		res, err := c.PlaylistImport(ctx, link, name)
		return importPlaylistMsg{res: res, seq: seq, err: err}
	}
}

func cmdDeletePlaylist(c *alx.Client, name string, seq int) tea.Cmd {
	return func() tea.Msg {
		err := c.PlaylistDelete(context.Background(), name)
		return deletePlaylistMsg{name: name, seq: seq, err: err}
	}
}

type playlistSearchMsg struct {
	seq             int
	keyword, source string
	page            int
	result          alx.PlaylistSearchResult
	err             error
}

type playlistPreviewMsg struct {
	seq    int
	link   string
	result alx.ImportedPlaylist
	err    error
}

func cmdSearchPlaylists(c *alx.Client, seq int, keyword, source string, page, limit int) tea.Cmd {
	return func() tea.Msg {
		result, err := c.SearchPlaylists(context.Background(), keyword, source, page, limit)
		return playlistSearchMsg{seq: seq, keyword: keyword, source: source, page: page, result: result, err: err}
	}
}

func cmdInspectPlaylist(c *alx.Client, seq int, link string) tea.Cmd {
	return func() tea.Msg {
		result, err := c.InspectPlaylist(context.Background(), link)
		return playlistPreviewMsg{seq: seq, link: link, result: result, err: err}
	}
}
