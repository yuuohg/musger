package ui

import (
	"os"
	"time"

	"musger/ipc"
	"musger/playlists"

	fp "charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/progress"
	txt "charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
)

var (
	border      lg.Border = lg.RoundedBorder()
	borderStyle           = lg.NewStyle().BorderStyle(border).Render
	padding               = lg.NewStyle().Padding(0, 1, 0, 1).Render
	errStyle              = lg.NewStyle().Bold(true).Foreground(lg.Red).Render
	titleStyle            = lg.NewStyle().Width(80).Align(lg.Center).Render
	noTime                = time.Time{}
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

type Screen int

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
		id   int
		idx  int
		hash string
	}
)

func hashSong(
	playlist *playlists.Playlist,
	idx int,
	id int,
) tea.Cmd {
	song := playlist.Songs[idx]
	if len(song.Hash) != 0 {
		return func() tea.Msg {
			return hashMsg{
				id:   id,
				idx:  idx,
				hash: song.Hash,
			}
		}
	}
	song.HashAudio()
	return func() tea.Msg {
		return hashMsg{
			id:   id,
			idx:  idx,
			hash: song.Hash,
		}
	}
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

type PlayState struct {
	song             *playlists.Song
	pause            bool
	durationMs       uint64
	timePosMs        uint64
	fileName         string
	fromHash         bool
	nextTitle        string
	greenlit         bool
	lastTimePosCheck time.Time
}

type Model struct {
	client       *ipc.MpvClient
	msgChan      chan ipc.MpvResponse
	playState    PlayState
	queue        int
	screen       Screen
	playlists    []playlists.Playlist
	filepicker   fp.Model
	progress     progress.Model
	ti           txt.Model
	menuSong     *playlists.Song
	loop         Loop
	ids          map[int]int
	tags         map[string]playlists.SongMetadata
	currentState string
	savePath     string
	overwrite    string
	overwriteD   bool
	overwriting  bool
	showQueue    bool
	saving       bool
	saved        bool
	availableId  int
	menuPos      int
	height       int
	width        int
	count        int64
	err          error
}

func (ps *PlayState) GetTimePos() uint64 {
	if noTime.Equal(ps.lastTimePosCheck) {
		return 0
	} else if ps.pause {
		return ps.timePosMs
	}
	return uint64(time.Since(ps.lastTimePosCheck).Milliseconds()) + ps.timePosMs
}

func (m Model) AsMUGR() playlists.MUGRFile {
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
	prog := progress.New()
	prog.EmptyColor = lg.BrightBlack
	prog.FullColor = lg.White
	prog.Full = '━'
	prog.Empty = '━'
	prog.ShowPercentage = false
	textInput := txt.New()
	textInput.SetWidth(45)
	ids := make(map[int]int)
	tags := make(map[string]playlists.SongMetadata)
	return Model{
		client:     client,
		msgChan:    msgChan,
		screen:     PickingMain,
		filepicker: filepicker,
		progress:   prog,
		ti:         textInput,
		showQueue:  true,
		menuPos:    NoMenu,
		ids:        ids,
		tags:       tags,
	}, quitChan, client, nil
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.filepicker.Init(), waitForMpv(m.msgChan))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return m.handleMpvMsg(msg)
		}
	case tea.WindowSizeMsg:
		{
			m.height, m.width = msg.Height, msg.Width
			m.progress.SetWidth(m.width)
			m.ti.SetWidth(int(float64(m.width) * 0.75))
			titleStyle = lg.NewStyle().Width(m.width).Align(lg.Center).Render
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
			playlistIdx, ok := m.ids[msg.id]
			if !ok {
				break
			}
			playlist := &m.playlists[playlistIdx]
			playlist.Songs[msg.idx].Hash = msg.hash
			metadata, ok := m.tags[playlist.Songs[msg.idx].Hash]
			if ok {
				playlist.Songs[msg.idx].MergeMetadata(metadata)
			}
			if msg.idx == len(playlist.Songs)-1 {
				delete(m.ids, msg.id)
				m.updateCurrState()
				return m, nil
			}
			m.updateCurrState()
			return m, tea.Batch(hashSong(playlist, msg.idx+1, msg.id))
		}
	}
	switch m.screen {
	case PickingMain:
		{
			return m.updatePickingMain(msg)
		}
	case Player:
		{
			m, cmd := m.updatePlayer(msg)
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
	}
	return m, nil
}

func (m Model) View() tea.View {
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
