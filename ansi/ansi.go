package ansi

import "strconv"

var (
	ESC     = "\x1b["
	RESET   = ESC + "m"
	BOLD    = sgr(1)
	DIM     = sgr(2)
	INVERT  = sgr(7)
	RED     = sgr(31)
	GREEN   = sgr(32)
	YELLOW  = sgr(33)
	BLUE    = sgr(34)
	MAGENTA = sgr(35)
	WHITE   = sgr(37)
	BBLACK  = sgr(90)
	CLEAR   = ESC + "H" + ESC + "2J" + ESC + "3J"
)

func sgr(num int64) string {
	return ESC + strconv.FormatInt(num, 10) + "m"
}
