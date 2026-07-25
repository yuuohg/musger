package main

import (
	"log"

	"musger/ipc"
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
	retries := 5
	for {
		err = c.UpdatePulsePath(true)
		if err == nil {
			break
		}
		if retries == 0 {
			log.Fatalf(
				"%vCould not get pulseaudio path%v\n",
				RED,
				RESET,
			)
		}
		c.KillPulse()
		ipc.StartPulse(ui.GeneratePulsePath())
		retries--
	}
	p := tea.NewProgram(model)
	p.Run()
	quitChan <- Nothing
	<-quitChan
	c.Close()
}
