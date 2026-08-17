package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"syscall"
	"time"

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

func (client *MpvClient) UpdatePulsePath(log bool) bool {
	Logf(ansi.BLUE, "Finding server path")
	path := GetPulsePath()
	if len(path) != 0 {
		client.PulsePath = path
		Logf(ansi.GREEN, "Found server path: %v", path)
		return true
	}
	return false
}

func GetPulsePath() string {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Millisecond*250,
	)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pactl", "-f", "json", "info")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var s PactlInfo
	json.Unmarshal(out, &s)
	if s.ServerString != "" {
		return s.ServerString
	}
	return ""
}

func PulseaudioIsDead(pulsePath string) bool {
	retries := 25
	for {
		if retries == 0 {
			break
		}
		conn, err := net.Dial("unix", pulsePath)
		if err == nil {
			defer conn.Close()
			retries = 15
			for {
				if retries == 0 {
					return true
				}
				fromNow := time.Now().Add(time.Microsecond * 50)
				conn.SetWriteDeadline(fromNow)
				n, e := conn.Write([]byte{0x00})
				if e == nil && n > 0 {
					return false
				}
				retries--
			}
		} else {
			operr, ok := err.(*net.OpError)
			if !ok {
				return true
			}
			sysc, ok := operr.Err.(*os.SyscallError)
			if !ok {
				return true
			}
			errno, ok := sysc.Err.(syscall.Errno)
			if !ok {
				return true
			}
			if errno == syscall.EAGAIN {
				retries--
				continue
			} else {
				return true
			}
		}
	}
	return true
}

func (client *MpvClient) PulseaudioIsDead() bool {
	return PulseaudioIsDead(client.PulsePath)
}

func PulseProcessAlive() bool {
	c := exec.Command("pulseaudio", "--check")
	r := c.Run()
	if r != nil {
		return false
	}
	return c.ProcessState.ExitCode() == 0
}
