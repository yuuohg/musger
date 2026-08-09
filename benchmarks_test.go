package main_test

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"musger/ipc"
	"musger/lyrics"
	"musger/playlists"
	"musger/ui"

	"github.com/zeebo/xxh3"
)

var sep [4]byte = [4]byte{
	'[',
	']',
	':',
	'.',
}

func IsDigit(r byte) bool {
	return '0' <= r && r <= '9'
}

func isTimestampDigits(timestamp string) bool {
	return (IsDigit(timestamp[1]) && IsDigit(timestamp[2]) &&
		IsDigit(timestamp[4]) && IsDigit(timestamp[5]) &&
		IsDigit(timestamp[7]) && IsDigit(timestamp[8]))
}

func hasValidSeperators(timestamp string) bool {
	return [4]byte{
		timestamp[0],
		timestamp[9],
		timestamp[3],
		timestamp[6],
	} == sep
}

func BenchmarkHash(b *testing.B) {
	s := playlists.Song{Path: "./audio/08 - Cadmium Colors.opus"}
	for b.Loop() {
		s.HashAudio()
	}
	fmt.Println()
	fmt.Println(s.Hash)
}

const sampleSize int64 = 64 * 1024

func BenchmarkRead(b *testing.B) {
	song := playlists.Song{Path: "./audio/08 - Cadmium Colors.opus"}
	for b.Loop() {
		var hasher *xxh3.Hasher = xxh3.New()
		file, err := os.Open(song.Path)
		if err != nil {
			log.Fatalln(err)
		}
		defer file.Close()
		s, err := file.Stat()
		size := s.Size()
		if err != nil {
			log.Fatalln(err)
		}
		if s.Size() < sampleSize*3 {
			io.Copy(hasher, file)
		} else {
			buf := make([]byte, sampleSize)
			_, err := io.ReadFull(file, buf)
			if errors.Is(err, io.ErrUnexpectedEOF) {
				log.Fatalln(err)
			}
			hasher.Write(buf)
			_, err = file.Seek(size/2, io.SeekStart)
			if err != nil {
				log.Fatalln(fmt.Errorf("failed to seek: %w", err))
			}
			_, err = io.ReadFull(file, buf)
			if errors.Is(err, io.ErrUnexpectedEOF) {
				log.Fatalln(err)
			}
			hasher.Write(buf)
			_, err = file.Seek(-sampleSize, io.SeekEnd)
			if err != nil {
				log.Fatalln(fmt.Errorf("failed to seek: %w", err))
			}
			_, err = io.ReadFull(file, buf)
			if errors.Is(err, io.ErrUnexpectedEOF) {
				log.Fatalln(err)
			}
			hasher.Write(buf)
		}
	}
}

func BenchmarkDead(b *testing.B) {
	err := ipc.StartPulse(ui.GeneratePulsePath())
	if err != nil {
		log.Fatalln(err.Error())
	}
	p := ipc.GetPulsePath()
	r := make([]bool, 0, 50_000)
	for b.Loop() {
		r = append(r, ipc.PulseaudioIsDead(p))
	}
	// fmt.Printf("r: %v\n", r)
	numOfTrue := 0
	for _, e := range r {
		if e {
			numOfTrue++
		}
	}
	numOfFalse := len(r) - numOfTrue
	fmt.Printf("numOfTrue: %v\n", numOfTrue)
	fmt.Printf("numOfFalse: %v\n", numOfFalse)
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
		Title:  "D/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リン) - D/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リン)D/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リン) - D/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リンD/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リン) - D/N/A (feat. 宵崎奏 & 朝比奈まふゆ & 東雲絵名 & 暁山瑞希 & 鏡音リン",
		Artist: "53ts",
	}

	for b.Loop() {
		ui.ViewSong(true, 40, &Song)
	}
}

func BenchmarkViewSongHappy(b *testing.B) {
	Song := playlists.Song{
		Path:   "/path/hello.opus",
		Title:  "hikdkf",
		Artist: "53ts",
	}

	for b.Loop() {
		ui.ViewSong(true, 40, &Song)
	}
}

func BenchmarkViewSongAscii(b *testing.B) {
	Song := playlists.Song{
		Path:   "/path/hello.opus",
		Title:  "hikdkfhfjdjdjdjjdhndjjdh",
		Artist: "53ts",
	}

	for b.Loop() {
		ui.ViewSong(true, 10, &Song)
	}
}

func BenchmarkViewSongHappyCJK(b *testing.B) {
	Song := playlists.Song{
		Path:   "/path/hello.opus",
		Title:  "hikdkf宵崎奏 & 朝比奈まふゆ & 東雲絵名宵崎奏 & 朝比奈まふゆ & 東雲絵名宵崎奏 & 朝比奈まふゆ & 東雲絵名宵崎奏 & 朝比奈まふゆ & 東雲絵名",
		Artist: "53ts",
	}

	for b.Loop() {
		ui.ViewSong(true, 40000, &Song)
	}
}

func BenchmarkViewSongCJK(b *testing.B) {
	Song := playlists.Song{
		Path:   "/path/hello.opus",
		Title:  "比奈まふゆ & 東雲絵名宵崎奏 & 朝比奈まふゆ & ",
		Artist: "53ts",
	}
	w := len(Song.Title) - 2

	for b.Loop() {
		ui.ViewSong(true, w, &Song)
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

func BenchmarkDM(b *testing.B) {
	for b.Loop() {
		ui.DisplayMenu(3)
	}
}

func BenchmarkLrcTxtToLrc(b *testing.B) {
	text, _ := playlists.ReadUTF8File(
		"/data/data/com.termux/files/home/mp_lyrics/act_lyrics/Cadmium Colors.txt",
	)
	var e error
	for b.Loop() {
		_, e = lyrics.LrctextToLrc(string(text), 295000)
	}
	if e != nil {
		log.Fatalln(e.Error())
	}
}

func BenchmarkShowLyric(b *testing.B) {
	text, _ := playlists.ReadUTF8File(
		"/data/data/com.termux/files/home/mp_lyrics/act_lyrics/Cadmium Colors.txt",
	)
	v, err := lyrics.LrctextToLrc(string(text), 295000)
	if err != nil {
		log.Fatalln(err.Error())
	}
	for b.Loop() {
		v.ShowLyrics(56788, 10, 10)
	}
}

func BenchmarkGetLyric(b *testing.B) {
	text, _ := playlists.ReadUTF8File(
		"/data/data/com.termux/files/home/mp_lyrics/act_lyrics/Cadmium Colors.txt",
	)
	v, err := lyrics.LrctextToLrc(string(text), 295000)
	if err != nil {
		log.Fatalln(err.Error())
	}
	for b.Loop() {
		v.GetLyricfromTimestamp(56788)
	}
}

func BenchmarkTTMS(b *testing.B) {
	for b.Loop() {
		lyrics.TimestampToMs("[04:09.77]")
	}
}

func BenchmarkLine(b *testing.B) {
	text, _ := playlists.ReadUTF8File(
		"/data/data/com.termux/files/home/mp_lyrics/act_lyrics/Cadmium Colors.txt",
	)
	for b.Loop() {
		iterLines := strings.Lines(string(text))
		var lines []string
		for line := range iterLines {
			line = strings.TrimSpace(line)
			if len(line) >= 10 && hasValidSeperators(line) &&
				isTimestampDigits(line) {
				lines = append(lines, line)
				continue
			}
		}

	}
}
