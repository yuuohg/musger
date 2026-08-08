package ui

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"musger/ipc"
	"musger/playlists"

	fp "charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
)

type Metadata int

const NEWLINE byte = 10

const (
	Title Metadata = iota
	Artist
	Album
	AlbumArtist
	LyricPath
)

const (
	NoMenu       int = -1
	MTitle       int = 5
	MArtist      int = 6
	MAlbum       int = 7
	MAlbumArtist int = 8
	MLyricPath   int = 9
)

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

func (m Model) Change(prev bool) Model {
	queue := m.GetQueue()
	if queue == nil {
		return m
	}
	var song int
	var err error
	m.playState.greenlit = false
	if prev {
		song, err = queue.Prev(true)
	} else {
		song, err = queue.Next(true)
	}
	if err != nil {
		m.err = err
		return m
	}
	if queue.IsShuffled {
		m.playState.song = queue.Songs[queue.ShuffledSongs[song]].Load(
			m.client,
		)
	} else {
		m.playState.song = queue.Songs[song].Load(m.client)
	}
	m.count++
	return m
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
					playState.nextTitle = playState.song.PathTitle
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
			if playState.song != nil {
				playState.song.Duration = secsToms(durationSecs)
			}
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
			m.updateCurrState()
		}
	} else if msg.Event == "end-file" && msg.Reason == "eof" {
		queue := m.GetQueue()
		if queue == nil {
			return m, tea.Batch(waitForMpv(m.msgChan))
		}
		switch m.loop {
		case RepeatOne:
			{
				m.count++
				m.playState.song = queue.Songs[queue.CurrSong].Load(m.client)
				m.updateCurrState()
			}
		case RepeatAll:
			{
				m = m.Change(false)
				m.updateCurrState()
			}
		case RepeatOnce:
			{
				isAtEnd := queue.CurrSong == len(queue.Songs)-1
				if isAtEnd {
					break
				}
				m = m.Change(false)
				m.updateCurrState()
			}
		}
	} else if msg.Event == "start-file" && msg.PlaylistEntryID == m.count {
		m.playState.greenlit = true
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
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

func (m *Model) updateTags(meta Metadata, hash string, data string) {
	switch meta {
	case Title:
		{
			currMetadata, ok := m.tags[hash]
			if ok {
				currMetadata.Title = data
				m.tags[hash] = currMetadata
			} else {
				m.tags[hash] = playlists.SongMetadata{Title: data}
			}
		}
	case Artist:
		{
			currMetadata, ok := m.tags[hash]
			if ok {
				currMetadata.Artist = data
				m.tags[hash] = currMetadata
			} else {
				m.tags[hash] = playlists.SongMetadata{Artist: data}
			}

		}
	case Album:
		{
			currMetadata, ok := m.tags[hash]
			if ok {
				currMetadata.Album = data
				m.tags[hash] = currMetadata
			} else {
				m.tags[hash] = playlists.SongMetadata{Album: data}
			}
		}
	case AlbumArtist:
		{
			currMetadata, ok := m.tags[hash]
			if ok {
				currMetadata.AlbumArtist = data
				m.tags[hash] = currMetadata
			} else {
				m.tags[hash] = playlists.SongMetadata{AlbumArtist: data}
			}
		}
	case LyricPath:
		{
			currMetadata, ok := m.tags[hash]
			if ok {
				currMetadata.LyricPath = data
				m.tags[hash] = currMetadata
			} else {
				m.tags[hash] = playlists.SongMetadata{LyricPath: data}
			}
		}
	}
}

func (m *Model) handleTextInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			switch msg.Keystroke() {
			case "enter":
				{
					if m.menuSong != nil {
						switch m.menuPos {
						case MTitle:
							{
								m.menuSong.Title = m.ti.Value()
								m.menuSong.FromHash = true
								hash := m.menuSong.Hash
								if len(hash) == 0 {
									m.menuSong.HashAudio()
								}
								m.updateTags(Title, m.menuSong.Hash, m.ti.Value())
							}
						case MArtist:
							{
								m.menuSong.Artist = m.ti.Value()
								hash := m.menuSong.Hash
								if len(hash) == 0 {
									m.menuSong.HashAudio()
								}
								m.updateTags(Artist, m.menuSong.Hash, m.ti.Value())
							}
						case MAlbum:
							{
								m.menuSong.Album = m.ti.Value()
								hash := m.menuSong.Hash
								if len(hash) == 0 {
									m.menuSong.HashAudio()
								}
								m.updateTags(Album, m.menuSong.Hash, m.ti.Value())
							}
						case MAlbumArtist:
							{
								m.menuSong.AlbumArtist = m.ti.Value()
								hash := m.menuSong.Hash
								if len(hash) == 0 {
									m.menuSong.HashAudio()
								}
								m.updateTags(AlbumArtist, m.menuSong.Hash, m.ti.Value())
							}
						}
					}
					m.ti.Blur()
					m.menuSong = nil
					m.menuPos = NoMenu
				}
			case "esc", "escape":
				{
					m.ti.Blur()
					m.menuSong = nil
					m.menuPos = NoMenu
				}
			}
		}
	}
	m.ti, cmd = m.ti.Update(msg)
	m.updateCurrState()
	return cmd
}

func (m *Model) handleKeybinds(msg tea.KeyPressMsg) {
	queue := m.GetQueue()
	switch msg.Keystroke() {
	case "p", "space", " ", "enter":
		{
			if m.menuPos != NoMenu && m.menuPos < 5 {
				m.menuPos += 5
				if m.menuPos < MLyricPath {
					m.ti.Focus()
					m.setupInput()
				} else if m.menuPos == MLyricPath {
					m.setupFilepicker()
				}
				break
			}
			if m.client.PulseaudioIsDead() {
				ipc.KillPulse()
				ipc.StartPulse(m.client.PulsePath)
			}
			if m.playState.fileName == "" &&
				len(queue.Songs) != 0 {
				m.playState.song = queue.Songs[queue.CurrSong].Load(
					m.client,
				)
				m.count++
				break
			}
			m.client.SendCommand(ipc.TogglePlay(m.playState.pause))
		}
	case "esc", "escape":
		{
			if m.menuPos != NoMenu {
				m.menuPos = NoMenu
				m.menuSong = nil
			}
		}
	case "n", "f", "down":
		{
			if m.menuPos != NoMenu {
				m.menuPos++
				m.menuPos %= 5
				break
			}
			*m = m.Change(false)
			m.updateCurrState()
		}
	case "b", "up":
		{
			if m.menuPos != NoMenu {
				m.menuPos--
				if m.menuPos < 0 {
					m.menuPos = 4
				}
				break
			}
			*m = m.Change(true)
			m.updateCurrState()
		}
	case "h", "left":
		{
			if m.playState.fileName != "" {
				m.client.SendCommand(ipc.SeekBackwardFive)
			}
		}
	case "l", "right":
		{
			if m.playState.fileName != "" {
				m.client.SendCommand(ipc.SeekForwardFive)
			}
		}
	case "c":
		{
			m.loop++
			m.loop = m.loop % (RepeatOne + 1)
		}
	case "s":
		{
			if !queue.IsShuffled {
				queue.AllocateShuffle()
				queue.ShufflePlaylist()
				m.updateCurrState()
			} else {
				queue.CurrSong = queue.ShuffledSongs[queue.CurrSong]
				queue.IsShuffled = false
				m.updateCurrState()
			}
		}
	case "z":
		{
			m.showQueue = !m.showQueue
		}
	case "i", "e":
		{
			if len(queue.Songs) == 0 || m.menuPos != NoMenu {
				break
			}
			if queue.IsShuffled {
				m.menuSong = &queue.Songs[queue.ShuffledSongs[queue.CurrSong]]
			} else {
				m.menuSong = &queue.Songs[queue.CurrSong]
			}
			m.menuPos = 0
		}
	}
}

func (m *Model) handleFP(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			if msg.Code == tea.KeyEsc {

				m.menuPos = NoMenu
				m.menuSong = nil
			}
		}
	}
	m.filepicker, cmd = m.filepicker.Update(msg)
	hasSelected, selection := m.filepicker.DidSelectFile(msg)
	if !hasSelected {
		return cmd
	}
	_, m.err = os.Stat(selection)
	if m.err != nil {
		return tea.Batch(cmd, errorCmd())
	}
	_, m.err = playlists.ReadUTF8File(selection)
	if m.err != nil {
		return tea.Batch(cmd, errorCmd())
	}
	m.menuSong.Lyricpath = selection
	hash := m.menuSong.Hash
	if len(hash) == 0 {
		m.menuSong.HashAudio()
	}
	m.updateTags(LyricPath, m.menuSong.Hash, selection)
	m.menuPos = NoMenu
	m.menuSong = nil
	return nil
}

func (m Model) updatePlayer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.menuPos >= MTitle && m.menuPos < MLyricPath {
		cmd := m.handleTextInput(msg)
		return m, cmd
	} else if m.menuPos == MLyricPath {
		cmd := m.handleFP(msg)
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			m.handleKeybinds(msg)
		}
	}
	if m.err != nil {
		return m, tea.Batch(waitForMpv(m.msgChan), errorCmd())
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
}

func (m *Model) setupInput() {
	switch m.menuPos {
	case MTitle:
		{
			m.ti.Reset()
			m.ti.SetValue(m.menuSong.Title)
		}
	case MArtist:
		{
			m.ti.Reset()
			m.ti.SetValue(m.menuSong.Artist)
		}
	case MAlbum:
		{
			m.ti.Reset()
			m.ti.SetValue(m.menuSong.Album)
		}
	case MAlbumArtist:
		{
			m.ti.Reset()
			m.ti.SetValue(m.menuSong.AlbumArtist)
		}
	}
}

func (m *Model) setupFilepicker() {
	m.filepicker.DirAllowed = false
	m.filepicker.FileAllowed = true
	m.filepicker.ShowHidden = true
	m.filepicker.CurrentDirectory, _ = os.Getwd()
	m.filepicker.KeyMap = fp.DefaultKeyMap()
	m.filepicker.Styles = fp.DefaultStyles()
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

func (m *Model) updateCurrState() {
	if len(m.playlists) == 0 || m.queue < 0 || m.queue >= len(m.playlists) {
		return
	}
	h := (m.height - 6) / 2
	m.currentState = CurrStateAsStr(m.width, h, h, &m.playlists[m.queue])
}

func (m Model) viewPlayer() tea.View {
	var s strings.Builder
	var progress float64 = 0
	timePos := MstoReadable(m.playState.GetTimePos())
	duration := MstoReadable(m.playState.durationMs)
	spaces := strings.Repeat(" ", m.width-(len(timePos)+len(duration)))
	if m.playState.durationMs != 0 {
		progress = float64(
			m.playState.GetTimePos(),
		) / float64(
			m.playState.durationMs,
		)
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
	s.WriteString(timePos)
	s.WriteString(spaces)
	s.WriteString(duration)
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
	if m.menuPos == NoMenu {
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
	v := tea.NewView(compositor.Render())
	v.AltScreen = true
	return v
}
