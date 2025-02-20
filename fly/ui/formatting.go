package ui

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
)

func Embolden(message string, params ...interface{}) string {
	if len(params) > 0 {
		message = fmt.Sprintf(message, params...)
	}

	if isatty.IsTerminal(os.Stdout.Fd()) {
		return fmt.Sprintf("\033[1m%s\033[22m", message)
	}

	return message
}

func WarningColor(message string, params ...interface{}) string {
	return color.New(color.FgRed).SprintfFunc()(message, params...)
}
