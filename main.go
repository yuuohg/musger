package main

import (
	"fmt"
	"log"
	"time"
)

func eventReciever(ec chan MpvEvent) {
	for e := range ec {
		fmt.Printf("%+v\n", e)
	}
}

func main() {
	dc, quitChan, e, err := SetupDaemon()
	if err != nil {
		log.Fatalln(err.Error())
	}
	go eventReciever(e)
	resp := dc.PlayFile("music.opus")
	fmt.Printf("resp: %+v\n", resp)
	time.Sleep(time.Second * 259)
	quitChan <- 1
	fmt.Printf("quitChan <- 1\n")
	<-quitChan
	fmt.Printf("<-quitChan\n")
}
