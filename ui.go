package main

import (
	"os"
	"strings"
	"time"

	fp "charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
	dump "github.com/goforj/godump"
)

func handleEvent(event MpvEvent, playState *PlayState) {
	var ok bool
	if event.Event == "property-change" {
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
					return
				}
			}
		case "media-title":
			{
				playState.title, ok = event.Data.(string)
				if !ok {
					return
				}
			}
		case "duration/full":
			{
				durationSecs, ok := event.Data.(float64)
				if !ok {
					return
				}
				playState.durationMs = secsToms(durationSecs)
			}
		}
	}
}

func eventHandler(
	eventChannel <-chan MpvEvent,
	dc *DaemonChannel,
	playstateChan chan PlayState,
	requestPlaystateChan chan Empty,
) {
	var playState PlayState
	dc.command(`{"command":["observe_property",1,"pause"]}`)
	dc.command(`{"command":["observe_property",1,"path"]}`)
	dc.command(`{"command":["observe_property",1,"media-title"]}`)
	dc.command(`{"command":["observe_property",1,"duration/full"]}`)
	for {
		select {
		case event := <-eventChannel:
			{
				handleEvent(event, &playState)
			}
		case <-requestPlaystateChan:
			{
				playstateChan <- playState
			}
		}
	}
}

func DurationDaemon(dc *DaemonChannel, dchan chan uint64, req chan bool) {
	var lastDurationQuery time.Time = time.Now()
	var lastCheckedDuration uint64
	tick := time.Tick(time.Millisecond * 100)
	for {
		select {
		case isPaused := <-req:
			{
				if isPaused {
					dchan <- lastCheckedDuration
				} else if lastCheckedDuration == 0 {
					dchan <- 0
				} else {
					timeSinceLastCheckMs := time.Since(lastDurationQuery).Milliseconds()
					d := uint64(timeSinceLastCheckMs) + uint64(lastCheckedDuration)
					dchan <- d
				}
			}
		case <-tick:
			{
				d := dc.CurrentPos()
				lastDurationQuery = time.Now()
				currentDuration, _ := d.Data.(float64)
				lastCheckedDuration = secsToms(currentDuration)
			}
		}
	}
}

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
	pause      bool
	durationMs uint64
	timePosMs  uint64
	fileName   string
	title      string
}

type Model struct {
	dc               *DaemonChannel
	playState        PlayState
	queue            Playlist
	screen           Screen
	playlists        []Playlist
	filepicker       fp.Model
	dchan            chan uint64
	reqdchan         chan bool
	playStateChan    chan PlayState
	reqPlayStateChan chan Empty
}

func initModel() (Model, chan Empty, error) {
	daemonChan, quitChan, eventChan, err := SetupDaemon()
	if err != nil {
		return Model{}, nil, err
	}
	durationChan := make(chan uint64, 10)
	durationReqChan := make(chan bool, 10)
	playstateChan := make(chan PlayState, 10)
	playstateReqChan := make(chan Empty, 10)
	go DurationDaemon(&daemonChan, durationChan, durationReqChan)
	go eventHandler(
		eventChan,
		&daemonChan,
		playstateChan,
		playstateReqChan,
	)
	filepicker := fp.New()
	filepicker.DirAllowed = true
	filepicker.FileAllowed = true
	filepicker.ShowHidden = true
	filepicker.ShowSize = true
	filepicker.KeyMap = fp.DefaultKeyMap()
	filepicker.Styles = fp.DefaultStyles()
	return Model{
		dc:               &daemonChan,
		playStateChan:    playstateChan,
		reqPlayStateChan: playstateReqChan,
		screen:           PickingMain,
		filepicker:       filepicker,
		dchan:            durationChan,
		reqdchan:         durationReqChan,
	}, quitChan, nil
}

func (m Model) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m Model) updatePickingMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)
	hasSelected, selection := m.filepicker.DidSelectFile(msg)
	if !hasSelected {
		return m, cmd
	}
	selectionF, _ := os.Stat(selection)
	if selectionF.IsDir() {
		playlist, _ := NewAD(selection, m.dc)
		m.playlists = append(m.playlists, playlist)
		m.queue = playlist
	} else {
		mug := MUGRFile{}
		mug.Load(selection)
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
					m.dc.TogglePlay()
				}
			case "n", "f":
				{
					m.queue.Next(true)
				}
			case "b", "h":
				{
					m.queue.Prev(true)
				}
			}
		}
	}
	m.reqPlayStateChan <- Nothing
	playState := <-m.playStateChan
	m.reqdchan <- playState.pause
	playState.timePosMs = <-m.dchan
	m.playState = playState
	return m, nil
}

func (m Model) viewPickingMain() tea.View {
	var s strings.Builder
	s.WriteString("Pick a directory with music or a valid json file\n\n")
	s.WriteString(m.filepicker.View())
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}

func (m Model) viewPlayer() tea.View {
	var s strings.Builder
	dump.Fdump(&s, m.playState)
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		{
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
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
