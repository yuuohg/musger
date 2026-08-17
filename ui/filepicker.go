package ui

import (
	"fmt"
	"os"
	"strings"

	"musger/playlists"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) updatePickingMain(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	*m.filepicker, cmd = m.filepicker.Update(msg)
	hasSelected, selection := m.filepicker.DidSelectFile(msg)
	if !hasSelected {
		return cmd
	}
	var selectionF os.FileInfo
	selectionF, m.err = os.Stat(selection)
	if m.err != nil {
		m.err = fmt.Errorf("selection: '%v', %w", selection, m.err)
		return tea.Batch(cmd, errorCmd())
	}
	if selectionF.IsDir() {
		var playlist playlists.Playlist
		playlist, m.err = playlists.NewAD(selection)
		if m.err != nil {
			return tea.Batch(cmd, errorCmd())
		}
		if len(playlist.Songs) == 0 {
			m.err = fmt.Errorf(
				"No audio files found in '%v' directory",
				selection,
			)
			return tea.Batch(cmd, errorCmd())
		}
		m.playlists = append(m.playlists, playlist)
		m.queue = 0
	} else {
		mugr := playlists.MUGRFile{}
		m.err = mugr.Load(selection)
		if m.err != nil {
			return tea.Batch(cmd, errorCmd())
		}
		m.err = m.Load(mugr)
		if m.err != nil {
			return tea.Batch(cmd, errorCmd())
		}
		m.savePath = selection
		var idx int = m.queue
		cmd = tea.Batch(hashSong(&m.playlists[idx], 0, idx), cmd)
	}
	m.screen = Player
	m.updateCurrState()
	return cmd
}

func (m *Model) viewPickingMain() tea.View {
	var s strings.Builder
	s.WriteString("Pick a directory with music or a valid json file\n\n")
	s.WriteString(m.filepicker.View())
	if m.err != nil {
		s.WriteString("\n\n")
		s.WriteString(errStyle(m.err.Error()))
	}
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}
