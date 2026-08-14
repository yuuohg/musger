package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/clipperhouse/uax29/v2/graphemes"
	rw "github.com/mattn/go-runewidth"
)

var (
	runeCache = make(map[rune]int)
	strCache  = make(map[string]int)
)

// taken from github.com/mattn/go-runewidth
func graphemeWidth(cluster string) int {
	width := 0
	for _, r := range cluster {
		width += RuneWidth(r)
	}
	if width > 2 {
		width = 2
	}
	return width
}

func StringWidth(s string) (width int) {
	width, ok := strCache[s]
	if ok {
		return width
	}
	if len(s) == 1 {
		b := s[0]
		if b < 0x20 || b == 0x7F {
			return 0
		}
		return 1
	}
	if len(s) > 0 && len(s) <= utf8.UTFMax {
		r, size := utf8.DecodeRuneInString(s)
		if size == len(s) {
			return RuneWidth(r)
		}
	}
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 0x80 {
			goto graphemes
		}
		if b >= 0x20 && b != 0x7F {
			width++
		}
	}
	strCache[s] = width
	return

graphemes:
	width = 0
	g := graphemes.FromString(s)
	for g.Next() {
		width += graphemeWidth(g.Value())
	}
	strCache[s] = width
	return
}

func RuneWidth(r rune) int {
	width, ok := runeCache[r]
	if ok {
		return width
	}
	width = rw.RuneWidth(r)
	runeCache[r] = width
	return width
}

func Truncate(str string, target int, tail string) string {
	target = target - StringWidth(tail)
	if len(str) < target {
		return str
	}
	var (
		final        strings.Builder
		currentWidth int
	)
	final.Grow(len(str))
	if target <= 0 {
		return final.String()
	}
	for _, ch := range str {
		if ch >= 0x20 && ch < 0x7F {
			currentWidth++
		} else {
			currentWidth += RuneWidth(ch)
		}
		if currentWidth > target {
			break
		}
		final.WriteRune(ch)
	}
	if final.Len() > 0 {
		final.WriteString(tail)
	}
	return final.String()
}
