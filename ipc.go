package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jfreymuth/pulse"
)

const (
	Play             = `{"command":["set_property","pause",false]}`
	Pause            = `{"command":["set_property","pause",true]}`
	SeekForwardFive  = `{"command":["seek",5]}`
	SeekBackwardFive = `{"command":["seek",-5]}`
	Stop             = `{"command":["stop"]}`
)

type MpvClient struct {
	path      string
	conn      net.Conn
	mpvCmd    *exec.Cmd
	pulsePath string
}

type MpvResponse struct {
	Data            any     `json:"data,omitempty"`
	Error           string  `json:"error,omitempty"`
	FileError       string  `json:"file_error,omitempty"`
	Event           string  `json:"event,omitempty"`
	Reason          string  `json:"reason,omitempty"`
	RequestID       int64   `json:"request_id,omitempty"`
	Text            string  `json:"text,omitempty"`
	Name            string  `json:"name,omitempty"`
	Level           string  `json:"level,omitempty"`
	Prefix          string  `json:"prefix,omitempty"`
	PlaylistEntryID float64 `json:"playlist_entry_id,omitempty"`
	originalJson    string
}

type PactlInfo struct {
	ServerString string `json:"server_string,omitempty"`
}

func (client *MpvClient) sendCommand(command string) {
	fmt.Fprintln(client.conn, strings.TrimSpace(command))
}

func loadfile(path string) string {
	return `{"command":["loadfile","` + path + `"]}`
}

func observeProperty(property string) string {
	return `{"command":["observe_property",1,"` + property + `"]}`
}

func TogglePlay(pause bool) string {
	if pause {
		return Play
	} else {
		return Pause
	}
}

func (client *MpvClient) mpvReplies(msgChan chan MpvResponse, qc chan Empty) {
	scanner := bufio.NewScanner(client.conn)
SCAN:
	for scanner.Scan() {
		select {
		case _ = <-qc:
			{
				qc <- Nothing
				break SCAN
			}
		default:
			{
				reply := scanner.Text()
				var response MpvResponse
				json.Unmarshal([]byte(reply), &response)
				response.originalJson = reply
				msgChan <- response
			}
		}
	}
	_ = scanner.Err()
	qc <- Nothing
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
	mpvc.mpvCmd.Process.Kill()
	return nil
}

func InitServer(path string, pulsePath string) (*MpvClient, error) {
	var err error
	ipcServerOption := fmt.Sprintf("--input-ipc-server=%v", path)
	logf(BLUE, "Socket: %v", path)
	if !PulseProcessAlive() {
		logf(BLUE, "Pulseaudio not running, starting")
		pulseSocketOption := fmt.Sprintf(
			"module-native-protocol-unix socket=%v",
			pulsePath,
		)
		pulse := exec.Command(
			"pulseaudio",
			"--start",
			"--exit-idle-time=-1",
			"-L",
			pulseSocketOption,
		)
		o, err := pulse.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("Couldn't start pulseaudio: %w", err)
		}
		if !pulse.ProcessState.Success() {
			return nil, fmt.Errorf(
				"Couldn't start pulseaudio:\nExit Code: %v\nOutput:\n%v\n",
				pulse.ProcessState.ExitCode(),
				string(o),
			)
		}
		logf(GREEN, "Pulseaudio started")
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
	logf(BLUE, "Starting mpv")
	if err != nil {
		return nil, fmt.Errorf("Couldn't start mpv: %w", err)
	}
	if cmd.Process == nil {
		return nil, fmt.Errorf("mpv did not start")
	}
	var i uint
	for range 1000 {
		i += 25
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
	logf(GREEN, "mpv started, took %.2fs", float64(i)/1000)
	return &MpvClient{path, conn, cmd, pulsePath}, nil
}

func logf(color, format string, a ...any) {
	fmt.Printf(color+format+RESET+"\n", a...)
}

func (client *MpvClient) KillPulse() error {
	pk := exec.Command("pulseaudio", "--kill")
	pk.Run()
	if PulseProcessAlive() {
		k := exec.Command("pkill", "-9", "-f", "pulseaudio.*")
		return k.Run()
	}
	return nil
}

func (client *MpvClient) UpdatePulsePath() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	logf(BLUE, "Finding server path")
	cmd := exec.CommandContext(ctx, "pactl", "-f", "json", "info")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	var s PactlInfo
	json.Unmarshal(out, &s)
	if s.ServerString != "" {
		logf(GREEN, "Found server path: %v", s.ServerString)
		client.pulsePath = s.ServerString
		return nil
	}
	return nil
}

func (client *MpvClient) PulseaudioIsDead() bool {
	c, e := pulse.NewClient(
		pulse.ClientServerString("unix:" + client.pulsePath),
	)
	if c != nil {
		c.Close()
	}
	return e != nil
}

func PulseProcessAlive() bool {
	c := exec.Command("pulseaudio", "--check")
	r := c.Run()
	if r != nil {
		return false
	}
	return c.ProcessState.ExitCode() == 0
}
