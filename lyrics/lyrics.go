package lyrics

import (
	"fmt"
	"strconv"
	"strings"

	"musger/ansi"
)

type compareNum struct {
	lower, upper uint64
}

type Lyric struct {
	timestamps compareNum
	lyric      string
}

type Lrc struct {
	lyrics []Lyric
}

type LrcDisplay struct {
	lrc        Lrc
	lookBehind uint64
	lookAhead  uint64
}

type LrcEmpty struct{}

func (LrcEmpty) Error() string {
	return "no lrc lyric"
}

func (ld LrcDisplay) ShowLyrics(timestamp uint64) (string, error) {
	emptyLyric := Lyric{}
	if ld.lrc.lyrics == nil {
		return "", LrcEmpty{}
	}
	lyric, index, err := ld.lrc.GetLyricfromTimestamp(timestamp)
	if err != nil {
		return "", err
	}
	final := strings.Builder{}
	var lB, lA int = int(ld.lookBehind), int(ld.lookAhead)
	if index < int(ld.lookBehind) {
		lB = index
		lA = (int(ld.lookBehind) - lB) + lA
	}
	if index >= len(ld.lrc.lyrics)-int(ld.lookAhead) {
		lA = len(ld.lrc.lyrics) - (index + 1)
		lB = (int(ld.lookAhead) - lA) + lB
	}
	if index == -1 {
		lA, lB = int(ld.lookAhead)+int(ld.lookBehind)+1, 0
		if lA > len(ld.lrc.lyrics) {
			lA = len(ld.lrc.lyrics)
		}
	}
	if len(ld.lrc.lyrics) == 1 {
		lA, lB = 0, 0
	}
	if lB > 0 {
		for s := index - lB; s != index; s++ {
			if s < 0 {
				continue
			}
			fmt.Fprintf(
				&final,
				"%v%v%v\n\n",
				ansi.DIM,
				ld.lrc.lyrics[s].lyric,
				ansi.RESET,
			)
		}
	}
	if lyric != emptyLyric {
		fmt.Fprintf(&final, "%v%v%v\n\n", ansi.BOLD, lyric.lyric, ansi.RESET)
	}
	if lA > 0 {
		for s := index + 1; s < index+lA+1; s++ {
			if s >= len(ld.lrc.lyrics) {
				break
			}
			fmt.Fprintf(
				&final,
				"%v%v%v\n\n",
				ansi.DIM,
				ld.lrc.lyrics[s].lyric,
				ansi.RESET,
			)
		}
	}
	return final.String(), nil
}

func LrctextToLrc(text string) (Lrc, error) {
	if text == "" {
		return Lrc{}, fmt.Errorf("empty text")
	}
	var lrc Lrc
	iterLines := strings.Lines(text)
	var lines []string
	for line := range iterLines {
		line = strings.TrimSpace(line)
		if len(line) < 11 {
			continue
		}
		if line[0] != '[' || !IsDigit(line[1]) {
			continue
		}
		lines = append(lines, line)
	}
	for index, line := range lines {
		if index == len(lines)-1 {
			timestampMs, err := timestampToMs(string(line[:10]))
			if err != nil {
				return Lrc{}, err
			}
			lrc.lyrics = append(lrc.lyrics,
				Lyric{compareNum{timestampMs, timestampMs}, string(line[11:])})
			continue
		}
		nextLine := lines[index+1]
		currLyricTimestamp, err := timestampToMs(string(line[:10]))
		if err != nil {
			return Lrc{}, fmt.Errorf("%w, '%v'", err, line)
		}
		nextLyricTimestamp, err := timestampToMs(string(nextLine[:10]))
		if err != nil {
			return Lrc{}, fmt.Errorf("%w, '%v'", err, nextLine)
		}
		lrc.lyrics = append(
			lrc.lyrics,
			Lyric{
				compareNum{currLyricTimestamp, nextLyricTimestamp},
				line[11:],
			},
		)
	}
	return lrc, nil
}

func (lrc *Lrc) GetLyricfromTimestamp(timeMs uint64) (Lyric, int, error) {
	if lrc.lyrics == nil {
		return Lyric{}, -1, LrcEmpty{}
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

func IsDigit(b byte) bool {
	return b >= 48 && b <= 57
}

func timestampToMs(timestamp string) (uint64, error) {
	if len(timestamp) != 10 {
		return 0, TimestampError{"invalid length"}
	}
	if [4]byte{
		timestamp[0],
		timestamp[9],
		timestamp[3],
		timestamp[6],
	} != [4]byte{
		'[',
		']',
		':',
		'.',
	} {
		return 0, TimestampError{"invalid seperators"}
	}
	if !(IsDigit(timestamp[1]) && IsDigit(timestamp[2]) &&
		IsDigit(timestamp[4]) && IsDigit(timestamp[5]) &&
		IsDigit(timestamp[7]) && IsDigit(timestamp[8])) {
		return 0, TimestampError{"not all digits"}
	}
	minute, _ := strconv.Atoi(string(timestamp[1:3]))
	secs, _ := strconv.Atoi(string(timestamp[4:6]))
	milis, _ := strconv.Atoi(string(timestamp[7:9]))
	return uint64(((minute*60)+secs)*1000 + milis*10), nil
}
