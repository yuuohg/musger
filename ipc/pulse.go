package ipc

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	. "musger/ansi"

	"github.com/jfreymuth/pulse"
)

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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()
	Logf(BLUE, "Finding server path")
	cmd := exec.CommandContext(ctx, "pactl", "-f", "json", "info")
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	var s PactlInfo
	json.Unmarshal(out, &s)
	if s.ServerString != "" {
		Logf(GREEN, "Found server path: %v", s.ServerString)
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
