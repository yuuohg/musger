package main

import (
	"fmt"
	"log"
	"time"
)

type Empty struct{}

var Nothing = Empty{}

func eventHandler(eventChannel <-chan MpvEvent, playlist *Playlist) {
	if playlist == nil {
		return
	}
	for event := range eventChannel {
		if event.Event == "end-file" && event.Reason == "eof" {
			playlist.Next(true)
		}
		fmt.Printf("%+v\n", event)
	}
}

func DurationDaemon(dc *DaemonChannel, dchan chan uint, req chan Empty) {
	if dc == nil {
		return
	}
	var lastDurationQuery time.Time = time.Now()
	var lastCheckedDuration uint
	tick := time.Tick(time.Millisecond * 100)
	for {
		select {
		case <-req:
			{
				if lastCheckedDuration != 0 {
					g := time.Since(lastDurationQuery)
					dchan <- uint(g.Milliseconds() + int64(lastCheckedDuration))
				} else {
					dchan <- 0
				}
			}
		case <-tick:
			{
				d := dc.CurrentPos()
				lastDurationQuery = time.Now()
				currentDuration, _ := d.Data.(float64)
				lastCheckedDuration = secsToms(currentDuration)
			}
		}
	}
}

func main() {
	dc, _, e, err := SetupDaemon()
	if err != nil {
		log.Fatalln(err.Error())
	}
	p, err := NewAD("audio", &dc)
	if err != nil {
		log.Fatal(err)
	}
	p.RemoveNonAudioFiles()
	go eventHandler(e, &p.Playlist)
	t := time.Tick(time.Millisecond * 1000)
	d := make(chan uint, 2)
	b := make(chan Empty)
	go DurationDaemon(&dc, d, b)
	p.Prev(false)
	for range t {
		b <- Nothing
		dur := <-d
		fmt.Println(dur, " from daemon")
	}
}
