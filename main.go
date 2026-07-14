package main

import (
	"fmt"
	"log"
	"time"
)

func eventHandler(ec chan MpvEvent) {
	for e := range ec {
		fmt.Printf("%+v\n", e)
	}
}

func DurationDaemon(dc *DaemonChannel, dchan chan uint) {
	if dc == nil {
		return
	}
	select {
	case <-dchan:
		{
			// todo
		}
	}
}

func main() {
	dc, q, e, err := SetupDaemon()
	if err != nil {
		log.Fatalln(err.Error())
	}
	go eventHandler(e)
	p, err := NewAD("audio", &dc)
	if err != nil {
		log.Fatal(err)
	}
	a := true
	i := 5
	p.RemoveNonAudioFiles()
	for {
		if i == 0 {
			q <- true
			<-q
			break
		}
		if a {
			fmt.Println(p.Prev(false))
			a = false
			time.Sleep(time.Second * 10)
			continue
		}
		fmt.Println(p.Next(true))
		time.Sleep(time.Second * 10)
		i--
	}
	fmt.Println("done.")
}
