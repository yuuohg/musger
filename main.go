package main

import (
	"fmt"
	"log"
	"math"
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
		d := dc.Duration().Data
		currpos := dc.CurrentPos().Data
		df, _ := d.(float64)
		currposf, _ := currpos.(float64)
		fmt.Printf("duration: %v\n", math.Round(df*1000))
		fmt.Printf("CurrentPos: %v\n", math.Round(currposf*1000))
		i--
	}
	fmt.Println("done.")
}
