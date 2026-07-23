package main

import (
	"log"

	tea "charm.land/bubbletea/v2"
)

type Empty struct{}

var Nothing = Empty{}

func main() {
	model, quitChan, c, err := initModel()
	if err != nil {
		log.Fatalf(
			"%vError when trying to initalize mpv/pulseaudio: %v%v\n",
			RED,
			err.Error(),
			RESET,
		)
	}
	c.UpdatePulsePath()
	p := tea.NewProgram(model)
	p.Run()
	quitChan <- Nothing
	<-quitChan
	c.Close()
}
