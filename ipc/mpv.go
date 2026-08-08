package ipc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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

func (mpvc *MpvClient) Close() error {
	err := mpvc.conn.Close()
	if err != nil {
		return err
	}
	os.Remove(mpvc.path)
	mpvc.mpvCmd.Process.Kill()
	return nil
}
