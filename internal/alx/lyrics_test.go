package alx

import (
	"testing"
	"time"
)

func TestParseLRCTimed(t *testing.T) {
	doc := "[ti:测试]\r\n[ar:歌手]\r\n[offset:500]\r\n[00:01.00]第一行\r\n" +
		"[00:02.50][00:03.250]重复行\r\n[00:04]无小数\r\n"
	ly := ParseLRC(doc)
	if !ly.Synced {
		t.Fatal("expected synced lyrics")
	}
	wantTimes := []time.Duration{
		1500 * time.Millisecond,
		3000 * time.Millisecond,
		3750 * time.Millisecond,
		4500 * time.Millisecond,
	}
	if len(ly.Lines) != len(wantTimes) {
		t.Fatalf("got %d lines, want %d: %+v", len(ly.Lines), len(wantTimes), ly.Lines)
	}
	for i, want := range wantTimes {
		if ly.Lines[i].Time != want {
			t.Errorf("line %d time = %v, want %v", i, ly.Lines[i].Time, want)
		}
	}
	if ly.Lines[1].Text != "重复行" || ly.Lines[2].Text != "重复行" {
		t.Errorf("multiple timestamp text mismatch: %+v", ly.Lines)
	}
}

func TestParseLRCPlainText(t *testing.T) {
	ly := ParseLRC("hello\n\n世界\n")
	if ly.Synced {
		t.Fatal("plain text should not be marked as synced")
	}
	if len(ly.Lines) != 2 || ly.Lines[1].Text != "世界" {
		t.Fatalf("unexpected lines: %+v", ly.Lines)
	}
}

func TestParseLRCEmpty(t *testing.T) {
	ly := ParseLRC("")
	if ly.Synced || len(ly.Lines) != 0 {
		t.Fatalf("expected empty lyrics, got %+v", ly)
	}
}

func TestLyricsIndexAt(t *testing.T) {
	ly := ParseLRC("[00:01.00]a\n[00:05.00]b\n[00:10.00]c\n")
	cases := []struct {
		at   time.Duration
		want int
	}{
		{0, -1},
		{time.Second, 0},
		{4 * time.Second, 0},
		{5 * time.Second, 1},
		{30 * time.Second, 2},
	}
	for _, tc := range cases {
		if got := ly.IndexAt(tc.at); got != tc.want {
			t.Errorf("IndexAt(%v) = %d, want %d", tc.at, got, tc.want)
		}
	}
}

func TestIntervals(t *testing.T) {
	cases := map[string]time.Duration{
		"03:46":   226 * time.Second,
		"1:02:03": time.Hour + 2*time.Minute + 3*time.Second,
		"":        0,
		"99":      99 * time.Second,
		"bad":     0,
	}
	for in, want := range cases {
		if got := ParseInterval(in); got != want {
			t.Errorf("ParseInterval(%q) = %v, want %v", in, got, want)
		}
	}
	if got := FormatSeconds(226); got != "03:46" {
		t.Errorf("FormatSeconds(226) = %q", got)
	}
	if got := FormatSeconds(3723); got != "1:02:03" {
		t.Errorf("FormatSeconds(3723) = %q", got)
	}
}
