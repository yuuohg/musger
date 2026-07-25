package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	. "musger/ansi"

	"github.com/jfreymuth/pulse"
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

func (client *MpvClient) KillPulse() error {
	pk := exec.Command("pulseaudio", "--kill")
	pk.Run()
	if PulseProcessAlive() {
		k := exec.Command("pkill", "-9", "-f", "pulseaudio.*")
		return k.Run()
	}
	return nil
}

func (client *MpvClient) UpdatePulsePath(log bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	if log {
		Logf(BLUE, "Finding server path")
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
			Logf(GREEN, "Found server path: %v", s.ServerString)
		}
		client.PulsePath = s.ServerString
		return nil
	}
	return nil
}

func (client *MpvClient) PulseaudioIsDead() bool {
	c, e := pulse.NewClient(
		pulse.ClientServerString("unix:" + client.PulsePath),
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
