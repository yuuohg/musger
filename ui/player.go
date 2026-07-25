package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "musger/ipc"
	. "musger/playlists"

	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
	trun "github.com/muesli/reflow/truncate"
)

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

// function below is ai-generated
func calculateLaLb(
	lookAhead, lookBehind, arrayLen, currentIdx int,
) (lB, lA int) {
	if arrayLen <= 1 {
		return 0, 0
	}
	maxIdx := arrayLen - 1
	availableBehind := currentIdx
	availableAhead := maxIdx - currentIdx
	lB = min(lookBehind, availableBehind)
	lA = min(lookAhead, availableAhead)
	unusedBehind := lookBehind - lB
	if unusedBehind > 0 {
		lA = min(availableAhead, lA+unusedBehind)
	}
	unusedAhead := lookAhead - lA
	if unusedAhead > 0 {
		lB = min(availableBehind, lB+unusedAhead)
	}
	return lB, lA
}

func handlePropertyChange(event MpvMsg, playState *PlayState) {
	var ok bool
	switch event.Name {
	case "pause":
		{
			playState.pause, ok = event.Data.(bool)
			if !ok {
				playState.pause = true
				return
			}
		}
	case "path":
		{
			playState.fileName, ok = event.Data.(string)
			if !ok {
				playState.fileName = ""
			}
		}
	case "media-title":
		{
			if !playState.fromHash {
				nextTitle, ok := event.Data.(string)
				if !ok || nextTitle == "" {
					playState.nextTitle = ""
					break
				}
				base := filepath.Base(playState.song.Path)
				if nextTitle == base {
					s := strings.FieldsFunc(
						base,
						func(r rune) bool { return r == '.' },
					)
					base = strings.Join(s[:len(s)-1], "")
					playState.nextTitle = base
				} else {
					playState.nextTitle = nextTitle
				}
			}
		}
	case "duration/full":
		{
			durationSecs, ok := event.Data.(float64)
			if !ok {
				playState.durationMs = 0
				return
			}
			playState.durationMs = secsToms(durationSecs)
		}
	case "time-pos/full":
		{
			timePos, ok := event.Data.(float64)
			if !ok {
				playState.timePosMs = 0
				playState.lastTimePosCheck = time.Time{}
				return
			}
			playState.timePosMs = secsToms(timePos)
			playState.lastTimePosCheck = time.Now()
		}
	}
}

func (m Model) handleMpvMsg(msg MpvMsg) (tea.Model, tea.Cmd) {
	if msg.Event == "property-change" {
		handlePropertyChange(msg, &m.playState)
		if m.playState.greenlit && !m.playState.song.FromHash {
			m.playState.song.Title = m.playState.nextTitle
		}
	} else if msg.Event == "end-file" && msg.Reason == "eof" {
		switch m.loop {
		case RepeatOne:
			{
				m.count++
				m.playState.song = m.queue.Songs[m.queue.CurrSong].Load(m.client)
			}
		case RepeatAll:
			{
				m = m.Change(false)
			}
		case RepeatOnce:
			{
				isAtEnd := m.queue.CurrSong == len(m.queue.Songs)-1
				if isAtEnd {
					break
				}
				m = m.Change(false)
			}
		}
	} else if msg.Event == "start-file" && msg.PlaylistEntryID == m.count {
		m.playState.greenlit = true
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
}

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
	lB, lA := calculateLaLb(
		lookAhead,
		lookBehind,
		len(playlistView),
		p.CurrSong,
	)
	if lB > 0 {
		for s := p.CurrSong - lB; s != p.CurrSong; s++ {
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
			fmt.Fprintln(
				&final,
				ViewSong(false, false, width, &playlistView[s]),
			)
		}
	}
	return final.String()
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
	s.WriteString("\n")
	fmt.Fprintf(&s, "Queue, (%v): \n", m.loop.loop())
	lines := strings.Count(s.String(), "\n")
	h := m.height - lines
	s.WriteString(CurrStateAsStr(m.width, h/2, h/2, &m.queue, m.playState))
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}
