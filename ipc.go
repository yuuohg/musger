package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"maps"
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
	closed    bool
	client    *MpvClient
	eventChan chan MpvEvent
	channels  map[int64]chan MpvResponse
	m         sync.Mutex
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
	Send    chan string
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

func (dc *DaemonChannel) Play() MpvResponse {
	return dc.command(`{"command":["set_property","pause",false]}`)
}

func (dc *DaemonChannel) Pause() MpvResponse {
	return dc.command(`{"command":["set_property","pause",true]}`)
}

func (dc *DaemonChannel) TogglePlay() MpvResponse {
	playState := dc.command(`{"command":["get_property","pause"]}`)
	if playState.Data == true {
		return dc.Play()
	}
	return dc.Pause()
}

func (dc *DaemonChannel) PlayFile(file string) MpvResponse {
	return dc.command(`{"command":["loadfile","` + file + `"]}`)
}

func InitDaemon(client *MpvClient) (chan MpvEvent, MpvDaemon) {
	eventc := make(chan MpvEvent, 10)
	channels := make(map[int64]chan MpvResponse)
	return eventc, MpvDaemon{
		client:    client,
		eventChan: eventc,
		channels:  channels,
	}
}

func mpvReplies(md *MpvDaemon, conn net.Conn, qc chan int) {
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case _ = <-qc:
			{
				qc <- 0
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
					md.eventChan <- event
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
	qc <- 0
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
	qc chan int,
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
				qc <- 0
				break
			}
		}
	}
}

func (md *MpvDaemon) RunDaemon(
	sendCommands chan string,
	recieveChan chan chan MpvResponse,
	quit chan int,
) {
	var (
		mpvRepQC         = make(chan int, 2)
		WaitOnCommandsQC = make(chan int, 2)
	)
	go mpvReplies(md, md.client.conn, mpvRepQC)
	go waitForCommands(sendCommands, recieveChan, md, WaitOnCommandsQC)
	<-quit
	mpvRepQC <- 1
	WaitOnCommandsQC <- 1
	<-mpvRepQC
	<-WaitOnCommandsQC
	md.client.Close()
	md.closed = true
	quit <- 0
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
		"--demuxer-max-bytes=1MiB",
		"--demuxer-max-back-bytes=0",
		"--demuxer-readahead-secs=0",
		"--audio-buffer=0.1",
		"--ytdl=no",
		"--osc=no",
		"--osd-level=0",
		"--no-input-default-bindings",
		"--load-scripts=no",
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

func SetupDaemon() (dc DaemonChannel, quitChan chan int, eventChan chan MpvEvent, err error) {
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
	quitChan = make(chan int)
	dc = DaemonChannel{Send: commandChan, Receive: responseChan}
	go daemon.RunDaemon(commandChan, responseChan, quitChan)
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
