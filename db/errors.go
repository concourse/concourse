package db

import "errors"

var ErrInputNotDetermined = errors.New("input not yet determined; cannot know if redundant")
var ErrInputRedundant = errors.New("resource version already used for input")

var ErrOutputNotDetermined = errors.New("output not yet determined; cannot know if redundant")
var ErrOutputRedundant = errors.New("resource version came from output")
