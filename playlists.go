package main

import (
	"bytes"
	"os"
	"os/exec"
	"slices"
	"strings"
)

type AudioDir struct {
	AudioFiles []string
	dir        string
}

func NewAD(d string) (AudioDir, error) {
	entries, err := os.ReadDir(d)
	if err != nil {
		return AudioDir{}, err
	}
	os.Chdir(d)
	ad := AudioDir{make([]string, 0, 5), d}
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
	f := strings.Join(files, " ")
	r, e := exec.Command("file", "--mime-type", "-b", f).Output()
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
			ad.AudioFiles = append(ad.AudioFiles, k)
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
