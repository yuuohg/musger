package ui

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
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
	client      *ipc.MpvClient
	msgChan     chan ipc.MpvResponse
	playState   PlayState
	queue       int
	screen      Screen
	playlists   []playlists.Playlist
	filepicker  fp.Model
	progress    progress.Model
	ti          txt.Model
	menuSong    *playlists.Song
	loop        Loop
	ids         map[int]*playlists.Playlist
	tags        map[string]playlists.SongMetadata
	availableId int
	showQueue   bool
	menuPos     int
	height      int
	width       int
	count       int64
	err         error
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

type Loop int

const (
	RepeatOnce Loop = iota
	RepeatAll
	RepeatOne
)

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

// function below is ai-generated
func calculateLaLb(
	lookAhead, lookBehind, arrayLen, currentIdx int,
) (lB, lA int) {
	if arrayLen <= 1 {
		return 0, 0
	}
	availBehind := currentIdx
	availAhead := (arrayLen - 1) - currentIdx
	lB = min(lookBehind, availBehind)
	lA = min(lookAhead, availAhead)
	if unusedB := lookBehind - lB; unusedB > 0 {
		lA = min(availAhead, lA+unusedB)
	} else if unusedA := lookAhead - lA; unusedA > 0 {
		lB = min(availBehind, lB+unusedA)
	}
	return lB, lA
}

var characters = []rune(
	"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_",
)

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = characters[rand.Intn(len(characters))]
	}
	return string(b)
}

func GeneratePath() string {
	rt := os.ExpandEnv("$TMPDIR")
	if rt == "" {
		return "/usr/tmp/" + randSeq(32) + "_mpv.sock"
	}
	return rt + "/" + randSeq(32) + "_mpv.sock"
}

func GeneratePulsePath() string {
	rt := os.ExpandEnv("$TMPDIR")
	if rt == "" {
		return "/usr/tmp/" + "pulse.sock"
	}
	return rt + "/" + "pulse.sock"
}

func secsToms(s float64) uint64 {
	return uint64(math.Round(s * 1000))
}

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

func (ps *PlayState) GetTimePos() uint64 {
	e := time.Time{}
	if e.Equal(ps.lastTimePosCheck) {
		return 0
	} else if ps.pause {
		return ps.timePosMs
	}
	return uint64(
		time.Since(ps.lastTimePosCheck).Milliseconds() + int64(
			ps.timePosMs,
		),
	)
}

func InitModel() (Model, chan struct{}, *ipc.MpvClient, error) {
	client, err := ipc.InitIpc(GeneratePath(), GeneratePulsePath())
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
	ids := make(map[int]*playlists.Playlist)
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

func (m Model) updatePickingMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)
	hasSelected, selection := m.filepicker.DidSelectFile(msg)
	if !hasSelected {
		return m, cmd
	}
	var selectionF os.FileInfo
	selectionF, m.err = os.Stat(selection)
	if m.err != nil {
		m.err = fmt.Errorf("selection: '%v', %w", selection, m.err)
		return m, tea.Batch(cmd, errorCmd())
	}
	if selectionF.IsDir() {
		var playlist playlists.Playlist
		playlist, m.err = playlists.NewAD(selection)
		if m.err != nil {
			return m, tea.Batch(cmd, errorCmd())
		}
		if len(playlist.Songs) == 0 {
			m.err = fmt.Errorf(
				"No audio files found in '%v' directory",
				selection,
			)
			return m, tea.Batch(cmd, errorCmd())
		}
		m.playlists = append(m.playlists, playlist)
		m.queue = 0
	} else {
		mug := playlists.MUGRFile{}
		m.err = mug.Load(selection)
		if m.err != nil {
			return m, tea.Batch(cmd, errorCmd())
		}
	}
	m.screen = Player
	return m, cmd
}

type (
	MpvMsg  ipc.MpvResponse
	hashMsg struct {
		id   int
		idx  int
		hash string
	}
	ClearErrMsg struct{}
)

func waitForMpv(msgChan chan ipc.MpvResponse) tea.Cmd {
	return func() tea.Msg {
		return MpvMsg(<-msgChan)
	}
}

func errorCmd() tea.Cmd {
	return tea.Tick(
		time.Second*2,
		func(t time.Time) tea.Msg { return ClearErrMsg{} },
	)
}

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

func (m Model) viewPickingMain() tea.View {
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

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			if msg.Mod == tea.ModCtrl && msg.Code == 'c' {
				return m, tea.Quit
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
		}
	case ClearErrMsg:
		{
			m.err = nil
			return m, tea.Batch(waitForMpv(m.msgChan))
		}
	case hashMsg:
		{
			playlist, ok := m.ids[msg.id]
			if !ok {
				break
			}
			playlist.Songs[msg.idx].Hash = msg.hash
			metadata, ok := m.tags[playlist.Songs[msg.idx].Hash]
			if ok {
				playlist.Songs[msg.idx].MergeMetadata(metadata)
			}
			if msg.idx == len(playlist.Songs)-1 {
				delete(m.ids, msg.id)
				return m, nil
			}
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
			return m.updatePlayer(msg)
		}
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
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
