package ui

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"musger/ipc"
	"musger/playlists"

	fp "charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/progress"
	txt "charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lg "charm.land/lipgloss/v2"
	"github.com/clipperhouse/uax29/v2/graphemes"
	rw "github.com/mattn/go-runewidth"
)

var (
	border      lg.Border = lg.RoundedBorder()
	borderStyle           = lg.NewStyle().BorderStyle(border).Render
	padding               = lg.NewStyle().Padding(0, 1, 0, 1).Render
	errStyle              = lg.NewStyle().Bold(true).Foreground(lg.Red).Render
	titleStyle            = lg.NewStyle().Width(80).Align(lg.Center).Render
	runeCache             = make(map[rune]int)
	strCache              = make(map[string]int)
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
	availableId  int
	showQueue    bool
	menuPos      int
	height       int
	width        int
	count        int64
	err          error
}

func (m Model) AsMUGR() playlists.MUGRFile {
	queue := m.GetQueue()
	mugr := playlists.MUGRFile{
		Loop:         int(m.loop),
		LastPlayed:   queue.CurrSong,
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

func (m *Model) Load(mugr playlists.MUGRFile) {
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

// taken from github.com/mattn/go-runewidth
func graphemeWidth(cluster string) int {
	width := 0
	for _, r := range cluster {
		width += RuneWidth(r)
	}
	if width > 2 {
		width = 2
	}
	return width
}

func StringWidth(s string) (width int) {
	width, ok := strCache[s]
	if ok {
		return width
	}
	if len(s) == 1 {
		b := s[0]
		if b < 0x20 || b == 0x7F {
			return 0
		}
		return 1
	}
	if len(s) > 0 && len(s) <= utf8.UTFMax {
		r, size := utf8.DecodeRuneInString(s)
		if size == len(s) {
			return RuneWidth(r)
		}
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 0x80 {
			goto graphemes
		}
		if b >= 0x20 && b != 0x7F {
			width++
		}
	}
	strCache[s] = width
	return

graphemes:
	width = 0
	g := graphemes.FromString(s)
	for g.Next() {
		width += graphemeWidth(g.Value())
	}
	strCache[s] = width
	return
}

func RuneWidth(r rune) int {
	width, ok := runeCache[r]
	if ok {
		return width
	}
	width = rw.RuneWidth(r)
	runeCache[r] = width
	return width
}

func Truncate(str string, target int, tail string) string {
	target = target - StringWidth(tail)
	if len(str) < target {
		return str
	}
	var (
		final        strings.Builder
		currentWidth int
	)
	final.Grow(len(str))
	if target <= 0 {
		return final.String()
	}
	for _, ch := range str {
		if ch >= 0x20 && ch < 0x7F {
			currentWidth++
		} else {
			currentWidth += RuneWidth(ch)
		}
		if currentWidth > target {
			break
		}
		final.WriteRune(ch)
	}
	if final.Len() > 0 {
		final.WriteString(tail)
	}
	return final.String()
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
		return "/usr/tmp/" + randSeq(48) + "_mpv.sock"
	}
	return rt + "/" + randSeq(48) + "_mpv.sock"
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
	return uint64(time.Since(ps.lastTimePosCheck).Milliseconds()) + ps.timePosMs
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
		mugr := playlists.MUGRFile{}
		m.err = mugr.Load(selection)
		if m.err != nil {
			return m, tea.Batch(cmd, errorCmd())
		}
		m.Load(mugr)
		var idx int = m.queue
		m.ids[m.availableId] = idx
		m.availableId++
		cmd = tea.Batch(hashSong(&m.playlists[idx], 0, m.availableId-1), cmd)
	}
	m.screen = Player
	m.updateCurrState()
	return m, cmd
}

type (
	MpvMsg      ipc.MpvResponse
	ClearErrMsg struct{}
	hashMsg     struct {
		id   int
		idx  int
		hash string
	}
)

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
			} else if msg.Mod == tea.ModCtrl && msg.Code == 's' {
				mugr := m.AsMUGR()
				mugr.Save("test.json")
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
			return m, tea.Batch(waitForMpv(m.msgChan))
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
