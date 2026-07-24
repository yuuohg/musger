package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	. "musger/ipc"
	. "musger/playlists"

	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
	trun "github.com/muesli/reflow/truncate"
)

func ViewSong(currSelected, currPlaying bool, width int, song *Song) string {
	title := "Unknown title"
	artist := ""
	var sep string
	if len(song.Title) != 0 {
		title = song.Title
	} else if len(song.Path) != 0 {
		base := filepath.Base(song.Path)
		s := strings.FieldsFunc(base, func(r rune) bool { return r == '.' })
		base = strings.Join(s[:len(s)-1], "")
		if len(base) != 0 {
			title = base
		}
	}
	if len(song.Artist) != 0 {
		artist = song.Artist
		sep = " - "
	} else if len(song.AlbumArtist) != 0 {
		artist = song.AlbumArtist
		sep = " - "
	}
	title = strings.TrimSpace(title)
	artist = strings.TrimSpace(artist)
	var ending string
	if currPlaying {
		ending = " (playing)"
	} else if currSelected {
		ending = " (paused)"
	}
	final := title + sep + artist + ending
	if lg.Width(final) > width-1 {
		target := lg.Width(title) - (lg.Width(final) - (width - 3))
		title = trun.StringWithTail(title, uint(target), "…")
		final = title + sep + artist + ending
	}
	return "- " + final
}

func CurrStateAsStr(
	width, lookAhead, lookBehind int, p *Playlist, ps PlayState,
) string {
	playlistView := make([]Song, 0, len(p.Songs))
	for i := range len(p.ShuffledSongs) {
		if p.IsShuffled {
			playlistView = append(playlistView, p.Songs[p.ShuffledSongs[i]])
		} else {
			playlistView = append(playlistView, p.Songs[i])
		}
	}
	if len(playlistView) == 0 {
		return "No songs"
	}
	var final strings.Builder
	lB, lA := lookBehind, lookAhead
	if p.CurrSong < lookBehind {
		lB = p.CurrSong
		lA = (lookBehind - lB) + lA
	}
	if p.CurrSong >= len(playlistView)-lookAhead {
		lA = len(playlistView) - (p.CurrSong + 1)
		lB = (lookAhead - lA) + lB
	}
	if p.CurrSong == -1 {
		lA, lB = lookAhead+lookBehind+1, 0
		if lA > len(playlistView) {
			lA = len(playlistView)
		}
	}
	if len(playlistView) == 1 {
		lA, lB = 0, 0
	}
	if lB > 0 {
		for s := p.CurrSong - lB; s != p.CurrSong; s++ {
			if s < 0 {
				continue
			}
			fmt.Fprintln(
				&final,
				ViewSong(false, false, width, &playlistView[s]),
			)
		}
	}
	fmt.Fprintln(
		&final,
		ViewSong(
			true,
			!ps.pause && ps.fileName != "",
			width,
			&playlistView[p.CurrSong],
		),
	)
	if lA > 0 {
		for s := p.CurrSong + 1; s != p.CurrSong+lA+1; s++ {
			if s >= len(playlistView) {
				break
			}
			fmt.Fprintln(
				&final,
				ViewSong(false, false, width, &playlistView[s]),
			)
		}
	}
	return final.String()
}

func (m Model) Change(prev bool) Model {
	var song int
	var err error
	m.playState.greenlit = false
	if prev {
		song, err = m.queue.Prev(true)
	} else {
		song, err = m.queue.Next(true)
	}
	if err != nil {
		m.err = err
		return m
	}
	if m.queue.IsShuffled {
		m.playState.song = m.queue.Songs[m.queue.ShuffledSongs[song]].Load(
			m.client,
		)
	} else {
		m.playState.song = m.queue.Songs[song].Load(m.client)
	}
	m.count++
	return m
}

func (m Model) updatePlayer(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			switch msg.String() {
			case "p", "space", " ":
				{
					if m.playState.fileName == "" && len(m.queue.Songs) != 0 {
						m.playState.song = m.queue.Songs[m.queue.CurrSong].Load(m.client)
						break
					}
					m.client.SendCommand(TogglePlay(m.playState.pause))
				}
			case "n", "f", "down":
				{
					m = m.Change(false)
				}
			case "b", "up":
				{
					m = m.Change(true)
				}
			case "h", "left":
				{
					if m.playState.fileName != "" {
						m.client.SendCommand(SeekBackwardFive)
					}
				}
			case "l", "right":
				{
					if m.playState.fileName != "" {
						m.client.SendCommand(SeekForwardFive)
					}
				}
			case "c":
				{
					m.loop++
					m.loop = m.loop % (RepeatOne + 1)
				}
			case "s":
				{
					if !m.queue.IsShuffled {
						m.queue.AllocateShuffle()
						m.queue.ShufflePlaylist()
					} else {
						m.queue.CurrSong = m.queue.ShuffledSongs[m.queue.CurrSong]
						m.queue.IsShuffled = false
					}
				}
			}
		}
	}
	if m.err != nil {
		return m, tea.Batch(waitForMpv(m.msgChan), errorCmd())
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
}

func (m Model) viewPlayer() tea.View {
	var s strings.Builder
	var progress float64 = 0
	timePos := msToReadable(m.playState.GetTimePos())
	duration := msToReadable(m.playState.durationMs)
	if m.playState.durationMs != 0 {
		progress = float64(
			m.playState.GetTimePos(),
		) / float64(
			m.playState.durationMs,
		)
	}
	titleStyle := lg.NewStyle().
		Width(m.width).
		Align(lg.Center).
		Bold(true)
	timePosStyle := lg.NewStyle().
		Width(m.width - len(duration)).
		Align(lg.Left)
	var title, artist string
	title = m.playState.nextTitle
	if m.playState.song != nil {
		artist = m.playState.song.Artist
	}
	s.WriteString(titleStyle.Render(title))
	s.WriteString("\n")
	s.WriteString(artist)
	s.WriteString("\n")
	s.WriteString(m.progress.ViewAs(progress))
	fmt.Fprintf(&s, "\n%v%v\n", timePosStyle.Render(timePos), duration)
	lines := strings.Count(s.String(), "\n")
	h := m.height - lines + 5
	s.WriteString("\n")
	fmt.Fprintf(&s, "Queue, (%v): \n", m.loop.loop())
	s.WriteString(CurrStateAsStr(m.width, h/2, h/2, &m.queue, m.playState))
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}
