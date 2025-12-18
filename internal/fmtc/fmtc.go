package fmtc

import (
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	RESET             = "\x1b[0m"
	BOLD              = "\x1b[1m"
	DIM               = "\x1b[2m"
	ITALIC            = "\x1b[3m"
	UNDERLINE         = "\x1b[4m"
	BLINK             = "\x1b[5m"
	REVERSE           = "\x1b[7m"
	HIDDEN            = "\x1b[8m"
	STRIKE            = "\x1b[9m"
	BLACK             = "\x1b[30m"
	BLACK_BG          = "\x1b[40m"
	BRIGHT_BLACK      = "\x1b[90m"
	BRIGHT_BLACK_BG   = "\x1b[100m"
	RED               = "\x1b[31m"
	RED_BG            = "\x1b[41m"
	BRIGHT_RED        = "\x1b[91m"
	BRIGHT_RED_BG     = "\x1b[101m"
	GREEN             = "\x1b[32m"
	GREEN_BG          = "\x1b[42m"
	BRIGHT_GREEN      = "\x1b[92m"
	BRIGHT_GREEN_BG   = "\x1b[102m"
	YELLOW            = "\x1b[33m"
	YELLOW_BG         = "\x1b[43m"
	BRIGHT_YELLOW     = "\x1b[93m"
	BRIGHT_YELLOW_BG  = "\x1b[103m"
	BLUE              = "\x1b[34m"
	BLUE_BG           = "\x1b[44m"
	BRIGHT_BLUE       = "\x1b[94m"
	BRIGHT_BLUE_BG    = "\x1b[104m"
	MAGENTA           = "\x1b[35m"
	MAGENTA_BG        = "\x1b[45m"
	BRIGHT_MAGENTA    = "\x1b[95m"
	BRIGHT_MAGENTA_BG = "\x1b[105m"
	CYAN              = "\x1b[36m"
	CYAN_BG           = "\x1b[46m"
	BRIGHT_CYAN       = "\x1b[96m"
	BRIGHT_CYAN_BG    = "\x1b[106m"
	WHITE             = "\x1b[37m"
	WHITE_BG          = "\x1b[47m"
	BRIGHT_WHITE      = "\x1b[97m"
	BRIGHT_WHITE_BG   = "\x1b[107m"
	DEFAULT           = "\x1b[39m"
	DEFAULT_BG        = "\x1b[49m"
)

var aliases = map[string]string{
	"{reset}":             RESET,
	"{bold}":              BOLD,
	"{dim}":               DIM,
	"{italic}":            ITALIC,
	"{underline}":         UNDERLINE,
	"{blink}":             BLINK,
	"{reverse}":           REVERSE,
	"{hidden}":            HIDDEN,
	"{strike}":            STRIKE,
	"{black}":             BLACK,
	"{bg:black}":          BLACK_BG,
	"{bright:black}":      BRIGHT_BLACK,
	"{bg:bright:black}":   BRIGHT_BLACK_BG,
	"{red}":               RED,
	"{bg:red}":            RED_BG,
	"{bright:red}":        BRIGHT_RED,
	"{bg:bright:red}":     BRIGHT_RED_BG,
	"{green}":             GREEN,
	"{bg:green}":          GREEN_BG,
	"{bright:green}":      BRIGHT_GREEN,
	"{bg:bright:green}":   BRIGHT_GREEN_BG,
	"{yellow}":            YELLOW,
	"{bg:yellow}":         YELLOW_BG,
	"{bright:yellow}":     BRIGHT_YELLOW,
	"{bg:bright:yellow}":  BRIGHT_YELLOW_BG,
	"{blue}":              BLUE,
	"{bg:blue}":           BLUE_BG,
	"{bright:blue}":       BRIGHT_BLUE,
	"{bg:bright:blue}":    BRIGHT_BLUE_BG,
	"{magenta}":           MAGENTA,
	"{bg:magenta}":        MAGENTA_BG,
	"{bright:magenta}":    BRIGHT_MAGENTA,
	"{bg:bright:magenta}": BRIGHT_MAGENTA_BG,
	"{cyan}":              CYAN,
	"{bg:cyan}":           CYAN_BG,
	"{bright:cyan}":       BRIGHT_CYAN,
	"{bg:bright:cyan}":    BRIGHT_CYAN_BG,
	"{white}":             WHITE,
	"{bg:white}":          WHITE_BG,
	"{bright:white}":      BRIGHT_WHITE,
	"{bg:bright:white}":   BRIGHT_WHITE_BG,
	"{default}":           DEFAULT,
	"{bg:default}":        DEFAULT_BG,
}

func expandColors(format string) string {
	for k, v := range aliases {
		format = strings.ReplaceAll(format, k, v)
	}
	return format
}

func Sprintf(format string, args ...any) string {
	return fmt.Sprintf(expandColors(format), args...)
}

func Printf(format string, args ...any) {
	fmt.Printf(expandColors(format), args...)
}

func Fprintf(w io.Writer, format string, args ...any) {
	fmt.Fprintf(w, expandColors(format), args...)
}

func Println(args ...any) {
	fmt.Fprintln(os.Stdout, args...)
}
