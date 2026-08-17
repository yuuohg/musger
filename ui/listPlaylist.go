package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *Model) updatePlaylists() tea.Cmd {
	return nil
}

func (m *Model) viewPlaylists() tea.View {
	var s strings.Builder
	s.WriteString(Center("Playlists", m.width))
	s.WriteByte(NEWLINE)
	if len(m.playlists) != 0 {
		for _, playlist := range m.playlists {
			s.WriteString(playlist.Name)
			s.WriteString(" - ")
			s.WriteString(strconv.Itoa(len(playlist.Songs)))
			s.WriteByte(NEWLINE)
		}
	} else {
		for range (m.height / 2) - 1 {
			s.WriteByte(NEWLINE)
		}
		for range (m.width / 2) - 12 {
			s.WriteByte(0x20)
		}
		s.WriteString("No playlists")
	}
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}
