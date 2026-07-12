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
	dc, q, e, err := SetupDaemon()
	if err != nil {
		log.Fatalln(err.Error())
	}
	go eventReciever(e)
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
		}
		fmt.Println(p.Next(true))
		time.Sleep(time.Second * 3)
		i--
	}
	fmt.Println("done.")
}
