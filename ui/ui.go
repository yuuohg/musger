package ui

import (
	"math"
	"math/rand"
	"os"
	"strings"
	"unicode/utf8"

	"musger/ansi"
)

var characters = []rune(
	"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_",
)

type ProgressBarOptions struct {
	FilledColor   string
	UnfilledColor string
	Width         int
	FilledChar    rune
	UnfilledChar  rune
}

// function below is ai-generated
func calculateLaLb(
	lookAhead, lookBehind, arrayLen, currentIdx int,
) (lB, lA int) {
	if arrayLen <= 1 {
		return 0, 0
	}
	availBehind := currentIdx
	availAhead := (arrayLen - 1) - currentIdx
	lB = min(lookBehind, availBehind)
	lA = min(lookAhead, availAhead)
	if unusedB := lookBehind - lB; unusedB > 0 {
		lA = min(availAhead, lA+unusedB)
	} else if unusedA := lookAhead - lA; unusedA > 0 {
		lB = min(availBehind, lB+unusedA)
	}
	return lB, lA
}

func randSeq(n int) string {
	b := make([]rune, n)
	for idx := range b {
		b[idx] = characters[rand.Intn(len(characters))]
	}
	return string(b)
}

func GeneratePath() string {
	rt := os.ExpandEnv("$TMPDIR")
	if rt == "" {
		return "/usr/tmp/" + randSeq(48) + "_mpv.sock"
	}
	return rt + "/" + randSeq(48) + "_mpv.sock"
}

func GeneratePulsePath() string {
	rt := os.ExpandEnv("$TMPDIR")
	if rt == "" {
		return "/usr/tmp/" + "pulse.sock"
	}
	return rt + "/" + "pulse.sock"
}

func secsToms(s float64) uint64 {
	return uint64(math.Round(s * 1000))
}

func Center(s string, width int) string {
	return strings.Repeat(" ", (width-StringWidth(s))>>1) + s
}

func ProgressBar(
	progress float64, options *ProgressBarOptions,
) string {
	var progressBar strings.Builder
	progress = min(max(progress, 0), 1)
	fcw := RuneWidth(options.FilledChar)
	ufcw := RuneWidth(options.UnfilledChar)
	filled := int(float64(options.Width)*progress) / fcw
	unfilled := (options.Width - filled*fcw) / ufcw
	var filledRune [4]byte
	var unfilledRune [4]byte
	lFB := utf8.EncodeRune(filledRune[:], options.FilledChar)
	lUFB := utf8.EncodeRune(unfilledRune[:], options.UnfilledChar)
	filledByte, unfilledByte := filledRune[:lFB], unfilledRune[:lUFB]
	if unfilled == 1 && progress >= 0.995 {
		filled++
		unfilled--
	}
	progressBar.Grow(
		filled*len(
			filledByte,
		) + unfilled*len(
			unfilledByte,
		) + len(
			options.FilledColor,
		) + len(
			options.UnfilledColor,
		) + 8,
	)
	progressBar.WriteString(options.FilledColor)
	for range filled {
		progressBar.Write(filledByte)
	}
	progressBar.WriteString(ansi.RESET)
	progressBar.WriteString(options.UnfilledColor)
	for range unfilled {
		progressBar.Write(unfilledByte)
	}
	progressBar.WriteString(ansi.RESET)
	return progressBar.String()
}
