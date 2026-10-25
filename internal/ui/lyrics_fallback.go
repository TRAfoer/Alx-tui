package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"alxtui/internal/alx"
)

// LRCLIB is used only when alx returns no usable lyric. The installed JS
// sources used by alx currently return the string Null"null" for every song
// instead of falling back to the native providers, so a fallback here keeps
// the lyrics pane useful without touching the user's alx config or database.
var lrclibBaseURL = "https://lrclib.net"

var lrclibHTTPClient = &http.Client{Timeout: 12 * time.Second}

type lrclibRecord struct {
	ID           int     `json:"id"`
	TrackName    string  `json:"trackName"`
	ArtistName   string  `json:"artistName"`
	AlbumName    string  `json:"albumName"`
	Duration     float64 `json:"duration"`
	Instrumental bool    `json:"instrumental"`
	PlainLyrics  string  `json:"plainLyrics"`
	SyncedLyrics string  `json:"syncedLyrics"`
}

// nullLyric reports whether alx produced its null sentinel rather than lyrics.
// Values such as "", "null" and Null"null" are treated as missing. Normal LRC
// text that merely contains the word "null" is not affected.
func nullLyric(doc string) bool {
	if strings.TrimSpace(doc) == "" {
		return true
	}
	var b strings.Builder
	for _, r := range strings.ToLower(doc) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	switch b.String() {
	case "null", "nullnull", "nil":
		return true
	}
	return false
}

func usableLyrics(l alx.Lyrics) bool {
	for _, line := range l.Lines {
		if strings.TrimSpace(line.Text) != "" {
			return true
		}
	}
	return false
}

func primaryArtist(s string) string {
	s = strings.TrimSpace(prettyText(s))
	lower := strings.ToLower(s)
	for _, marker := range []string{" feat. ", " feat ", " ft. ", " ft "} {
		if i := strings.Index(lower, marker); i > 0 {
			s = s[:i]
			lower = strings.ToLower(s)
		}
	}
	for _, sep := range []string{"、", ",", "，", "/", "&", ";", "；", "|"} {
		if i := strings.Index(s, sep); i > 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

func normalizeMatch(s string) string {
	s = prettyText(s)
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type lrclibQuery struct {
	track  string
	artist string
}

// fetchFallbackLyrics looks up the song on LRCLIB. It returns the parsed lyric
// and a display label for the source.
func fetchFallbackLyrics(ctx context.Context, song alx.Song) (alx.Lyrics, string, error) {
	title := strings.TrimSpace(prettyText(song.Name))
	if title == "" {
		return alx.Lyrics{}, "", errors.New("歌曲名称为空，无法查找歌词")
	}
	primary := primaryArtist(song.Singer)
	fullArtist := strings.TrimSpace(prettyText(song.Singer))
	queries := []lrclibQuery{{title, primary}, {title, fullArtist}, {title, ""}}
	seenQuery := map[string]bool{}
	seenRecord := map[int]bool{}
	var candidates []lrclibRecord
	var lastErr error

	for _, q := range queries {
		if q.artist == "" && len(candidates) > 0 {
			continue
		}
		key := q.track + "\x00" + q.artist
		if seenQuery[key] {
			continue
		}
		seenQuery[key] = true
		records, err := searchLRCLIB(ctx, q.track, q.artist)
		if err != nil {
			lastErr = err
			continue
		}
		for _, rec := range records {
			if seenRecord[rec.ID] {
				continue
			}
			seenRecord[rec.ID] = true
			candidates = append(candidates, rec)
		}
		if best, score := pickLRCLIB(candidates, song); best != nil && score >= 150 {
			return lrclibLyrics(*best)
		}
	}

	best, _ := pickLRCLIB(candidates, song)
	if best == nil {
		if lastErr != nil {
			return alx.Lyrics{}, "", fmt.Errorf("歌词回退失败: %w", lastErr)
		}
		return alx.Lyrics{}, "", errors.New("未找到可用歌词")
	}
	return lrclibLyrics(*best)
}

func lrclibLyrics(rec lrclibRecord) (alx.Lyrics, string, error) {
	doc := strings.TrimSpace(rec.SyncedLyrics)
	if doc == "" {
		doc = strings.TrimSpace(rec.PlainLyrics)
	}
	if doc == "" {
		return alx.Lyrics{}, "", errors.New("未找到可用歌词")
	}
	lyrics := alx.ParseLRC(doc)
	if !usableLyrics(lyrics) {
		return alx.Lyrics{}, "", errors.New("未找到可用歌词")
	}
	return lyrics, "LRCLIB", nil
}

func searchLRCLIB(ctx context.Context, track, artist string) ([]lrclibRecord, error) {
	params := url.Values{}
	params.Set("track_name", track)
	if artist != "" {
		params.Set("artist_name", artist)
	}
	endpoint := strings.TrimRight(lrclibBaseURL, "/") + "/api/search?" + params.Encode()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			delay := time.Duration(300*attempt) * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "alx-tui/"+Version+" (terminal music player)")
		resp, err := lrclibHTTPClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode == http.StatusServiceUnavailable || resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("LRCLIB 暂时不可用 (HTTP %d)", resp.StatusCode)
			continue
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("LRCLIB 返回 HTTP %d", resp.StatusCode)
		}
		var records []lrclibRecord
		if err := json.Unmarshal(body, &records); err != nil {
			return nil, fmt.Errorf("解析 LRCLIB 响应失败: %w", err)
		}
		return records, nil
	}
	if lastErr == nil {
		lastErr = errors.New("LRCLIB 无响应")
	}
	return nil, lastErr
}

func pickLRCLIB(records []lrclibRecord, song alx.Song) (*lrclibRecord, int) {
	title := normalizeMatch(song.Name)
	artist := normalizeMatch(primaryArtist(song.Singer))
	fullArtist := normalizeMatch(song.Singer)
	album := normalizeMatch(song.AlbumName)
	duration := song.Duration().Seconds()
	bestScore := -1
	var best *lrclibRecord
	for i := range records {
		rec := &records[i]
		if rec.Instrumental {
			continue
		}
		if strings.TrimSpace(rec.SyncedLyrics) == "" && strings.TrimSpace(rec.PlainLyrics) == "" {
			continue
		}
		recTitle := normalizeMatch(rec.TrackName)
		if recTitle == "" {
			continue
		}
		score := 0
		switch {
		case title != "" && recTitle == title:
			score += 100
		case title != "" && (strings.Contains(recTitle, title) || strings.Contains(title, recTitle)):
			score += 55
		default:
			continue
		}
		recArtist := normalizeMatch(rec.ArtistName)
		switch {
		case artist != "" && recArtist == artist:
			score += 70
		case fullArtist != "" && recArtist == fullArtist:
			score += 70
		case artist != "" && (strings.Contains(recArtist, artist) || strings.Contains(artist, recArtist)):
			score += 35
		}
		if album != "" && normalizeMatch(rec.AlbumName) == album {
			score += 15
		}
		if duration > 0 && rec.Duration > 0 {
			diff := math.Abs(rec.Duration - duration)
			switch {
			case diff <= 3:
				score += 25
			case diff <= 10:
				score += 15
			case diff <= 30:
				score += 5
			}
		}
		if strings.TrimSpace(rec.SyncedLyrics) != "" {
			score += 10
		}
		if score > bestScore {
			bestScore = score
			best = rec
		}
	}
	return best, bestScore
}
