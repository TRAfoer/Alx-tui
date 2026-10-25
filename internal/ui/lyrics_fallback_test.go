package ui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"alxtui/internal/alx"
)

func TestNullLyric(t *testing.T) {
	for _, value := range []string{"", "  ", "null", "NULL", `Null"null"`, "nil"} {
		if !nullLyric(value) {
			t.Errorf("expected %q to be treated as null", value)
		}
	}
	for _, value := range []string{"[00:01.00]hello", "this song has null in the title"} {
		if nullLyric(value) {
			t.Errorf("real lyric %q was rejected", value)
		}
	}
}

func TestPrimaryArtist(t *testing.T) {
	cases := map[string]string{
		"rejection、Such":     "rejection",
		"周杰伦":                "周杰伦",
		"A & B":              "A",
		"Artist feat. Other": "Artist",
	}
	for in, want := range cases {
		if got := primaryArtist(in); got != want {
			t.Errorf("primaryArtist(%q) = %q, want %q", in, got, want)
		}
	}
}

func withLRCLIBServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	oldBase := lrclibBaseURL
	oldClient := lrclibHTTPClient
	lrclibBaseURL = server.URL
	lrclibHTTPClient = server.Client()
	t.Cleanup(func() {
		lrclibBaseURL = oldBase
		lrclibHTTPClient = oldClient
		server.Close()
	})
	return server
}

func TestFetchFallbackLyricsChoosesSyncedMatch(t *testing.T) {
	var calls int
	withLRCLIBServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/api/search" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("track_name"); got != "Signal (feat. Such)" {
			t.Errorf("track_name = %q", got)
		}
		records := []map[string]any{
			{
				"id": 1, "trackName": "Signal (feat. Such)", "artistName": "rejection",
				"albumName": "Other", "duration": 400.0, "instrumental": false,
				"plainLyrics": "wrong lyrics", "syncedLyrics": "",
			},
			{
				"id": 2, "trackName": "Signal (feat. Such)", "artistName": "rejection",
				"albumName": "Beyond core EVANGELIX 01", "duration": 315.0, "instrumental": false,
				"plainLyrics": "", "syncedLyrics": "[00:01.00]one\n[00:02.00]two\n",
			},
		}
		_ = json.NewEncoder(w).Encode(records)
	})
	song := alx.Song{
		CLIID: "x", Name: "Signal (feat. Such)", Singer: "rejection、Such",
		AlbumName: "Beyond core EVANGELIX 01", Interval: "05:15", Source: "wy",
	}
	lyrics, source, err := fetchFallbackLyrics(context.Background(), song)
	if err != nil {
		t.Fatal(err)
	}
	if source != "LRCLIB" {
		t.Fatalf("source = %q", source)
	}
	if !lyrics.Synced || len(lyrics.Lines) != 2 || lyrics.Lines[0].Text != "one" {
		t.Fatalf("lyrics = %+v", lyrics)
	}
	if calls == 0 {
		t.Fatal("LRCLIB was not queried")
	}
}

func TestCmdLoadLyricsFallsBackFromNull(t *testing.T) {
	withLRCLIBServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{
			"id": 9, "trackName": "晴天", "artistName": "周杰伦",
			"albumName": "叶惠美", "duration": 269.0, "instrumental": false,
			"plainLyrics": "", "syncedLyrics": "[00:01.00]故事的小黄花\n",
		}})
	})

	client, logPath := testBackend(t, `{"cli_id":"x","lyric":"Null\"null\"","name":"晴天","singer":"周杰伦"}`, 0)
	song := alx.Song{CLIID: "x", Name: "晴天", Singer: "周杰伦", Interval: "04:29", Source: "wy"}
	msg, ok := cmdLoadLyrics(client, song, 7)().(lyricsMsg)
	if !ok {
		t.Fatal("unexpected message type")
	}
	if msg.err != nil {
		t.Fatal(msg.err)
	}
	if msg.source != "LRCLIB" {
		t.Fatalf("source = %q", msg.source)
	}
	if len(msg.lyrics.Lines) == 0 || !strings.Contains(msg.lyrics.Lines[0].Text, "故事") {
		t.Fatalf("lyrics = %+v", msg.lyrics)
	}
	if got, want := testBackendArgs(t, logPath), []string{"lyric", "x", "--json", "--no-color"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("backend args = %q, want %q", got, want)
	}
}

func TestFetchFallbackLyricsNoMatch(t *testing.T) {
	withLRCLIBServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("[]"))
	})
	_, _, err := fetchFallbackLyrics(context.Background(), alx.Song{Name: "无此歌曲", Singer: "无此歌手"})
	if err == nil {
		t.Fatal("expected a no-match error")
	}
}
