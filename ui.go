package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"strings"
	"time"

	fp "charm.land/bubbles/v2/filepicker"
	tea "charm.land/bubbletea/v2"
)

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
			playState.title, ok = event.Data.(string)
			if !ok {
				playState.title = ""
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
				playState.lastTimePosCheck = time.Now()
				return
			}
			playState.timePosMs = secsToms(timePos)
			playState.lastTimePosCheck = time.Now()
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
	pause            bool
	durationMs       uint64
	timePosMs        uint64
	fileName         string
	title            string
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
}

func initModel() (Model, chan Empty, *MpvClient, error) {
	client, err := InitServer(GeneratePath(), GeneratePulsePath())
	if err != nil {
		return Model{}, nil, nil, err
	}
	msgChan := make(chan MpvResponse, 50)
	quitChan := make(chan Empty, 2)
	go client.mpvReplies(msgChan, quitChan)
	client.sendCommand(observeProperty("pause"))
	client.sendCommand(observeProperty("path"))
	client.sendCommand(observeProperty("media-title"))
	client.sendCommand(observeProperty("duration/full"))
	client.sendCommand(observeProperty("time-pos/full"))
	filepicker := fp.New()
	filepicker.DirAllowed = true
	filepicker.FileAllowed = true
	filepicker.ShowHidden = true
	filepicker.ShowSize = true
	filepicker.KeyMap = fp.DefaultKeyMap()
	filepicker.Styles = fp.DefaultStyles()
	return Model{
		client:     client,
		msgChan:    msgChan,
		screen:     PickingMain,
		filepicker: filepicker,
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
	selectionF, _ := os.Stat(selection)
	if selectionF.IsDir() {
		playlist, _ := NewAD(selection)
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
					m.client.sendCommand(TogglePlay(m.playState.pause))
				}
			case "n", "f":
				{
					path, err := m.queue.Next(true)
					if err != nil {
						break
					}
					if m.client.PulseaudioIsDead() {
						m.client.KillPulse()
					}
					m.client.sendCommand(loadfile(path))
				}
			case "b", "h":
				{
					path, err := m.queue.Prev(true)
					if err != nil {
						break
					}
					if m.client.PulseaudioIsDead() {
						m.client.KillPulse()
					}
					m.client.sendCommand(loadfile(path))
				}
			}
		}
	}
	return m, tea.Batch(waitForMpv(m.msgChan))
}

type MpvMsg MpvResponse

func waitForMpv(msgChan chan MpvResponse) tea.Cmd {
	return func() tea.Msg {
		return MpvMsg(<-msgChan)
	}
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
	fmt.Fprintf(
		&s,
		"Title: %v\nPosition: %v\nDuration: %v\nServer: %v\n",
		m.playState.title,
		m.playState.timePosMs,
		m.playState.durationMs,
		m.client.pulsePath,
	)
	v := tea.NewView(s.String())
	v.AltScreen = true
	return v
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
			handlePropertyChange(msg, &m.playState)
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
