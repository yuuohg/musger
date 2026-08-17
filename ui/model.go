package ui

import (
	"os"
	"time"

	"musger/ansi"
	"musger/ipc"
	"musger/playlists"

	fp "charm.land/bubbles/v2/filepicker"
	txt "charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
)

var (
	border      lg.Border = lg.RoundedBorder()
	borderStyle           = lg.NewStyle().BorderStyle(border).Render
	padding               = lg.NewStyle().Padding(0, 1, 0, 1).Render
	errStyle              = lg.NewStyle().Bold(true).Foreground(lg.Red).Render
	noTime                = time.Time{}
)

type Metadata byte

const NEWLINE byte = 10

const (
	Title Metadata = iota
	Artist
	Album
	AlbumArtist
	LyricPath
)

const (
	NoMenu       byte = 0
	MTitle       byte = 6
	MArtist      byte = 7
	MAlbum       byte = 8
	MAlbumArtist byte = 9
	MLyricPath   byte = 10
)

type Screen byte

const (
	Player Screen = iota
	PickingMain
	PickingFolder
	Lyrics
	Songs
	ListPlaylist
)

type Loop byte

const (
	RepeatOnce Loop = iota
	RepeatAll
	RepeatOne
)

type (
	MpvMsg        ipc.MpvResponse
	ClearErrMsg   struct{}
	ClearSavedMsg struct{}
	hashMsg       struct {
		hash        string
		idx         int
		playlisyIdx int
	}
)

type PlayState struct {
	lastTimePosCheck time.Time
	song             *playlists.Song
	fileName         string
	nextTitle        string
	durationMs       uint64
	timePosMs        uint64
	pause            bool
	fromHash         bool
	greenlit         bool
}

type Model struct {
	err          error
	playState    *PlayState
	progress     *ProgressBarOptions
	tags         map[string]playlists.SongMetadata
	menuSong     *playlists.Song
	msgChan      chan ipc.MpvResponse
	client       *ipc.MpvClient
	filepicker   *fp.Model
	ti           *txt.Model
	overwrite    string
	currentState string
	savePath     string
	playlists    []playlists.Playlist
	count        uint64
	width        int
	height       int
	queue        int
	menuPos      byte
	loop         Loop
	saved        bool
	saving       bool
	showQueue    bool
	overwriteD   bool
	screen       Screen
}

func waitForMpv(msgChan chan ipc.MpvResponse) tea.Cmd {
	return func() tea.Msg {
		return MpvMsg(<-msgChan)
	}
}

func errorCmd() tea.Cmd {
	return tea.Tick(
		time.Second*2,
		func(_ time.Time) tea.Msg { return ClearErrMsg{} },
	)
}

func savedCmd() tea.Cmd {
	return tea.Tick(
		time.Second*2,
		func(_ time.Time) tea.Msg { return ClearSavedMsg{} },
	)
}

func (l Loop) loop() string {
	switch l {
	case RepeatOnce:
		{
			return "Repeat Once"
		}
	case RepeatAll:
		{
			return "Repeat All"
		}
	case RepeatOne:
		{
			return "Repeat One"
		}
	}
	return "Unknown Loop"
}

func hashSong(
	playlist *playlists.Playlist,
	idx int,
	playIdx int,
) tea.Cmd {
	song := playlist.Songs[idx]
	if len(song.Hash) != 0 {
		return func() tea.Msg {
			return hashMsg{
				playlisyIdx: playIdx,
				idx:         idx,
				hash:        song.Hash,
			}
		}
	}
	song.HashAudio()
	return func() tea.Msg {
		return hashMsg{
			playlisyIdx: playIdx,
			idx:         idx,
			hash:        song.Hash,
		}
	}
}

func (ps *PlayState) GetTimePos() uint64 {
	if noTime.Equal(ps.lastTimePosCheck) {
		return 0
	} else if ps.pause {
		return ps.timePosMs
	}
	return uint64(time.Since(ps.lastTimePosCheck).Milliseconds()) + ps.timePosMs
}

func (m *Model) AsMUGR() playlists.MUGRFile {
	queue := m.GetQueue()
	cs := queue.CurrSong
	if queue.IsShuffled {
		cs = queue.ShuffledSongs[cs]
	}
	mugr := playlists.MUGRFile{
		Loop:         byte(m.loop),
		LastPlayed:   cs,
		Queue:        m.queue,
		SongMetadata: m.tags,
		Playlists:    make(map[string][]string),
	}
	for _, playlist := range m.playlists {
		name, files, err := playlist.Save()
		if err != nil {
			continue
		}
		mugr.Playlists[name] = files
	}
	return mugr
}

func (m *Model) Load(mugr playlists.MUGRFile) error {
	e := mugr.Validate()
	if e != nil {
		return e
	}
	m.loop = Loop(mugr.Loop)
	m.queue = mugr.Queue
	m.tags = mugr.SongMetadata
	i := 0
	for name, files := range mugr.Playlists {
		playlist := playlists.Load(name, files)
		if i == m.queue {
			playlist.CurrSong = mugr.LastPlayed
		}
		m.playlists = append(m.playlists, playlist)
	}
	return nil
}

func (m *Model) GetQueue() *playlists.Playlist {
	if len(m.playlists) == 0 {
		return nil
	}
	if m.queue < 0 {
		return nil
	}
	if m.queue > len(m.playlists)-1 {
		return nil
	}
	return &m.playlists[m.queue]
}

func InitModel() (Model, chan struct{}, *ipc.MpvClient, error) {
	client, err := ipc.InitIpc(GeneratePath(), GeneratePulsePath(), true)
	if err != nil {
		return Model{}, nil, nil, err
	}
	msgChan := make(chan ipc.MpvResponse, 50)
	quitChan := make(chan struct{}, 2)
	go client.MpvReplies(msgChan, quitChan)
	client.SendCommand(ipc.ObserveProperty("pause"))
	client.SendCommand(ipc.ObserveProperty("path"))
	client.SendCommand(ipc.ObserveProperty("media-title"))
	client.SendCommand(ipc.ObserveProperty("duration/full"))
	client.SendCommand(ipc.ObserveProperty("time-pos/full"))
	filepicker := fp.New()
	filepicker.DirAllowed = true
	filepicker.FileAllowed = true
	filepicker.ShowHidden = true
	filepicker.KeyMap = fp.DefaultKeyMap()
	filepicker.Styles = fp.DefaultStyles()
	var op ProgressBarOptions = ProgressBarOptions{
		Width:      80,
		FilledChar: '━', UnfilledChar: '━',
		FilledColor: ansi.WHITE, UnfilledColor: ansi.BBLACK,
	}
	textInput := txt.New()
	textInput.SetWidth(45)
	tags := make(map[string]playlists.SongMetadata)
	playState := PlayState{}
	return Model{
		client:     client,
		msgChan:    msgChan,
		screen:     PickingMain,
		filepicker: &filepicker,
		progress:   &op,
		ti:         &textInput,
		showQueue:  true,
		menuPos:    NoMenu,
		tags:       tags,
		playState:  &playState,
	}, quitChan, client, nil
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.filepicker.Init(), waitForMpv(m.msgChan))
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			if msg.Keystroke() == "ctrl+c" {
				return m, tea.Quit
			} else if msg.Keystroke() == "ctrl+s" && m.menuPos == NoMenu && !m.saving {
				if len(m.savePath) != 0 {
					_, e := os.Stat(m.savePath)
					if e != nil {
						m.saving = true
					} else {
						m.AsMUGR().Save(m.savePath)
						m.saved = true
						cmds = append(cmds, savedCmd())
					}
				} else {
					m.saving = true
				}
			} else if msg.Keystroke() == "ctrl+e" && m.menuPos == NoMenu && !m.saving {
				m.saving = true
			}
		}
	case MpvMsg:
		{
			m.handleMpvMsg(msg)
			return m, tea.Batch(waitForMpv(m.msgChan))
		}
	case tea.WindowSizeMsg:
		{
			m.height, m.width = msg.Height, msg.Width
			m.progress.Width = m.width
			m.ti.SetWidth(int(float64(m.width) * 0.75))
			m.updateCurrState()
		}
	case ClearErrMsg:
		{
			m.err = nil
			return m, nil
		}
	case ClearSavedMsg:
		{
			m.saved = false
			return m, nil
		}
	case hashMsg:
		{
			playlist := &m.playlists[msg.playlisyIdx]
			playlist.Songs[msg.idx].Hash = msg.hash
			metadata, ok := m.tags[playlist.Songs[msg.idx].Hash]
			if ok {
				playlist.Songs[msg.idx].MergeMetadata(metadata)
			}
			if msg.idx == len(playlist.Songs)-1 {
				m.updateCurrState()
				return m, nil
			}
			return m, tea.Batch(hashSong(playlist, msg.idx+1, msg.playlisyIdx))
		}
	}
	switch m.screen {
	case PickingMain:
		{
			cmd := m.updatePickingMain(msg)
			return m, cmd
		}
	case Player:
		{
			cmd := m.updatePlayer(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
	}
	return m, nil
}

func (m *Model) View() tea.View {
	switch m.screen {
	case Player:
		{
			return m.viewPlayer()
		}
	case PickingMain:
		{
			return m.viewPickingMain()
		}
	}
	return tea.NewView("")
}
