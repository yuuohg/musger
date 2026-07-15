package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"
)

type MpvClient struct {
	path   string
	conn   net.Conn
	mpvCmd *exec.Cmd
}

type MpvDaemon struct {
	m            sync.Mutex
	closed       bool
	client       *MpvClient
	channels     map[int64]chan MpvResponse
	eventChannel chan MpvEvent
}

type MpvResponse struct {
	Data      any    `json:"data,omitempty"`
	Error     string `json:"error,omitempty"`
	Event     string `json:"event,omitempty"`
	Reason    string `json:"reason,omitempty"`
	RequestID int64  `json:"request_id,omitempty"`
}

type MpvEvent struct {
	Data            any     `json:"data,omitempty"`
	Text            string  `json:"text,omitempty"`
	Name            string  `json:"name,omitempty"`
	Event           string  `json:"event,omitempty"`
	Level           string  `json:"level,omitempty"`
	Reason          string  `json:"reason,omitempty"`
	Prefix          string  `json:"prefix,omitempty"`
	PlaylistEntryID float64 `json:"playlist_entry_id,omitempty"`
}

type DaemonChannel struct {
	Send    chan<- string
	Receive chan chan MpvResponse
	Lock    sync.Mutex
}

func (dc *DaemonChannel) command(c string) MpvResponse {
	dc.Lock.Lock()
	dc.Send <- c
	r := <-dc.Receive
	dc.Lock.Unlock()
	return <-r
}

func (dc *DaemonChannel) getProperty(property string) MpvResponse {
	return dc.command(`{"command":["get_property","` + property + `"]}`)
}

func (dc *DaemonChannel) setProperty(property string, data any) MpvResponse {
	return dc.command(
		fmt.Sprintf(`{"command":["set_property", "%v", %v]}`, property, data),
	)
}

func (dc *DaemonChannel) Play() MpvResponse {
	return dc.setProperty("pause", false)
}

func (dc *DaemonChannel) Pause() MpvResponse {
	return dc.setProperty("pause", true)
}

func (dc *DaemonChannel) TogglePlay() MpvResponse {
	playState := dc.getProperty("pause")
	if playState.Data == true {
		return dc.Play()
	}
	return dc.Pause()
}

func (dc *DaemonChannel) Duration() MpvResponse {
	return dc.getProperty("duration/full")
}

func (dc *DaemonChannel) CurrentPos() MpvResponse {
	return dc.getProperty("time-pos/full")
}

func secsToms(s float64) uint {
	return uint(math.Round(s * 1000))
}

func (dc *DaemonChannel) PlayFile(file string) MpvResponse {
	return dc.command(`{"command":["loadfile","` + file + `"]}`)
}

func InitDaemon(client *MpvClient) (chan MpvEvent, MpvDaemon) {
	eventc := make(chan MpvEvent, 10)
	channels := make(map[int64]chan MpvResponse)
	return eventc, MpvDaemon{
		client:       client,
		channels:     channels,
		eventChannel: eventc,
	}
}

func mpvReplies(md *MpvDaemon, qc chan Empty) {
	scanner := bufio.NewScanner(md.client.conn)
	for scanner.Scan() {
		select {
		case _ = <-qc:
			{
				qc <- Nothing
				break
			}
		default:
			{
				reply := scanner.Text()
				var response MpvResponse
				bReply := []byte(reply)
				json.Unmarshal(bReply, &response)
				if response.Event != "" {
					var event MpvEvent
					json.Unmarshal(bReply, &event)
					md.eventChannel <- event
				}
				if response.RequestID != 0 {
					md.m.Lock()
					ch, inChan := md.channels[response.RequestID]
					if !inChan {
						md.m.Unlock()
						continue
					}
					ch <- response
					close(ch)
					delete(md.channels, response.RequestID)
					md.m.Unlock()
				}
			}
		}
	}
	_ = scanner.Err()
	qc <- Nothing
}

func generateUniqueRID(md *MpvDaemon) int64 {
	var requestID int64
	rids := slices.Collect(maps.Keys(md.channels))
	for {
		requestID = int64(rand.Uint64())
		if !slices.Contains(rids, requestID) {
			break
		}
	}
	return requestID
}

func cleanUpString(s string) string {
	noSpaces := strings.TrimSpace(s)
	return noSpaces[:len(noSpaces)-1]
}

func waitForCommands(
	sendCommands chan string,
	recieveChan chan chan MpvResponse,
	md *MpvDaemon,
	qc chan Empty,
) {
	for {
		select {
		case command := <-sendCommands:
			{
				if len(command) == 0 {
					continue
				}
				md.m.Lock()
				requestID := generateUniqueRID(md)
				f := fmt.Sprintf(
					`%v,"request_id":%v}`,
					cleanUpString(command),
					requestID,
				)
				_, err := fmt.Fprintln(md.client.conn, f)
				if err != nil {
					md.m.Unlock()
					break
				}
				responseChan := make(chan MpvResponse, 2)
				md.channels[requestID] = responseChan
				md.m.Unlock()
				recieveChan <- responseChan
			}
		case _ = <-qc:
			{
				qc <- Nothing
				break
			}
		}
	}
}

func (md *MpvDaemon) RunDaemon(
	sendCommands chan string,
	recieveChan chan chan MpvResponse,
	quit chan Empty,
) {
	var (
		mpvRepQC         = make(chan Empty, 2)
		WaitOnCommandsQC = make(chan Empty, 2)
	)
	go mpvReplies(md, mpvRepQC)
	go waitForCommands(sendCommands, recieveChan, md, WaitOnCommandsQC)
	<-quit
	mpvRepQC <- Nothing
	WaitOnCommandsQC <- Nothing
	<-mpvRepQC
	<-WaitOnCommandsQC
	md.client.Close()
	md.closed = true
	quit <- Nothing
}

func (mpvc *MpvClient) Close() error {
	err := mpvc.conn.Close()
	if err != nil {
		return err
	}
	err = os.Remove(mpvc.path)
	if err != nil {
		return err
	}
	k := exec.Command("pkill", "-9", "-f", `mpv --idle=yes .*_mpv\.sock`)
	k.Run()
	k = exec.Command("pkill", "-9", "-f", "pulseaudio.*")
	k.Run()
	return nil
}

func InitServer(path string) (*MpvClient, error) {
	p := exec.Command("pkill", "-9", "-f", `mpv --idle=yes .*_mpv\.sock`)
	k := exec.Command("pkill", "-9", "-f", "pulseaudio.*")
	p.Run()
	k.Run()
	ipcServerOption := fmt.Sprintf("--input-ipc-server=%v", path)
	pulse := exec.Command("pulseaudio", "--start", "--exit-idle-time=-1")
	err := pulse.Run()
	if err != nil {
		return nil, fmt.Errorf("Couldn't start pulseaudio: %w", err)
	}
	cmd := exec.Command(
		"mpv",
		"--idle=yes",
		"--ao=pulse",
		"--no-video",
		"--audio-display=no",
		"--vo=null",
		"--no-config",
		"--cache=no",
		"--demuxer-max-bytes=512KiB",
		"--demuxer-max-back-bytes=512KiB",
		"--demuxer-readahead-secs=0",
		"--audio-buffer=0.1",
		"--ytdl=no",
		"--osc=no",
		"--osd-level=0",
		"--no-input-default-bindings",
		"--load-scripts=no",
		"--terminal=no",
		ipcServerOption,
	)
	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("Couldn't start mpv: %w", err)
	}
	if cmd.Process == nil {
		return nil, fmt.Errorf("Server did not start")
	}
	for range 1000 {
		_, err := os.Stat(path)
		if err == nil {
			break
		}
		time.Sleep(time.Millisecond * 25)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, fmt.Errorf("Couldn't connect to ipc socket: %w", err)
	}
	return &MpvClient{path, conn, cmd}, nil
}

func SetupDaemon() (dc DaemonChannel, quitChan chan Empty, eventChan <-chan MpvEvent, err error) {
	mpvClient, err := InitServer(GeneratePath())
	if err != nil {
		return DaemonChannel{}, nil, nil, fmt.Errorf(
			"Could not start mpv: %w",
			err,
		)
	}
	eventChan, daemon := InitDaemon(mpvClient)
	commandChan := make(chan string, 10)
	responseChan := make(chan chan MpvResponse, 10)
	quitChan = make(chan Empty)
	go daemon.RunDaemon(commandChan, responseChan, quitChan)
	dc = DaemonChannel{Send: commandChan, Receive: responseChan}
	return
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
		return "/usr/tmp/" + randSeq(24) + "_mpv.sock"
	}
	return rt + "/" + randSeq(24) + "_mpv.sock"
}
