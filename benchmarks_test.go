package main_test

import (
	"fmt"
	"io"
	"log"
	"os"
	"testing"

	"musger/ipc"
	"musger/playlists"
	"musger/ui"

	"github.com/zeebo/xxh3"
)

func BenchmarkHash(b *testing.B) {
	s := playlists.Song{Path: "./audio/08 - Cadmium Colors.opus"}
	for b.Loop() {
		s.HashAudio()
	}
	fmt.Println(s.Hash)
}

func BenchmarkRead(b *testing.B) {
	s := playlists.Song{Path: "./audio/08 - Cadmium Colors.opus"}
	for b.Loop() {
		var hasher *xxh3.Hasher = xxh3.New()
		file, err := os.Open(s.Path)
		if err != nil {
			return
		}
		defer file.Close()
		_, err = file.Stat()
		if err != nil {
			return
		}
		io.Copy(hasher, file)
	}
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

func BenchmarkViewSong(b *testing.B) {
	Song := playlists.Song{
		Path:   "/path/hello.opus",
		Title:  "D/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リン) - D/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リン)",
		Artist: "53ts",
	}
	for b.Loop() {
		ui.ViewSong(true, 40, &Song)
	}
}

func BenchmarkCurrState(b *testing.B) {
	ad, err := playlists.NewAD("audio")
	if err != nil {
		log.Fatalln(err.Error())
	}
	for b.Loop() {
		ui.CurrStateAsStr(
			102,
			27,
			27,
			&ad,
		)
	}
}

func BenchmarkMstoRead(b *testing.B) {
	for b.Loop() {
		ui.MstoReadable(837834)
	}
}
