package ui

import (
	"math"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"

	"musger/ansi"
	"musger/playlists"
)

func MstoReadable(ms uint64) string {
	var s strings.Builder
	secs := float64(ms) / 1000
	hours := int(secs / 3600)
	mins := int(math.Mod(secs, 3600) / 60)
	sec := int(math.Mod(math.Mod(math.Mod(secs, 3600), 60), 60))
	if hours > 0 {
		if hours < 10 {
			s.WriteByte('0')
		}
		s.WriteString(strconv.Itoa(hours))
		s.WriteByte(':')
	}
	if mins < 10 {
		s.WriteByte('0')
	}
	s.WriteString(strconv.Itoa(mins))
	s.WriteByte(':')
	if sec < 10 {
		s.WriteByte('0')
	}
	s.WriteString(strconv.Itoa(sec))
	return s.String()
}

func DisplayMenu(c int) string {
	var s strings.Builder
	s.Grow(52)
	if c == 0 {
		s.WriteString("> ")
	} else {
		s.WriteString("  ")
	}
	s.WriteString("Title")
	s.WriteString("\n")
	if c == 1 {
		s.WriteString("> ")
	} else {
		s.WriteString("  ")
	}
	s.WriteString("Artist")
	s.WriteString("\n")
	if c == 2 {
		s.WriteString("> ")
	} else {
		s.WriteString("  ")
	}
	s.WriteString("Album")
	s.WriteString("\n")
	if c == 3 {
		s.WriteString("> ")
	} else {
		s.WriteString("  ")
	}
	s.WriteString("Album Artist")
	s.WriteString("\n")
	if c == 4 {
		s.WriteString("> ")
	} else {
		s.WriteString("  ")
	}
	s.WriteString("Lyric File")
	return s.String()
}

func DisplayDialogue(yes bool, basepath string) string {
	var s strings.Builder
	s.WriteString("Overwrite ")
	s.WriteString(basepath)
	s.WriteByte('?')
	l := StringWidth(basepath)
	s.WriteByte(NEWLINE)
	if yes {
		s.WriteString(ansi.INVERT)
		s.WriteString("<yes>")
		s.WriteString(ansi.RESET)
	} else {
		s.WriteString("<yes>")
	}
	for range l + 2 {
		s.WriteByte(0x20)
	}
	if !yes {
		s.WriteString(ansi.INVERT)
		s.WriteString("<no>")
		s.WriteString(ansi.RESET)
	} else {
		s.WriteString("<no>")
	}
	return s.String()
}

func PathTitle(path string) string {
	base := filepath.Base(path)
	s := strings.FieldsFunc(base, func(r rune) bool { return r == '.' })
	base = strings.Join(s[:len(s)-1], "")
	if len(base) != 0 {
		return base
	}
	return "Unknown title"
}

func ViewSong(
	currSelected bool,
	width int,
	song *playlists.Song,
) string {
	title := "Unknown title"
	artist := ""
	var sep string
	var final strings.Builder
	if len(song.Title) != 0 {
		title = song.Title
	} else if len(song.PathTitle) != 0 {
		title = song.PathTitle
	} else {
		title = PathTitle(song.Path)
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
	var arrow string = "   "
	if currSelected {
		arrow = "-> "
	}
	length := len(title) + len(artist) + len(sep) + 3
	final.Grow(length)
	final.WriteString(arrow)
	final.WriteString(title)
	final.WriteString(sep)
	final.WriteString(artist)
	if final.Len() > width-1 {
		fWidth := StringWidth(final.String())
		if fWidth > width-1 {
			final.Reset()
			final.Grow(length)
			target := StringWidth(title) - (fWidth - (width - 1))
			title = Truncate(title, target, "…")
			final.WriteString(arrow)
			final.WriteString(title)
			final.WriteString(sep)
			final.WriteString(artist)
		}
	}
	return final.String()
}

func CurrStateAsStr(
	width, lookAhead, lookBehind int, p *playlists.Playlist,
) string {
	if len(p.Songs) == 0 {
		return "No songs"
	}
	var final strings.Builder
	lB, lA := calculateLaLb(
		lookAhead,
		lookBehind,
		len(p.Songs),
		p.CurrSong,
	)
	final.Grow((lA + lB + 1) * (width + 1))
	if lB > 0 {
		for s := p.CurrSong - lB; s != p.CurrSong; s++ {
			actualIdx := s
			if p.IsShuffled {
				actualIdx = p.ShuffledSongs[actualIdx]
			}
			final.WriteString(ViewSong(false, width, &p.Songs[actualIdx]))
			final.WriteByte(NEWLINE)
		}
	}
	actualIdx := p.CurrSong
	if p.IsShuffled {
		actualIdx = p.ShuffledSongs[actualIdx]
	}
	final.WriteString(ViewSong(true, width, &p.Songs[actualIdx]))
	final.WriteByte(NEWLINE)
	if lA > 0 {
		for s := p.CurrSong + 1; s != p.CurrSong+lA+1; s++ {
			actualIdx := s
			if p.IsShuffled {
				actualIdx = p.ShuffledSongs[actualIdx]
			}
			final.WriteString(ViewSong(false, width, &p.Songs[actualIdx]))
			final.WriteByte(NEWLINE)
		}
	}
	return final.String()
}

func TruncateTitle(title string, width int) string {
	target := width - 2
	if len(title) >= target {
		actualWidth := StringWidth(title)
		if actualWidth >= target {
			return Truncate(title, target, "…")
		}
	}
	return title
}

func (m *Model) viewMenu(compositor *lg.Compositor) {
	content := DisplayMenu(m.menuPos)
	styledContent := borderStyle(padding(content))
	menu := lg.NewLayer(styledContent).Z(1)
	compositor.AddLayers(
		menu.X(m.width/2 - menu.Width()/2).Y(m.height/2 - menu.Height()/2),
	)
}

func (m *Model) viewText(compositor *lg.Compositor) {
	content := m.ti.View()
	styledContent := borderStyle(padding(content))
	menu := lg.NewLayer(styledContent).Z(1)
	compositor.AddLayers(
		menu.X(m.width/2 - menu.Width()/2).Y(m.height/2 - menu.Height()/2),
	)
}

func (m *Model) viewFP(compositor *lg.Compositor) {
	content := m.filepicker.View()
	content = strings.TrimRight(content, "\n")
	if m.err != nil {
		content += "\n\n"
		content += errStyle(m.err.Error())
	}
	styledContent := borderStyle(padding(content))
	menu := lg.NewLayer(styledContent).Z(1)
	compositor.AddLayers(
		menu.X(m.width/2 - menu.Width()/2).Y(m.height/2 - menu.Height()/2),
	)
}

func (m *Model) viewSave(compositor *lg.Compositor) {
	content := "Saved to: " + path.Base(m.savePath)
	styledContent := borderStyle(padding(content))
	menu := lg.NewLayer(styledContent).Z(1)
	compositor.AddLayers(
		menu.X(m.width/2 - menu.Width()/2).Y(m.height - menu.Height()),
	)
}

func (m *Model) viewDialogue(compositor *lg.Compositor) {
	content := DisplayDialogue(m.overwriteD, filepath.Base(m.overwrite))
	styledContent := borderStyle(padding(content))
	menu := lg.NewLayer(styledContent).Z(1)
	compositor.AddLayers(
		menu.X(m.width/2 - menu.Width()/2).Y(m.height/2 - menu.Height()/2),
	)
}

func (m Model) viewPlayer() tea.View {
	var s strings.Builder
	var progress float64 = 0
	timePos := m.playState.GetTimePos()
	duration := m.playState.durationMs
	timePosReadable := MstoReadable(timePos)
	durationReadable := MstoReadable(duration)
	spaces := strings.Repeat(
		" ",
		m.width-(len(timePosReadable)+len(durationReadable)),
	)
	if m.playState.durationMs != 0 {
		progress = float64(timePos) / float64(duration)
	}
	queue := m.GetQueue()
	if queue == nil {
		return tea.NewView("")
	}
	var play string = "paused"
	var shuffled string
	if !m.playState.pause && m.playState.song != nil &&
		len(m.playState.song.Path) != 0 {
		play = "playing"
	}
	if queue.IsShuffled {
		shuffled = ", Shuffled"
	}
	if m.showQueue {
		shuffled += ":"
	}
	var title, artist string
	title = m.playState.nextTitle
	if m.playState.song != nil {
		title = m.playState.song.Title
		artist = m.playState.song.Artist
	}
	title = TruncateTitle(title, m.width)
	s.WriteString(titleStyle(title))
	s.WriteByte(NEWLINE)
	s.WriteString(artist)
	s.WriteByte(NEWLINE)
	s.WriteString(m.progress.ViewAs(progress))
	s.WriteByte(NEWLINE)
	s.WriteString(timePosReadable)
	s.WriteString(spaces)
	s.WriteString(durationReadable)
	s.WriteByte(NEWLINE)
	s.WriteByte(NEWLINE)
	s.WriteString(strconv.Itoa(len(queue.Songs)))
	s.WriteString(" songs, (")
	s.WriteString(m.loop.loop())
	s.WriteString(", currently ")
	s.WriteString(play)
	s.WriteByte(')')
	s.WriteString(shuffled)
	s.WriteByte(NEWLINE)
	if m.showQueue {
		s.WriteString(m.currentState)
	}
	s.WriteByte(NEWLINE)
	if m.menuPos == NoMenu && !m.saving && !m.saved && !m.overwriting {
		v := tea.NewView(s.String())
		v.AltScreen = true
		return v
	}
	compositor := lg.NewCompositor()
	compositor.AddLayers(lg.NewLayer(s.String()))
	if m.menuPos != NoMenu && m.menuPos < 5 {
		m.viewMenu(compositor)
	} else if m.menuPos >= 5 && m.menuPos != MLyricPath {
		m.viewText(compositor)
	} else if m.menuPos == MLyricPath {
		m.viewFP(compositor)
	}
	if m.saving {
		m.viewText(compositor)
	} else if m.saved {
		m.viewSave(compositor)
	}
	if m.overwriting {
		m.viewDialogue(compositor)
	}
	v := tea.NewView(compositor.Render())
	v.AltScreen = true
	return v
}
