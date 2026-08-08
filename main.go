package main

import (
	"log"

	"musger/ansi"
	"musger/ipc"
	"musger/ui"

	tea "charm.land/bubbletea/v2"
)

var nothing = struct{}{}

func main() {
	model, quitChan, c, err := ui.InitModel()
	if err != nil {
		log.Fatalf(
			"%vError when trying to initalize mpv/pulseaudio: %v%v\n",
			ansi.RED,
			err.Error(),
			ansi.RESET,
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
				ansi.RED,
				ansi.RESET,
			)
		}
		ipc.KillPulse()
		ipc.StartPulse(ui.GeneratePulsePath())
		retries--
	}
	program := tea.NewProgram(model)
	program.Run()
	quitChan <- nothing
	<-quitChan
	c.Close()
}
