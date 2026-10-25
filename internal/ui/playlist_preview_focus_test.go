package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPlaylistPreviewRetainsNavigationWhenSearchPageReopens(t *testing.T) {
	for _, leavePage := range []bool{false, true} {
		t.Run(map[bool]string{false: "active search tab", true: "return from another page"}[leavePage], func(t *testing.T) {
			a := playlistSearchApp()
			a.openSearchPlaylist(0)
			if leavePage {
				a.switchPage(PagePlayback)
			}
			a.switchPage(PageSearch)
			if a.searchFocus || a.searchInput.Focused() {
				t.Fatal("preview focused an invisible search input after reopening search page")
			}
			quit := a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
			if quit == nil {
				t.Fatal("preview swallowed global quit shortcut")
			}
			if _, ok := quit().(tea.QuitMsg); !ok {
				t.Fatal("preview did not preserve global quit shortcut")
			}
			a.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
			if a.page != PagePlaylists {
				t.Fatal("preview swallowed global page navigation")
			}
		})
	}
}
