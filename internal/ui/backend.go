package ui

import (
	"context"
	"sync"
	"sync/atomic"

	"alxtui/internal/alx"
	tea "github.com/charmbracelet/bubbletea"
)

// Commands can outlive both their originating Update and their backend. The
// shared epoch guards execution as well as delivery, including nested batches.
type backendLifecycle struct {
	mu            sync.Mutex
	epoch         atomic.Uint64
	client        *alx.Client
	sessionCancel context.CancelFunc
	closed        bool
}

type backendEnvelope struct {
	epoch uint64
	msg   tea.Msg
}

func (a *App) ensureBackendLifecycle() {
	if a.lifecycle != nil {
		return
	}
	a.lifecycle = &backendLifecycle{}
	a.installBackendSession(a.client)
}

func (a *App) installBackendSession(client *alx.Client) {
	life := a.lifecycle
	life.mu.Lock()
	defer life.mu.Unlock()
	if life.sessionCancel != nil {
		life.sessionCancel()
	}
	if client == nil || life.closed {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	life.sessionCancel = cancel
	a.client = client.WithSession(ctx)
	life.client = a.client
}

func (a *App) backendCommand(cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	a.ensureBackendLifecycle()
	return commandForEpoch(a.lifecycle, a.lifecycle.epoch.Load(), cmd)
}

func commandForEpoch(life *backendLifecycle, epoch uint64, cmd tea.Cmd) tea.Cmd {
	if cmd == nil {
		return nil
	}
	return func() tea.Msg {
		if life.epoch.Load() != epoch {
			return nil
		}
		msg := cmd()
		if life.epoch.Load() != epoch {
			return nil
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			wrapped := make(tea.BatchMsg, len(batch))
			for i, child := range batch {
				wrapped[i] = commandForEpoch(life, epoch, child)
			}
			return wrapped
		}
		if msg == nil {
			return nil
		}
		if _, ok := msg.(backendEnvelope); ok {
			return msg
		}
		// Bubble Tea consumes its own control messages before Update. Keep
		// those intact so Quit, ClearScreen, focus and cursor commands work.
		switch msg.(type) {
		case infoMsg, configMsg, sourcesMsg, playlistsMsg, playlistSongsMsg,
			stateMsg, queueMsg, lyricsMsg, searchMsg, playSongMsg, queuePlayMsg, playerActionMsg,
			playlistPlayMsg, addToPlaylistMsg, removeFromPlaylistMsg, downloadAddMsg,
			queueAddMsg, configSetMsg, addSourceMsg, removeSourceMsg, createPlaylistMsg,
			importPlaylistMsg, renamePlaylistMsg, deletePlaylistMsg,
			clockMsg, pollMsg, themeSavedMsg:
			return backendEnvelope{epoch: epoch, msg: msg}
		default:
			return msg
		}
	}
}
