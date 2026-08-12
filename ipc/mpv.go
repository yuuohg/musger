package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

const (
	Play             = `{"command":["set_property","pause",false]}`
	Pause            = `{"command":["set_property","pause",true]}`
	SeekForwardFive  = `{"command":["seek",5]}`
	SeekBackwardFive = `{"command":["seek",-5]}`
	Stop             = `{"command":["stop"]}`
)

func (client *MpvClient) SendCommand(command string) {
	fmt.Fprintln(client.conn, strings.TrimSpace(command))
}

func Loadfile(path string) string {
	return `{"command":["loadfile","` + path + `"]}`
}

func ObserveProperty(property string) string {
	return `{"command":["observe_property",1,"` + property + `"]}`
}

func TogglePlay(pause bool) string {
	if pause {
		return Play
	} else {
		return Pause
	}
}

func (client *MpvClient) MpvReplies(
	msgChan chan MpvResponse,
	qc chan struct{},
) {
	scanner := bufio.NewScanner(client.conn)
SCAN:
	for scanner.Scan() {
		select {
		case _ = <-qc:
			{
				qc <- struct{}{}
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
	qc <- struct{}{}
}

func (client *MpvClient) Close() error {
	err := client.conn.Close()
	if err != nil {
		return err
	}
	os.Remove(client.path)
	client.mpvCmd.Process.Kill()
	return nil
}
