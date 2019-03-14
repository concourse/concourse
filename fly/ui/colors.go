package ui

import "github.com/fatih/color"

var PendingColor = color.New(color.FgWhite)
var StartedColor = color.New(color.FgYellow)
var SucceededColor = color.New(color.FgGreen)
var FailedColor = color.New(color.FgRed)
var ErroredColor = color.New(color.FgRed, color.Bold)
var BlinkingErrorColor = color.New(color.BlinkSlow, color.FgWhite, color.BgRed, color.Bold)
var AbortedColor = color.New(color.FgMagenta)
var PausedColor = color.New(color.FgCyan)

var OnColor = color.New(color.FgCyan)
var OffColor = color.New(color.Faint)
