package main

const (
	ESC     = "\x1b["
	RESET   = ESC + "m"
	BOLD    = ESC + "1" + "m"
	DIM     = ESC + "2" + "m"
	RED     = ESC + "31" + "m"
	GREEN   = ESC + "32" + "m"
	YELLOW  = ESC + "33" + "m"
	BLUE    = ESC + "34" + "m"
	MAGENTA = ESC + "35" + "m"
	CLEAR   = ESC + "H" + ESC + "2J" + ESC + "3J"
)
