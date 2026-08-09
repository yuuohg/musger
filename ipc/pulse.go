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
	// c := &proto.Client{}
	// c.SetTimeout(time.Millisecond * 10)
	retries := 5
	for {
		if retries == 0 {
			break
		}
		conn, err := net.Dial("unix", pulsePath)
		if err == nil {
			fromNow := time.Now().Add(time.Microsecond * 300)
			conn.SetWriteDeadline(fromNow)
			_, e := conn.Write([]byte{0x00})
			if e != nil {
				return true
			}
			return false
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
			}
		}
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
