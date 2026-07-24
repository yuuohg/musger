package playlists

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	. "musger/ipc"
	. "musger/lyrics"

	"github.com/gabriel-vasile/mimetype"
	"github.com/zeebo/xxh3"
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

type MUGRFile struct {
	Loop         int                     `json:"loop"`
	SongMetadata map[string]SongMetadata `json:"song_metadata"`
	Playlists    map[string][]string     `json:"playlists"`
	Queue        []string                `json:"queue"`
	LastPlayed   int                     `json:"last_played"`
}

type SongMetadata struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	Lyrics      string `json:"lyrics,omitempty"`
}

type Song struct {
	Path        string
	Title       string
	Artist      string
	AlbumArtist string
	Hash        strings.Builder
	Lyricpath   string
	Lrc         Lrc
	FromHash    bool
}

func (song *Song) HashAudio(hasher *xxh3.Hasher) error {
	file, err := os.Open(song.Path)
	if err != nil {
		return err
	}
	defer file.Close()
	s, err := file.Stat()
	if err != nil {
		return err
	}
	io.Copy(hasher, file)
	fmt.Fprintf(&song.Hash, "%v-%x", s.Size(), hasher.Sum128().Bytes())
	hasher.Reset()
	return nil
}

func (s *Song) isAudio() bool {
	t, err := mimetype.DetectFile(s.Path)
	if err != nil {
		return false
	}
	ty := t.String()
	if len(ty) >= 6 && ty[:6] == "audio/" {
		return true
	}
	if ty == "application/octet-stream" {
		f, e := exec.Command("file", "--mime-type", "-b", s.Path).Output()
		if e != nil {
			return false
		}
		return bytes.Contains(f, []byte("audio/"))
	}
	return false
}

func (song *Song) Load(client *MpvClient) *Song {
	if client.PulseaudioIsDead() {
		client.KillPulse()
	}
	client.SendCommand(Loadfile(song.Path))
	return song
}

func (song *Song) AddMetadata(metadata SongMetadata) {
	song.Title = metadata.Title
	song.Artist = metadata.Artist
	song.AlbumArtist = metadata.AlbumArtist
	song.Lyricpath = metadata.Lyrics
}

func (song *Song) GetLyrics() error {
	text, err := ReadUTF8File(song.Lyricpath)
	if err != nil {
		return err
	}
	lrc, err := LrctextToLrc(string(text))
	if err != nil {
		return err
	}
	song.Lrc = lrc
	return nil
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
	jsonNotVaild := !json.Valid(contents)
	if jsonNotVaild {
		return fmt.Errorf("Invaild json in %v", file)
	}
	json.Unmarshal(contents, mug)
	emptyMUGR := &MUGRFile{}
	if mug == emptyMUGR {
		return fmt.Errorf("%v has no useful data", file)
	}
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
	notVaildUTF8 := !utf8.Valid(contents)
	if notVaildUTF8 {
		return nil, fmt.Errorf("%v does not have valid utf8", name)
	}
	return contents, nil
}

type Playlist struct {
	Name          string
	Songs         []Song
	ShuffledSongs []int
	IsShuffled    bool
	CurrSong      int
}

func (p *Playlist) ShufflePlaylist() {
	if len(p.Songs) == 0 {
		return
	}
	p.ShuffledSongs[0] = p.CurrSong
	if len(p.Songs) == 1 {
		return
	}
	pool := make([]int, 0, len(p.Songs)-1)
	for i := range len(p.Songs) {
		if i == p.CurrSong {
			continue
		}
		pool = append(pool, i)
	}
	var i int = 1
	for len(pool) != 0 {
		poolIdx := rand.Intn(len(pool))
		p.ShuffledSongs[i] = pool[poolIdx]
		pool = remove(pool, poolIdx)
		i++
	}
	p.CurrSong = 0
	p.IsShuffled = true
}

func (p *Playlist) RemoveNonAudioFiles() {
	if len(p.Songs) == 0 {
		return
	}
	removal := make([]int, 0)
	for idx, song := range p.Songs {
		if !song.isAudio() {
			removal = append(removal, idx)
		}
	}
	for i, idx := range removal {
		p.Songs = remove(p.Songs, idx-i)
	}
}

func (p *Playlist) Next(wrap bool) (int, error) {
	if len(p.Songs) == 0 {
		return 0, fmt.Errorf("Empty playlist")
	}
	p.CurrSong++
	isAtEnd := p.CurrSong >= len(p.Songs)
	if wrap && isAtEnd {
		p.CurrSong = 0
	} else if isAtEnd {
		p.CurrSong = len(p.Songs)
	}
	return p.CurrSong, nil
}

func (p *Playlist) Prev(wrap bool) (int, error) {
	if len(p.Songs) == 0 {
		return 0, fmt.Errorf("Empty playlist")
	}
	p.CurrSong--
	isAtStart := p.CurrSong < 0
	if isAtStart && wrap {
		p.CurrSong = len(p.Songs) - 1
	} else if isAtStart {
		p.CurrSong = 0
	}
	return p.CurrSong, nil
}

func (p *Playlist) ExpandToAbsPath() error {
	for i, f := range p.Songs {
		abs, err := filepath.Abs(f.Path)
		if err != nil {
			return err
		}
		p.Songs[i].Path = abs
	}
	return nil
}

func (p *Playlist) HashAudioFiles() {
	if len(p.Songs) == 0 {
		return
	}
	hasher := xxh3.New()
	for _, song := range p.Songs {
		song.HashAudio(hasher)
	}
}

func (p *Playlist) Save() (string, []string, error) {
	err := p.ExpandToAbsPath()
	if err != nil {
		return "", nil, err
	}
	files := make([]string, 0, len(p.Songs))
	for _, song := range p.Songs {
		if song.isAudio() {
			files = append(files, song.Path)
		}
	}
	return p.Name, files, nil
}

func Load(name string, files []string) Playlist {
	var playlist Playlist
	playlist.Name = name
	for _, file := range files {
		playlist.Songs = append(playlist.Songs, Song{Path: file})
	}
	playlist.RemoveNonAudioFiles()
	playlist.RemoveDuplicates()
	playlist.ExpandToAbsPath()
	return playlist
}

func (p *Playlist) Moveto(idx int, destidx int) error {
	if idx >= len(p.Songs) || destidx >= len(p.Songs) || idx < 0 ||
		destidx < 0 {
		return fmt.Errorf("out of range")
	}
	s := p.Songs[idx]
	p.Songs = remove(p.Songs, idx)
	p.Songs = slices.Insert(p.Songs, destidx, s)
	return nil
}

func (p *Playlist) AddPlaylist(other Playlist) {
	p.Songs = append(p.Songs, other.Songs...)
	p.AllocateShuffle()
}

func (p *Playlist) AddSong(song Song) {
	p.Songs = append(p.Songs, song)
	p.AllocateShuffle()
}

func (p *Playlist) AllocateShuffle() {
	if !p.IsShuffled && len(p.Songs) != len(p.ShuffledSongs) {
		p.ShuffledSongs = make([]int, len(p.Songs))
	}
}

func (p *Playlist) RemoveDuplicates() {
	nd := make([]Song, 0, len(p.Songs))
D:
	for _, file := range p.Songs {
		for _, s := range nd {
			if file.Path == s.Path {
				continue D
			}
		}
		nd = append(nd, file)
	}
	p.Songs = nd
}

func NewAD(dir string) (Playlist, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return Playlist{}, err
	}
	os.Chdir(dir)
	audioDir := Playlist{Name: dir, Songs: make([]Song, 0, 5)}
	if len(entries) == 0 {
		return audioDir, nil
	}
	files := make([]Song, 0, 5)
	for _, f := range entries {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		files = append(files, Song{Path: name})
	}
	audioDir.Songs = files
	audioDir.RemoveNonAudioFiles()
	audioDir.ExpandToAbsPath()
	err = os.Chdir("..")
	if err != nil {
		return Playlist{}, err
	}
	audioDir.ShuffledSongs = make([]int, len(audioDir.Songs))
	go audioDir.HashAudioFiles()
	return audioDir, nil
}
