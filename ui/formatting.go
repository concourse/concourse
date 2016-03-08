package ui

import "github.com/fatih/color"

func Embolden(message string, params ...interface{}) string {
	return color.New(color.Bold).SprintfFunc()(message, params...)
}
