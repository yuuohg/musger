package ui

import (
	"os"
	"path"
	"path/filepath"
	"time"

	"musger/ipc"
	"musger/playlists"

	fp "charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
)

func (m *Model) setupSaveInput() {
	m.ti.Focus()
	if len([]byte(m.savePath)) > 0 {
		m.ti.SetValue(m.savePath)
	} else {
		d, err := os.Getwd()
		if err != nil {
			return
		}
		m.ti.SetValue(d + string(os.PathSeparator))
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

func (m *Model) ChangeSong(prev bool) {
	queue := m.GetQueue()
	if queue == nil {
		return
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
		return
	}
	if queue.IsShuffled {
		m.playState.song = queue.Songs[queue.ShuffledSongs[song]].Load(
			m.client,
		)
	} else {
		m.playState.song = queue.Songs[song].Load(m.client)
	}
	m.count++
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
				m.ChangeSong(false)
				m.updateCurrState()
			}
		case RepeatOnce:
			{
				isAtEnd := queue.CurrSong == len(queue.Songs)-1
				if isAtEnd {
					break
				}
				m.ChangeSong(false)
				m.updateCurrState()
			}
		}
	} else if msg.Event == "start-file" && msg.PlaylistEntryID == m.count {
		m.playState.greenlit = true
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
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

func (m *Model) handleEnter() {
	queue := m.GetQueue()
	if m.menuPos != NoMenu && m.menuPos < 5 {
		m.menuPos += 5
		if m.menuPos < MLyricPath {
			m.ti.Focus()
			m.setupInput()
		} else if m.menuPos == MLyricPath {
			m.setupFilepicker()
		}
		return
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
		return
	}
	m.client.SendCommand(ipc.TogglePlay(m.playState.pause))
}

func (m *Model) handleSaveInput(msg tea.Msg) tea.Cmd {
	var cmd []tea.Cmd
	c := true
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			switch msg.Keystroke() {
			case "enter":
				{
					filePath := m.ti.Value()
					basePath := path.Base(filePath)
					dirPath := path.Dir(filePath)
					if basePath == "/" {
						m.ti.Blur()
						m.saving = false
						c = false
						break
					}
					if len(m.savePath) != 0 && filePath == m.savePath {
						m.AsMUGR().Save(m.savePath)
						m.saving = false
						m.saved = true
						c = false
						break
					}
					stats, err := os.Stat(filePath)
					if err == nil && stats.Size() != 0 {
						m.overwrite = filePath
						m.overwriting = true
						m.saving = false
						m.saved = false
						m.ti.Blur()
						c = false
						break
					}
					os.MkdirAll(dirPath, os.ModeDir)
					m.ti.Blur()
					m.savePath = filePath
					m.AsMUGR().Save(m.savePath)
					m.saving = false
					m.saved = true
					cmd = append(cmd, savedCmd())
				}
			case "esc", "escape":
				{
					m.ti.Blur()
					m.saving = false
					c = false
				}
			}
		}
	}
	if c {
		var cm tea.Cmd
		m.ti, cm = m.ti.Update(msg)
		cmd = append(cmd, cm)
	}
	m.updateCurrState()
	return tea.Batch(cmd...)
}

func (m *Model) handleDialogue(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			switch msg.Keystroke() {
			case "left":
				{
					m.overwriteD = true
				}
			case "right":
				{
					m.overwriteD = false
				}
			case "up", "down":
				{
					m.overwriteD = !m.overwriteD
				}
			case "enter":
				{
					if m.overwriteD {
						filePath := m.overwrite
						dirPath := path.Dir(filePath)
						os.MkdirAll(dirPath, os.ModeDir)
						m.savePath = filePath
						m.AsMUGR().Save(m.savePath)
						m.saved = true
						cmd = savedCmd()
					}
					m.overwriteD = false
					m.overwriting = false
				}
			case "esc", "escape":
				{
					m.overwriteD = false
					m.overwrite = ""
					m.overwriting = false
				}
			}
		}
	}
	m.updateCurrState()
	return cmd
}

func (m *Model) handleKeybinds(msg tea.KeyPressMsg) {
	queue := m.GetQueue()
	switch msg.Keystroke() {
	case "p", "space", " ", "enter":
		{
			m.handleEnter()
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
			m.ChangeSong(false)
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
			m.ChangeSong(true)
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
			m.loop = (m.loop + 1) % (RepeatOne + 1)
		}
	case "s":
		{
			if !queue.IsShuffled {
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

func (m *Model) updateCurrState() {
	if len(m.playlists) == 0 || m.queue < 0 || m.queue >= len(m.playlists) {
		return
	}
	h := (m.height - 6) / 2
	m.currentState = CurrStateAsStr(m.width, h, h, &m.playlists[m.queue])
}

func (m Model) updatePlayer(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.menuPos >= MTitle && m.menuPos < MLyricPath {
		cmd := m.handleTextInput(msg)
		return m, cmd
	} else if m.menuPos == MLyricPath {
		cmd := m.handleFP(msg)
		return m, cmd
	} else if m.saving {
		if !m.ti.Focused() {
			m.setupSaveInput()
		}
		cmd := m.handleSaveInput(msg)
		return m, cmd
	}
	if m.overwriting {
		cmd := m.handleDialogue(msg)
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			m.handleKeybinds(msg)
		}
	}
	if m.err != nil {
		return m, tea.Batch(errorCmd())
	}
	return m, nil
}
