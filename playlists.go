package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"unicode/utf8"

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

type PlaylistJSON struct {
	Songs []string `json:"songs"`
	Name  string   `json:"name"`
}

type MUGRFile struct {
	Loop      int      `json:"loop"`
	Playlists []string `json:"playlists"`
	Queue     string   `json:"queue"`
}

func (mug *MUGRFile) Save(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	b, err := json.Marshal(mug)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func (mug *MUGRFile) Load(file string) error {
	contents, err := ReadUTF8File(file)
	if err != nil {
		return err
	}
	if !json.Valid(contents) {
		return fmt.Errorf("Invaild json in %v", file)
	}
	json.Unmarshal(contents, mug)
	return nil
}

func ReadUTF8File(name string) ([]byte, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("Could not open %v: %w", name, err)
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("Could not stat %v: %w", name, err)
	}
	if stat.IsDir() {
		return nil, fmt.Errorf("%v is a directory", name)
	}
	contents := make([]byte, stat.Size())
	_, err = file.Read(contents)
	if err != nil {
		return nil, fmt.Errorf("Could not read from %v: %w", name, err)
	}
	isVaildUTF8 := utf8.Valid(contents)
	if !isVaildUTF8 {
		return nil, fmt.Errorf("%v does not have valid utf8", name)
	}
	return contents, nil
}

type Playlist struct {
	name     string
	Files    []string
	dc       *DaemonChannel
	currSong int
}

func (p *Playlist) RemoveNonAudioFiles() {
	if len(p.Files) == 0 {
		return
	}
	removal := make([]int, 0)
	for idx, filename := range p.Files {
		if !isAudio(filename) {
			removal = append(removal, idx)
		}
	}
	for i, idx := range removal {
		p.Files = remove(p.Files, idx-i)
	}
}

func (p *Playlist) Next(wrap bool) (MpvResponse, error) {
	if len(p.Files) == 0 {
		return MpvResponse{}, fmt.Errorf("Empty playlist")
	}
	p.currSong++
	isAtEnd := p.currSong == len(p.Files)
	if wrap && isAtEnd {
		p.currSong = 0
	} else if isAtEnd {
		p.currSong--
	}
	return p.dc.PlayFile(p.Files[p.currSong]), nil
}

func (p *Playlist) Prev(wrap bool) (MpvResponse, error) {
	if len(p.Files) == 0 {
		return MpvResponse{}, fmt.Errorf("Empty playlist")
	}
	p.currSong--
	isAtStart := p.currSong == -1
	if isAtStart && wrap {
		p.currSong = len(p.Files) - 1
	} else if isAtStart {
		p.currSong++
	}
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
	pj := PlaylistJSON{
		Name:  p.name,
		Songs: p.Files,
	}
	b, err := json.Marshal(pj)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err != nil {
		return err
	}
	return nil
}

func (p *Playlist) Load(file string) error {
	contents, err := ReadUTF8File(file)
	if err != nil {
		return err
	}
	if !json.Valid(contents) {
		return fmt.Errorf("Invaild json in %v", file)
	}
	pJson := PlaylistJSON{
		Name:  "",
		Songs: make([]string, 0),
	}
	json.Unmarshal(contents, &pJson)
	if pJson.Name == "" && len(pJson.Songs) == 0 {
		return fmt.Errorf("%v does not have 'name' and 'songs' field", file)
	}
	p.name = pJson.Name
	p.Files = pJson.Songs
	p.RemoveNonAudioFiles()
	return nil
}

func (p *Playlist) Moveto(idx int, destidx int) error {
	if idx >= len(p.Files) || destidx >= len(p.Files) || idx < 0 ||
		destidx < 0 {
		return fmt.Errorf("out of range")
	}
	s := p.Files[idx]
	p.Files = remove(p.Files, idx)
	p.Files = slices.Insert(p.Files, destidx, s)
	return nil
}

func (p *Playlist) AddPlaylist(other Playlist) {
	p.Files = append(p.Files, other.Files...)
}

func (p *Playlist) RemoveDuplicates() {
	nd := make([]string, 0, len(p.Files))
	for _, file := range p.Files {
		if !slices.Contains(nd, file) {
			nd = append(nd, file)
		}
	}
	p.Files = nd
}

func NewAD(dir string, dc *DaemonChannel) (Playlist, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Playlist{dc: dc}, err
	}
	os.Chdir(dir)
	audioDir := Playlist{dir, make([]string, 0, 5), dc, 0}
	if len(entries) == 0 {
		return audioDir, nil
	}
	files := make([]string, 0, 5)
	for _, f := range entries {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		files = append(files, name)
	}
	audioDir.Files = files
	audioDir.RemoveNonAudioFiles()
	audioDir.ExpandToAbsPath()
	err = os.Chdir("..")
	if err != nil {
		return Playlist{dc: dc}, err
	}
	return audioDir, nil
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
