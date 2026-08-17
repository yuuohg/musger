package lyrics

import (
	"fmt"
	"strings"

	"musger/ansi"
)

const NEWLINE byte = 10

type compareNum struct {
	lower, upper uint64
}

type Lyric struct {
	lyric      string
	timestamps compareNum
}

type Lrc struct {
	lyrics []Lyric
}

type LrcEmpty struct{}

func (LrcEmpty) Error() string {
	return "no lrc lyric"
}

// function below is ai-generated
func calculateLaLb(
	lookAhead, lookBehind, arrayLen, currentIdx int,
) (lB, lA int) {
	if arrayLen <= 1 {
		return 0, 0
	}
	var neg bool
	if currentIdx < 0 {
		neg = true
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
	if neg {
		return 0, lA + lB
	}
	return lB, lA
}

func (lrc Lrc) ShowLyrics(
	timestamp, lookBehind, lookAhead uint64,
) (string, error) {
	emptyLyric := Lyric{}
	if lrc.lyrics == nil {
		return "", LrcEmpty{}
	}
	lyric, index, err := lrc.GetLyricfromTimestamp(timestamp)
	if err != nil {
		return "", err
	}
	final := strings.Builder{}
	var lB, lA int = calculateLaLb(int(lookAhead), int(lookBehind), len(lrc.lyrics), index)
	final.Grow((lB + lA + 1) * (6 + 2 + len(lyric.lyric)*2))
	if lB > 0 {
		for s := index - lB; s != index; s++ {
			final.WriteString(ansi.DIM)
			final.WriteString(lrc.lyrics[s].lyric)
			final.WriteString(ansi.RESET)
			final.WriteByte(NEWLINE)
			final.WriteByte(NEWLINE)
		}
	}
	if lyric != emptyLyric {
		final.WriteString(ansi.BOLD)
		final.WriteString(lyric.lyric)
		final.WriteString(ansi.RESET)
		final.WriteByte(NEWLINE)
		final.WriteByte(NEWLINE)
	}
	if lA > 0 {
		for s := index + 1; s < index+lA+1; s++ {
			final.WriteString(ansi.DIM)
			final.WriteString(lrc.lyrics[s].lyric)
			final.WriteString(ansi.RESET)
			final.WriteByte(NEWLINE)
			final.WriteByte(NEWLINE)
		}
	}
	return final.String(), nil
}

func LrctextToLrc(text string, duration uint64) (Lrc, error) {
	if text == "" {
		return Lrc{}, fmt.Errorf("empty text")
	}
	var err error
	iterLines := strings.Lines(text)
	lines := make([]string, 0, 256)
	for line := range iterLines {
		line = strings.TrimSpace(line)
		if len(line) >= 10 && hasValidSeperators(line[:10]) &&
			isTimestampDigits(line[:10]) {
			lines = append(lines, line)
		}
	}
	var lrc Lrc = Lrc{lyrics: make([]Lyric, 0, len(lines))}
	for index, line := range lines {
		if index == len(lines)-1 {
			timestampMs, err := TimestampToMs(line[:10])
			if err != nil {
				return lrc, err
			}
			lrc.lyrics = append(
				lrc.lyrics,
				Lyric{
					strings.TrimSpace(line[10:]),
					compareNum{timestampMs, duration},
				},
			)
			continue
		}
		var currLyricTimestamp, nextLyricTimestamp uint64
		nextLine := lines[index+1]
		currLyricTimestamp, _ = TimestampToMs(line[:10])
		nextLyricTimestamp, _ = TimestampToMs(nextLine[:10])
		lrc.lyrics = append(
			lrc.lyrics,
			Lyric{
				strings.TrimSpace(line[10:]),
				compareNum{currLyricTimestamp, nextLyricTimestamp},
			},
		)
	}
	return lrc, err
}

func (lrc *Lrc) GetLyricfromTimestamp(timeMs uint64) (Lyric, int, error) {
	if lrc.lyrics == nil {
		return Lyric{}, -1, LrcEmpty{}
	} else if timeMs < lrc.lyrics[0].timestamps.lower {
		return Lyric{}, -1, nil
	}
	isHalfWay := timeMs >= (lrc.lyrics[len(lrc.lyrics)-1].timestamps.upper / 2)
	if isHalfWay {
		for i := len(lrc.lyrics) - 1; i >= 0; i-- {
			lyric := lrc.lyrics[i]
			if lyric.timestamps.Between(timeMs) {
				return lyric, i, nil
			}
		}
	} else {
		for i := 0; i < len(lrc.lyrics); i++ {
			lyric := lrc.lyrics[i]
			if lyric.timestamps.Between(timeMs) {
				return lyric, i, nil
			}
		}
	}
	return Lyric{}, -1, nil
}

func (cn *compareNum) Between(num uint64) bool {
	return cn.lower <= num && num <= cn.upper
}

type TimestampError struct {
	cause string
}

func (ts TimestampError) Error() string {
	return fmt.Sprintf("[timestamp error] %s", ts.cause)
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
	return timestamp[0] == '[' &&
		timestamp[9] == ']' &&
		timestamp[3] == ':' &&
		timestamp[6] == '.'
}

func numRune(first, second byte) uint64 {
	return uint64((first-'0')*10 + second - '0')
}

func TimestampToMs(timestamp string) (uint64, error) {
	if !hasValidSeperators(timestamp) {
		return 0, TimestampError{"invalid seperators"}
	}
	if !isTimestampDigits(timestamp) {
		return 0, TimestampError{"not all digits"}
	}
	minute := numRune(timestamp[1], timestamp[2])
	secs := numRune(timestamp[4], timestamp[5])
	milis := numRune(timestamp[7], timestamp[8])
	return uint64(((minute*60)+secs)*1000 + milis*10), nil
}
