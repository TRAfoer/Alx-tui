package alx

import (
	"context"
	"os"
	"testing"
)

func TestPlaylistSearchAndInspect(t *testing.T) {
	c, log, _ := newFakeClient(t, "playlist-response")
	c.Env = append(c.Env, `FAKE_RESPONSE={"playlists":[{"id":"p1","name":"夜行","description":"晚间","source":"wy","link":"https://music.163.com/playlist?id=1","author":"DJ","pic_url":"cover","count":12,"play_count":5000000000}],"total":40,"page":2,"limit":10,"has_more":true,"warnings":["tx unavailable"]}`)
	got, err := c.SearchPlaylists(context.Background(), " 夜行 ", "all", 2, 10)
	if err != nil || len(got.Playlists) != 1 || got.Playlists[0].PlayCount != 5000000000 || !got.HasMore || got.Total != 40 || len(got.Warnings) != 1 {
		t.Fatalf("search = %+v, %v", got, err)
	}
	c.Env = append(c.Env[:len(c.Env)-1], `FAKE_RESPONSE={"id":"p1","name":"夜行","description":"晚间","source":"wy","link":"https://music.163.com/playlist?id=1","songs":[{"cli_id":"wy:1","name":"Song"}],"total":1,"warnings":["skipped"]}`)
	preview, err := c.InspectPlaylist(context.Background(), " https://music.163.com/playlist?id=1 ")
	if err != nil || preview.Name != "夜行" || len(preview.Songs) != 1 || preview.Songs[0].CLIID != "wy:1" || preview.Total != 1 {
		t.Fatalf("inspect = %+v, %v", preview, err)
	}
	assertCalls(t, log, [][]string{{"playlist", "search", "夜行", "--source", "all", "--page", "2", "--limit", "10", "--json", "--no-color"}, {"playlist", "inspect", "https://music.163.com/playlist?id=1", "--json", "--no-color"}})
}

func TestPlaylistSearchInvalidInput(t *testing.T) {
	c, log, _ := newFakeClient(t, "playlist-response")
	for _, tc := range []struct {
		q, s string
		p, l int
	}{{" ", "wy", 1, 20}, {"x", "kg", 1, 20}, {"x", "wy", 1000001, 20}, {"x", "wy", 1, 101}} {
		if _, err := c.SearchPlaylists(context.Background(), tc.q, tc.s, tc.p, tc.l); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if _, err := c.InspectPlaylist(context.Background(), " "); err == nil {
		t.Fatal("accepted empty link")
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("invalid input invoked process")
	}
}

func TestPlaylistSearchDefaultsAndErrors(t *testing.T) {
	c, log, _ := newFakeClient(t, "playlist-response")
	c.Env = append(c.Env, `FAKE_RESPONSE={"playlists":[],"total":0,"page":1,"limit":20,"has_more":false}`)
	out, err := c.SearchPlaylists(context.Background(), "q", "", 0, 0)
	if err != nil || out.Playlists == nil || len(out.Playlists) != 0 || out.HasMore {
		t.Fatalf("empty result = %+v, %v", out, err)
	}
	assertCalls(t, log, [][]string{{"playlist", "search", "q", "--source", "all", "--page", "1", "--limit", "20", "--json", "--no-color"}})
	for _, payload := range []string{`{"error":"catalog failed"}`, `invalid JSON`} {
		c, _, _ = newFakeClient(t, "playlist-response")
		c.Env = append(c.Env, "FAKE_RESPONSE="+payload)
		if _, err = c.SearchPlaylists(context.Background(), "q", "wy", 1, 20); err == nil {
			t.Fatalf("search accepted %q", payload)
		}
		if _, err = c.InspectPlaylist(context.Background(), "https://music.163.com/playlist?id=1"); err == nil {
			t.Fatalf("inspect accepted %q", payload)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = c.InspectPlaylist(ctx, "https://music.163.com/playlist?id=1"); err == nil {
		t.Fatal("inspect ignored cancellation")
	}
}
