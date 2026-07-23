package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

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
				playState.nextTitle, ok = event.Data.(string)
				if !ok {
					playState.nextTitle = ""
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
	if len(final) > width-1 {
		diff := (len(final) - (width - 1)) + 3 + 1
		final = title[:len(final)-diff] + "..." + sep + artist + ending
	}
	return final
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
		ViewSong(true, !ps.pause, width, &playlistView[p.CurrSong]),
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

func initModel() (Model, chan Empty, *MpvClient, error) {
	logf(BLUE, "Starting procesaes")
	client, err := InitServer(GeneratePath(), GeneratePulsePath())
	if err != nil {
		return Model{}, nil, nil, err
	}
	msgChan := make(chan MpvResponse, 50)
	quitChan := make(chan Empty, 2)
	go client.mpvReplies(msgChan, quitChan)
	logf(BLUE, "Listening for mpv's replies")
	client.sendCommand(observeProperty("pause"))
	logf(BLUE, "Observing property: 'pause'")
	client.sendCommand(observeProperty("path"))
	logf(BLUE, "Observing property: 'path'")
	client.sendCommand(observeProperty("media-title"))
	logf(BLUE, "Observing property: 'media-title'")
	client.sendCommand(observeProperty("duration/full"))
	logf(BLUE, "Observing property: 'duration/full'")
	client.sendCommand(observeProperty("time-pos/full"))
	logf(BLUE, "Observing property: 'time-pos/full'")
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
					m.client.sendCommand(TogglePlay(m.playState.pause))
				}
			case "n", "f", "down":
				{
					song, err := m.queue.Next(true)
					if err != nil {
						m.err = err
						break
					}
					if m.queue.IsShuffled {
						m.playState.song = m.queue.Songs[m.queue.ShuffledSongs[song]].Load(m.client)
					} else {
						m.playState.song = m.queue.Songs[song].Load(m.client)
					}
				}
			case "b", "up":
				{
					song, err := m.queue.Prev(true)
					if err != nil {
						m.err = err
						break
					}
					if m.queue.IsShuffled {
						m.playState.song = m.queue.Songs[m.queue.ShuffledSongs[song]].Load(m.client)
					} else {
						m.playState.song = m.queue.Songs[song].Load(m.client)
					}
				}
			case "h", "left":
				{
					if m.playState.fileName != "" {
						m.client.sendCommand(SeekBackwardFive)
					}
				}
			case "l", "right":
				{
					if m.playState.fileName != "" {
						m.client.sendCommand(SeekForwardFive)
					}
				}
			case "c":
				{
					if m.loop == RepeatOne {
						m.loop = RepeatOnce
					} else {
						m.loop++
					}
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

type (
	MpvMsg      MpvResponse
	ClearErrMsg Empty
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
				m.playState.song = m.queue.Songs[m.queue.CurrSong].Load(m.client)
			}
		case RepeatAll:
			{

				song, err := m.queue.Next(true)
				if err != nil {
					m.err = err
					break
				}

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

				if m.queue.IsShuffled {
					m.playState.song = m.queue.Songs[m.queue.ShuffledSongs[song]].Load(m.client)
				} else {
					m.playState.song = m.queue.Songs[song].Load(m.client)
				}
			}
		}
	} else if msg.Event == "file-loaded" {
		m.playState.greenlit = true
	} else if msg.Event == "start-file" {
		m.playState.greenlit = false
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
