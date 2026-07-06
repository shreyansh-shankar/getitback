package output

import (
	"os"
	"runtime"
)

var (
	ColorReset  = ""
	ColorRed    = ""
	ColorGreen  = ""
	ColorYellow = ""
	ColorBlue   = ""
	ColorCyan   = ""
	ColorBold   = ""
)

func init() {
	if isTerminal() {
		ColorReset = "\033[0m"
		ColorRed = "\033[31m"
		ColorGreen = "\033[32m"
		ColorYellow = "\033[33m"
		ColorBlue = "\033[34m"
		ColorCyan = "\033[36m"
		ColorBold = "\033[1m"
	}
}

func isTerminal() bool {
	if runtime.GOOS == "windows" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func SprintfColor(color, s string) string {
	if color == "" {
		return s
	}
	return color + s + ColorReset
}
