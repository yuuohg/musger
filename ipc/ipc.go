package ipc

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	. "musger/ansi"
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
	PulsePath string
}

type MpvResponse struct {
	Data            any    `json:"data,omitempty"`
	Error           string `json:"error,omitempty"`
	FileError       string `json:"file_error,omitempty"`
	Event           string `json:"event,omitempty"`
	Reason          string `json:"reason,omitempty"`
	RequestID       int64  `json:"request_id,omitempty"`
	Text            string `json:"text,omitempty"`
	Name            string `json:"name,omitempty"`
	Level           string `json:"level,omitempty"`
	Prefix          string `json:"prefix,omitempty"`
	PlaylistEntryID int64  `json:"playlist_entry_id,omitempty"`
	originalJson    string
}

type PactlInfo struct {
	ServerString string `json:"server_string,omitempty"`
}

func InitIpc(path string, pulsePath string) (*MpvClient, error) {
	var err error
	ipcServerOption := fmt.Sprintf("--input-ipc-server=%v", path)
	Logf(BLUE, "Socket: %v", path)
	if !PulseProcessAlive() {
		Logf(BLUE, "Pulseaudio not running, starting")
		err = StartPulse(pulsePath)
		if err != nil {
			return nil, err
		}
		Logf(GREEN, "Pulseaudio started")
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
	Logf(BLUE, "Starting mpv")
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
	Logf(GREEN, "mpv started, took %.2fs", float64(i)/1000)
	return &MpvClient{path, conn, cmd, pulsePath}, nil
}

func Logf(color, format string, a ...any) {
	fmt.Printf(color+format+RESET+"\n", a...)
}
