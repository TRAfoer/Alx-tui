package alx

import (
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Lyrics is a parsed lyric document.
type Lyrics struct {
	Lines  []LyricLine
	Synced bool
}

var (
	lrcTimeRe   = regexp.MustCompile(`\[(\d{1,3}):(\d{1,2})(?:[.:](\d{1,3}))?\]`)
	lrcOffsetRe = regexp.MustCompile(`(?i)\[offset:\s*([+-]?\d+)\s*\]`)
)

// ParseLRC parses an LRC document. When no timestamps are present it falls
// back to unsynced plain-text lyrics.
func ParseLRC(doc string) Lyrics {
	doc = strings.TrimPrefix(doc, "\ufeff")
	doc = strings.ReplaceAll(doc, "\r\n", "\n")
	doc = strings.ReplaceAll(doc, "\r", "\n")

	offset := time.Duration(0)
	if m := lrcOffsetRe.FindStringSubmatch(doc); len(m) == 2 {
		if ms, err := strconv.Atoi(m[1]); err == nil {
			offset = time.Duration(ms) * time.Millisecond
		}
	}

	var timed []LyricLine
	plain := make([]string, 0, 8)
	for _, raw := range strings.Split(doc, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		matches := lrcTimeRe.FindAllStringSubmatchIndex(line, -1)
		if len(matches) == 0 {
			// Metadata tags such as [ti:], [ar:], [al:], [by:] are ignored.
			if strings.HasPrefix(line, "[") && strings.Contains(line, ":") {
				if idx := strings.Index(line, "]"); idx > 0 && !strings.Contains(line[:idx], ".") {
					continue
				}
			}
			plain = append(plain, line)
			continue
		}
		text := strings.TrimSpace(line[matches[len(matches)-1][1]:])
		for _, idx := range matches {
			min, err1 := strconv.Atoi(line[idx[2]:idx[3]])
			sec, err2 := strconv.Atoi(line[idx[4]:idx[5]])
			if err1 != nil || err2 != nil {
				continue
			}
			var frac time.Duration
			if idx[6] >= 0 {
				rawFrac := line[idx[6]:idx[7]]
				switch len(rawFrac) {
				case 1:
					rawFrac += "00"
				case 2:
					rawFrac += "0"
				}
				if len(rawFrac) > 3 {
					rawFrac = rawFrac[:3]
				}
				ms, err := strconv.Atoi(rawFrac)
				if err == nil {
					frac = time.Duration(ms) * time.Millisecond
				}
			}
			at := time.Duration(min)*time.Minute + time.Duration(sec)*time.Second + frac + offset
			if at < 0 {
				at = 0
			}
			timed = append(timed, LyricLine{Time: at, Text: text})
		}
	}

	if len(timed) == 0 {
		lines := make([]LyricLine, 0, len(plain))
		for _, text := range plain {
			if text == "" {
				continue
			}
			lines = append(lines, LyricLine{Text: text})
		}
		return Lyrics{Lines: lines, Synced: false}
	}

	sort.SliceStable(timed, func(i, j int) bool { return timed[i].Time < timed[j].Time })
	// Drop exact duplicate timestamp+text pairs (common in LRC files with
	// repeated tags).
	out := timed[:0]
	for _, l := range timed {
		if n := len(out); n > 0 && out[n-1].Time == l.Time && out[n-1].Text == l.Text {
			continue
		}
		out = append(out, l)
	}
	return Lyrics{Lines: out, Synced: true}
}

// IndexAt returns the index of the last lyric line at or before pos. It
// returns -1 before the first line.
func (l Lyrics) IndexAt(pos time.Duration) int {
	if len(l.Lines) == 0 || !l.Synced {
		return -1
	}
	idx := -1
	for i := range l.Lines {
		if l.Lines[i].Time <= pos {
			idx = i
		} else {
			break
		}
	}
	return idx
}
