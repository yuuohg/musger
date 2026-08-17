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

func scan(scanner *bufio.Scanner, subChan chan<- []byte, iqc chan<- struct{}) {
	for scanner.Scan() {
		subChan <- scanner.Bytes()
	}
	iqc <- struct{}{}
}

func (client *MpvClient) MpvReplies(
	msgChan chan MpvResponse,
	qc chan struct{},
) {
	scanner := bufio.NewScanner(client.conn)
	subChan := make(chan []byte, 5)
	iqc := make(chan struct{})
	go scan(scanner, subChan, iqc)
	buffer := make([]byte, 0, 512)
SCAN:
	for {
		select {
		case msg := <-subChan:
			{
				var response MpvResponse
				buffer = append(buffer, msg...)
				err := json.Unmarshal(buffer, &response)
				if err != nil {
					continue SCAN
				}
				buffer = buffer[:0]
				msgChan <- response
			}
		case <-iqc:
			{
				break SCAN
			}
		case <-qc:
			{
				qc <- struct{}{}
				break SCAN
			}
		}
	}
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
