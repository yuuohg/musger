package ui

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "musger/ansi"
	. "musger/ipc"
	. "musger/playlists"

	fp "charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
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
	song             *Song
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
	client     *MpvClient
	msgChan    chan MpvResponse
	playState  PlayState
	queue      Playlist
	screen     Screen
	playlists  []Playlist
	filepicker fp.Model
	progress   progress.Model
	height     int
	width      int
	loop       Loop
	count      int64
	err        error
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

func msToReadable(ms uint64) string {
	var s strings.Builder
	secs := float64(ms) / 1000
	hours := math.Trunc(secs / 3600)
	mins := math.Trunc(math.Mod(secs, 3600) / 60)
	sec := math.Trunc(math.Mod(math.Mod(math.Mod(secs, 3600), 60), 60))
	if hours > 0 {
		if hours < 10 {
			s.WriteString("0")
		}
		fmt.Fprint(&s, hours)
		s.WriteString(":")
	}
	if mins < 10 {
		s.WriteString("0")
	}
	fmt.Fprint(&s, mins)
	s.WriteString(":")
	if sec < 10 {
		s.WriteString("0")
	}
	fmt.Fprint(&s, sec)
	return s.String()
}

func handlePropertyChange(event MpvMsg, playState *PlayState) {
	var ok bool
	switch event.Name {
	case "pause":
		{
			playState.pause, ok = event.Data.(bool)
			if !ok {
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

func (ps *PlayState) GetTimePos() uint64 {
	e := time.Time{}
	if e.Equal(ps.lastTimePosCheck) {
		return 0
	}
	return uint64(
		time.Since(ps.lastTimePosCheck).Milliseconds() + int64(
			ps.timePosMs,
		),
	)
}

func InitModel() (Model, chan struct{}, *MpvClient, error) {
	Logf(BLUE, "Starting procesaes")
	client, err := InitServer(GeneratePath(), GeneratePulsePath())
	if err != nil {
		return Model{}, nil, nil, err
	}
	msgChan := make(chan MpvResponse, 50)
	quitChan := make(chan struct{}, 2)
	go client.MpvReplies(msgChan, quitChan)
	Logf(BLUE, "Listening for mpv's replies")
	client.SendCommand(ObserveProperty("pause"))
	Logf(BLUE, "Observing property: 'pause'")
	client.SendCommand(ObserveProperty("path"))
	Logf(BLUE, "Observing property: 'path'")
	client.SendCommand(ObserveProperty("media-title"))
	Logf(BLUE, "Observing property: 'media-title'")
	client.SendCommand(ObserveProperty("duration/full"))
	Logf(BLUE, "Observing property: 'duration/full'")
	client.SendCommand(ObserveProperty("time-pos/full"))
	Logf(BLUE, "Observing property: 'time-pos/full'")
	filepicker := fp.New()
	filepicker.DirAllowed = true
	filepicker.FileAllowed = true
	filepicker.ShowHidden = true
	filepicker.ShowSize = true
	filepicker.KeyMap = fp.DefaultKeyMap()
	filepicker.Styles = fp.DefaultStyles()
	prog := progress.New()
	prog.EmptyColor = lg.BrightBlack
	prog.FullColor = lg.White
	prog.Full = '━'
	prog.Empty = '━'
	prog.ShowPercentage = false
	return Model{
		client:     client,
		msgChan:    msgChan,
		screen:     PickingMain,
		filepicker: filepicker,
		progress:   prog,
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
		var playlist Playlist
		playlist, m.err = NewAD(selection)
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
		m.queue = playlist
	} else {
		mug := MUGRFile{}
		m.err = mug.Load(selection)
		if m.err != nil {
			return m, tea.Batch(cmd, errorCmd())
		}
	}
	m.screen = Player
	return m, cmd
}

type (
	MpvMsg      MpvResponse
	ClearErrMsg struct{}
)

func waitForMpv(msgChan chan MpvResponse) tea.Cmd {
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

func (m Model) viewPickingMain() tea.View {
	var s strings.Builder
	s.WriteString("Pick a directory with music or a valid json file\n\n")
	s.WriteString(m.filepicker.View())
	if m.err != nil {
		errStyle := lg.NewStyle().Bold(true).Foreground(lg.Red)
		s.WriteString("\n\n")
		s.WriteString(errStyle.Render(m.err.Error()))
	}
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}

func (m Model) handleMpvMsg(msg MpvMsg) (tea.Model, tea.Cmd) {
	if msg.Event == "property-change" {
		prevNT := m.playState.nextTitle
		handlePropertyChange(msg, &m.playState)
		if prevNT != m.playState.nextTitle && m.playState.greenlit &&
			!m.playState.song.FromHash {
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

				song, err := m.queue.Next(true)
				if err != nil {
					m.err = err
					break
				}
				m.count++
				if m.queue.IsShuffled {
					m.playState.song = m.queue.Songs[m.queue.ShuffledSongs[song]].Load(m.client)
				} else {
					m.playState.song = m.queue.Songs[song].Load(m.client)
				}
			}
		case RepeatOnce:
			{
				if m.queue.CurrSong == len(m.queue.Songs)-1 {
					break
				}
				song, err := m.queue.Next(true)
				if err != nil {
					m.err = err
					break
				}
				m.count++
				if m.queue.IsShuffled {
					m.playState.song = m.queue.Songs[m.queue.ShuffledSongs[song]].Load(m.client)
				} else {
					m.playState.song = m.queue.Songs[song].Load(m.client)
				}
			}
		}
	} else if msg.Event == "start-file" && msg.PlaylistEntryID == m.count {
		m.playState.greenlit = true
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			if msg.String() == "q" {
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
		}
	case ClearErrMsg:
		{
			m.err = nil
			return m, tea.Batch(waitForMpv(m.msgChan))
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
