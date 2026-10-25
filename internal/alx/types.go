package alx

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Song is a track returned by the alx CLI (search results, playlists, queue).
type Song struct {
	CLIID     string          `json:"cli_id"`
	SongID    string          `json:"song_id"`
	Name      string          `json:"name"`
	Singer    string          `json:"singer"`
	Source    string          `json:"source"`
	Interval  string          `json:"interval"`
	AlbumName string          `json:"album_name"`
	AlbumID   string          `json:"album_id"`
	PicURL    string          `json:"pic_url"`
	Songmid   string          `json:"songmid"`
	Hash      string          `json:"hash"`
	Extra     json.RawMessage `json:"extra"`
}

// Key returns a stable identifier for a song inside the UI.
func (s Song) Key() string {
	if s.CLIID != "" {
		return s.CLIID
	}
	return s.Source + ":" + s.SongID
}

// Duration returns the song length parsed from its "mm:ss" interval.
func (s Song) Duration() time.Duration {
	return ParseInterval(s.Interval)
}

// Label is the "artist - title" presentation used in menus.
func (s Song) Label() string {
	if s.Singer == "" {
		return s.Name
	}
	return s.Name + " — " + s.Singer
}

// ParseInterval parses "03:46", "1:02:03" or a bare number of seconds.
func ParseInterval(v string) time.Duration {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	parts := strings.Split(v, ":")
	if len(parts) == 1 {
		n, err := strconv.Atoi(parts[0])
		if err != nil {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	var total time.Duration
	for _, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n < 0 {
			return 0
		}
		total = total*60 + time.Duration(n)*time.Second
	}
	return total
}

// FormatSeconds renders a duration as mm:ss (or h:mm:ss).
func FormatSeconds(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	total := int(sec)
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// PlayerState mirrors `alx state --json`.
type PlayerState struct {
	CurrentSong  *Song   `json:"current_song"`
	DaemonActive bool    `json:"daemon_active"`
	Duration     float64 `json:"duration"`
	Position     float64 `json:"position"`
	QueueLength  int     `json:"queue_length"`
	Repeat       string  `json:"repeat"`
	Shuffle      bool    `json:"shuffle"`
	Status       string  `json:"status"`
	Volume       int     `json:"volume"`
}

// Queue mirrors `alx queue show --json`.
type Queue struct {
	Songs        []Song `json:"songs"`
	CurrentIndex int    `json:"current_index"`
}

// PlayResult mirrors `alx play --json`.
type PlayResult struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Singer     string `json:"singer"`
	Source     string `json:"source"`
	Status     string `json:"status"`
	URL        string `json:"url"`
	QueueCount int    `json:"queue_count"`
}

// LyricResponse mirrors `alx lyric --json`.
type LyricResponse struct {
	CLIID  string `json:"cli_id"`
	Lyric  string `json:"lyric"`
	Name   string `json:"name"`
	Singer string `json:"singer"`
	SongID string `json:"song_id"`
	Track  string `json:"track"`
}

// LyricLine is one timed line of an LRC document.
type LyricLine struct {
	Time time.Duration
	Text string
}

// Playlist is a playlist row from `alx playlist list --json`.
type Playlist struct {
	Name        string
	Description string
	Count       int
}

// PlaylistImportResult mirrors `alx playlist import --json`.
type PlaylistImportResult struct {
	Name     string   `json:"name"`
	Count    int      `json:"count"`
	Source   string   `json:"source"`
	Warnings []string `json:"warnings,omitempty"`
}

// UnmarshalJSON accepts both the array form returned by the CLI
// (["name","description",count]) and a future object form.
func (p *Playlist) UnmarshalJSON(data []byte) error {
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		if len(arr) > 0 {
			_ = json.Unmarshal(arr[0], &p.Name)
		}
		if len(arr) > 1 {
			_ = json.Unmarshal(arr[1], &p.Description)
		}
		if len(arr) > 2 {
			_ = json.Unmarshal(arr[2], &p.Count)
		}
		return nil
	}
	var obj struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Count       int    `json:"count"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	p.Name, p.Description, p.Count = obj.Name, obj.Description, obj.Count
	return nil
}

// Source is an installed custom JavaScript music source.
type Source struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Author      string `json:"author"`
	Homepage    string `json:"homepage"`
	Repository  string `json:"repository"`
	ScriptPath  string `json:"script_path"`
	SourceURL   string `json:"source_url"`
	ContentHash string `json:"content_hash"`
	Platforms   string `json:"platforms"`
	Qualities   string `json:"qualities"`
	Enabled     bool   `json:"enabled"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// PlatformList decodes the JSON-encoded platforms field.
func (s Source) PlatformList() []string {
	var out []string
	if s.Platforms != "" {
		_ = json.Unmarshal([]byte(s.Platforms), &out)
	}
	return out
}

// QualityMap decodes the JSON-encoded qualities field.
func (s Source) QualityMap() map[string][]string {
	out := map[string][]string{}
	if s.Qualities != "" {
		_ = json.Unmarshal([]byte(s.Qualities), &out)
	}
	return out
}

// Config is the subset of alx configuration used by the TUI.
type Config struct {
	Source struct {
		DefaultSource    string   `json:"default_source"`
		DefaultQuality   string   `json:"default_quality"`
		QualityFallback  []string `json:"quality_fallback"`
		JSPriority       bool     `json:"js_priority"`
		Priority         []string `json:"priority"`
		PlatformPriority []string `json:"platform_priority"`
	} `json:"source"`
	Player struct {
		DefaultVolume int      `json:"default_volume"`
		Repeat        string   `json:"repeat"`
		Shuffle       bool     `json:"shuffle"`
		MPVArgs       []string `json:"mpv_args"`
		EnableMPRIS   bool     `json:"enable_mpris"`
	} `json:"player"`
	Download struct {
		OutputDir      string `json:"output_dir"`
		FilenameTmpl   string `json:"filename_template"`
		MaxConcurrent  int    `json:"max_concurrent"`
		SkipExisting   bool   `json:"skip_existing"`
		EmbedLyrics    bool   `json:"embed_lyrics"`
		SaveLyricsFile bool   `json:"save_lyrics_file"`
	} `json:"download"`
}

// DownloadTask mirrors one row of `alx download list --json`.
type DownloadTask struct {
	ID              int     `json:"id"`
	SongID          string  `json:"song_id"`
	Source          string  `json:"source"`
	Name            string  `json:"name"`
	Singer          string  `json:"singer"`
	Quality         string  `json:"quality"`
	Status          string  `json:"status"`
	Progress        float64 `json:"progress"`
	BytesDownloaded int64   `json:"bytes_downloaded"`
	BytesTotal      int64   `json:"bytes_total"`
	ErrorMessage    string  `json:"error_message"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
}

// DownloadAddResult mirrors `alx download add --json`.
type DownloadAddResult struct {
	Songs  []string `json:"songs"`
	Status string   `json:"status"`
}

// QueueAddResult mirrors `alx queue add --json`.
type QueueAddResult struct {
	Count  int    `json:"count"`
	Status string `json:"status"`
}

// RemotePlaylist is metadata from the native public playlist catalog.
type RemotePlaylist struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
	Link        string `json:"link"`
	Author      string `json:"author"`
	PicURL      string `json:"pic_url"`
	Count       int    `json:"count"`
	PlayCount   int64  `json:"play_count"`
}

type PlaylistSearchResult struct {
	Playlists []RemotePlaylist `json:"playlists"`
	Total     int              `json:"total"`
	Page      int              `json:"page"`
	Limit     int              `json:"limit"`
	HasMore   bool             `json:"has_more"`
	Warnings  []string         `json:"warnings,omitempty"`
}

// ImportedPlaylist is a read-only remote preview; its songs are cached by CLI.
type ImportedPlaylist struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Source      string   `json:"source"`
	Link        string   `json:"link"`
	Songs       []Song   `json:"songs"`
	Total       int      `json:"total"`
	Warnings    []string `json:"warnings,omitempty"`
}
