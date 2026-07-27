package main_test

import (
	"fmt"
	"log"
	"testing"

	"musger/ipc"
	"musger/playlists"
	"musger/ui"

	"github.com/zeebo/xxh3"
)

func BenchmarkHash(b *testing.B) {
	h := xxh3.New()
	s := playlists.Song{Path: "./audio/08 - Cadmium Colors.opus"}
	for b.Loop() {
		s.Hash.Reset()
		s.HashAudio(h)
	}
	fmt.Println(s.Hash.String())
}

func BenchmarkDead(b *testing.B) {
	c, err := ipc.InitIpc(ui.GeneratePath(), ui.GeneratePulsePath())
	if err != nil {
		log.Fatalln(err.Error())
	}
	var r bool
	for b.Loop() {
		r = c.PulseaudioIsDead()
	}
	fmt.Println(r)
}

func BenchmarkShuffle(b *testing.B) {
	play, err := playlists.NewAD("audio")
	if err != nil {
		log.Fatalln(err.Error())
	}
	for b.Loop() {
		play.ShufflePlaylist()
	}
}
