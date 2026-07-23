package main_test

import (
	"fmt"
	"log"
	"testing"

	"musger"

	"github.com/zeebo/xxh3"
)

func BenchmarkHash(b *testing.B) {
	h := xxh3.New()
	s := main.Song{Path: "./audio/08 - Cadmium Colors.opus"}
	for b.Loop() {
		s.Hash.Reset()
		s.HashAudio(h)
	}
	fmt.Println(s.Hash.String())
}

func BenchmarkDead(b *testing.B) {
	c, err := main.InitServer(main.GeneratePath(), main.GeneratePulsePath())
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
	play, err := main.NewAD("audio")
	if err != nil {
		log.Fatalln(err.Error())
	}
	for b.Loop() {
		play.ShufflePlaylist()
	}
}
