package alx

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

func (c *Client) SearchPlaylists(ctx context.Context, keyword, source string, page, limit int) (PlaylistSearchResult, error) {
	var out PlaylistSearchResult
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return out, errors.New("搜索关键词不能为空")
	}
	if source == "" {
		source = "all"
	}
	if source != "all" && source != "wy" && source != "tx" {
		return out, errors.New("歌单仅支持 all / wy / tx")
	}
	if page < 1 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if page > 1000000 || limit > 100 {
		return out, errors.New("搜索页码或数量超出范围")
	}
	err := c.callJSON(ctx, searchTimeout, &out, "playlist", "search", keyword, "--source", source, "--page", strconv.Itoa(page), "--limit", strconv.Itoa(limit), "--json", "--no-color")
	return out, err
}

func (c *Client) InspectPlaylist(ctx context.Context, link string) (ImportedPlaylist, error) {
	var out ImportedPlaylist
	link = strings.TrimSpace(link)
	if link == "" {
		return out, errors.New("歌单链接不能为空")
	}
	err := c.callJSON(ctx, sourceTimeout, &out, "playlist", "inspect", link, "--json", "--no-color")
	return out, err
}
