package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

func remove[T any](slice []T, s int) []T {
	if len(slice) == 0 {
		return slice
	}
	if len(slice) == 1 {
		return make([]T, 0)
	}
	return append(slice[:s], slice[s+1:]...)
}

type Playlist struct {
	Files    []string
	dc       *DaemonChannel
	currSong int
}

func (p *Playlist) RemoveEmptyStrings() {
	for idx, file := range p.Files {
		if file == "" {
			p.Files = remove(p.Files, idx)
		}
	}
}

func (p *Playlist) RemoveNonAudioFiles() error {
	p.RemoveEmptyStrings()
	f := strings.Join(p.Files, " ")
	r, e := exec.Command("file", "--mime-type", "-b", f).Output()
	if e != nil {
		return e
	}
	types := slices.Collect(strings.Lines(string(r)))
	for i, v := range types {
		if strings.Contains(v, "audio") {
			continue
		}
		p.Files = remove(p.Files, i)
	}
	return nil
}

func (p *Playlist) Next(wrap bool) (MpvResponse, error) {
	p.RemoveNonAudioFiles()
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
	p.RemoveNonAudioFiles()
	if len(p.Files) == 0 {
		return MpvResponse{}, fmt.Errorf("Empty playlist")
	}
	p.currSong++
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

func (p *Playlist) Save(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	f.WriteString(strings.Join(p.Files, "\n"))
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
	comm := make([]string, 0)
	comm = append(comm, "--mime-type", "-b")
	comm = append(comm, files...)
	r, e := exec.Command("file", comm...).Output()
	if e != nil {
		return AudioDir{}, e
	}
	types := slices.Collect(strings.Lines(string(r)))
	m := make(map[string]string)
	for i := 0; i < len(files); i++ {
		m[files[i]] = types[i]
	}
	for k, v := range m {
		if strings.Contains(v, "audio") {
			ad.Files = append(ad.Files, dir+"/"+k)
		}
	}
	err = os.Chdir("..")
	if err != nil {
		return AudioDir{}, err
	}
	return ad, nil
}

func isAudio(file string) bool {
	f, e := exec.Command("file", "--mime-type", "-b", file).Output()
	if e != nil {
		return false
	}
	return bytes.Contains(f, []byte("audio"))
}
