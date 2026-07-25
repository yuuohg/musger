package main

import (
	"log"

	"musger/ui"

	tea "charm.land/bubbletea/v2"
)

var Nothing = struct{}{}

func main() {
	model, quitChan, c, err := ui.InitModel()
	if err != nil {
		log.Fatalf(
			"%vError when trying to initalize mpv/pulseaudio: %v%v\n",
			RED,
			err.Error(),
			RESET,
		)
	}
	c.UpdatePulsePath(true)
	p := tea.NewProgram(model)
	p.Run()
	quitChan <- Nothing
	<-quitChan
	c.Close()
}
