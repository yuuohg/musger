package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"time"

	// "github.com/jfreymuth/pulse/proto"
	"musger/ansi"
)

func StartPulse(pulsePath string) error {
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
		return fmt.Errorf("Couldn't start pulseaudio: %w", err)
	}
	if !pulse.ProcessState.Success() {
		return fmt.Errorf(
			"Couldn't start pulseaudio:\nExit Code: %v\nOutput:\n%v\n",
			pulse.ProcessState.ExitCode(),
			string(o),
		)
	}
	return nil
}

func KillPulse() error {
	pk := exec.Command("pulseaudio", "--kill")
	pk.Run()
	if PulseProcessAlive() {
		k := exec.Command("pkill", "-9", "-f", "pulseaudio.*")
		return k.Run()
	}
	return nil
}

func (client *MpvClient) UpdatePulsePath(log bool) error {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Millisecond*250,
	)
	defer cancel()
	if log {
		Logf(ansi.BLUE, "Finding server path")
	}
	cmd := exec.CommandContext(ctx, "pactl", "-f", "json", "info")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	var s PactlInfo
	json.Unmarshal(out, &s)
	if s.ServerString != "" {
		if log {
			Logf(ansi.GREEN, "Found server path: %v", s.ServerString)
		}
		client.PulsePath = s.ServerString
		return nil
	}
	return nil
}

func (client *MpvClient) PulseaudioIsDead() bool {
	// c := &proto.Client{}
	// c.SetTimeout(time.Millisecond * 10)
	conn, err := net.Dial("unix", client.PulsePath)
	if err != nil {
		return true
	}
	// c.Open(conn)
	//
	// cookiePath := os.Getenv("HOME") + "/.config/pulse/cookie"
	// if path, ok := os.LookupEnv("PULSE_COOKIE"); ok {
	// 	cookiePath = path
	// }
	//
	// cookie, err := os.ReadFile(cookiePath)
	// if err != nil {
	// 	if !os.IsNotExist(err) {
	// 		conn.Close()
	// 		return true
	// 	}
	// 	cookie = make([]byte, 256)
	// }
	// var authReply proto.AuthReply
	// err = c.Request(
	// 	&proto.Auth{
	// 		Version: c.Version(),
	// 		Cookie:  cookie,
	// 	}, &authReply,
	// )
	// if err != nil {
	// 	conn.Close()
	// 	return true
	// }
	conn.Close()
	return false
}

func PulseProcessAlive() bool {
	c := exec.Command("pulseaudio", "--check")
	r := c.Run()
	if r != nil {
		return false
	}
	return c.ProcessState.ExitCode() == 0
}
