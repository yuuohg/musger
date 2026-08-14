package ui

import (
	"math"
	"math/rand"
	"os"
)

var characters = []rune(
	"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_",
)

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
