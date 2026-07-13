package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gabriel-vasile/mimetype"
)

func remove[T any](slice []T, s int) []T {
	if len(slice) == 0 {
		return slice
	}
	if len(slice) == 1 {
		return make([]T, 0)
	}
	if s == len(slice)-1 {
		return slice[:len(slice)-1]
	}
	if s == 0 {
		return slice[1:]
	}
	return append(slice[:s], slice[s+1:]...)
}

type Playlist struct {
	Files    []string
	dc       *DaemonChannel
	currSong int
}

func (p *Playlist) RemoveNonAudioFiles() error {
	if len(p.Files) == 0 {
		return nil
	}
	removal := make([]int, 0)
	for idx, filename := range p.Files {
		if isAudio(filename) {
			continue
		}
		removal = append(removal, idx)
	}
	for i, idx := range removal {
		p.Files = remove(p.Files, idx-i)
	}
	return nil
}

func (p *Playlist) Next(wrap bool) (MpvResponse, error) {
	if len(p.Files) == 0 {
		return MpvResponse{}, fmt.Errorf("Empty playlist")
	}
	p.currSong++
	if p.currSong != len(p.Files) {
		return p.dc.PlayFile(p.Files[p.currSong]), nil
	}
	if wrap {
		p.currSong = 0
		return p.dc.PlayFile(p.Files[p.currSong]), nil
	}
	p.currSong--
	return p.dc.PlayFile(p.Files[p.currSong]), nil
}

func (p *Playlist) Prev(wrap bool) (MpvResponse, error) {
	if len(p.Files) == 0 {
		return MpvResponse{}, fmt.Errorf("Empty playlist")
	}
	p.currSong--
	if p.currSong != -1 {
		return p.dc.PlayFile(p.Files[p.currSong]), nil
	}
	if wrap {
		p.currSong = len(p.Files) - 1
		return p.dc.PlayFile(p.Files[p.currSong]), nil
	}
	p.currSong++
	return p.dc.PlayFile(p.Files[p.currSong]), nil
}

func (p *Playlist) ExpandToAbsPath() error {
	for i, f := range p.Files {
		abs, err := filepath.Abs(f)
		if err != nil {
			return err
		}
		p.Files[i] = abs
	}
	return nil
}

func (p *Playlist) Save(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	err = p.ExpandToAbsPath()
	if err != nil {
		return err
	}
	f.WriteString(strings.Join(p.Files, "\n"))
	return nil
}

func (p *Playlist) Load(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	s, err := f.Stat()
	if err != nil {
		return err
	}
	if s.IsDir() {
		return fmt.Errorf("oh, come on")
	}
	buf := make([]byte, s.Size())
	_, err = f.Read(buf)
	if err != nil {
		return err
	}
	audioFiles := strings.Lines(string(buf))
	for file := range audioFiles {
		file = strings.TrimSpace(file)
		if file == "" {
			continue
		}
		p.Files = append(p.Files, file)
	}
	p.RemoveNonAudioFiles()
	return nil
}

// moves to destidx+1
func (p *Playlist) Moveto(idx int, destidx int) error {
	if idx >= len(p.Files) || destidx >= len(p.Files) || idx < 0 ||
		destidx < 0 {
		return fmt.Errorf("out of range")
	}
	s := p.Files[idx]
	p.Files = remove(p.Files, idx)
	p.Files = slices.Insert(p.Files, int(destidx), s)
	return nil
}

type AudioDir struct {
	Playlist
	dir string
}

func NewAD(dir string, dc *DaemonChannel) (AudioDir, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return AudioDir{}, err
	}
	os.Chdir(dir)
	ad := AudioDir{Playlist{make([]string, 0, 5), dc, 0}, dir}
	if len(entries) == 0 {
		return ad, nil
	}
	files := make([]string, 0, 5)
	for _, f := range entries {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		files = append(files, name)
	}
	ad.Files = files
	ad.RemoveNonAudioFiles()
	ad.ExpandToAbsPath()
	err = os.Chdir("..")
	if err != nil {
		return AudioDir{}, err
	}
	return ad, nil
}

func isAudio(file string) bool {
	t, err := mimetype.DetectFile(file)
	if err != nil {
		return false
	}
	ty := t.String()
	if len(ty) >= 6 && ty[:6] == "audio/" {
		return true
	}
	// fallback to file(1)
	if ty == "application/octet-stream" {
		f, e := exec.Command("file", "--mime-type", "-b", file).Output()
		if e != nil {
			return false
		}
		return bytes.Contains(f, []byte("audio/"))
	}
	return false
}
