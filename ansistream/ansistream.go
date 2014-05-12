package ansistream

import (
	"bytes"
	"io"
	"strings"
	"text/template"
	"unicode/utf8"
)

type Writer struct {
	destination io.WriteCloser

	dangling []byte
}

func NewWriter(destination io.WriteCloser) *Writer {
	return &Writer{
		destination: destination,
	}
}

func (streamer *Writer) Write(data []byte) (int, error) {
	fullData := append(streamer.dangling, data...)

	streamer.dangling = nil

	fullReader := bytes.NewBuffer(fullData)

	writeBuf := new(bytes.Buffer)

	styled := false

	prevbold := false
	prevcolor := ""
	prevbright := false
	prevbgcolor := ""
	prevbgbright := false

	bold := prevbold
	bright := prevbright
	color := prevcolor
	bgcolor := prevbgcolor
	bgbright := prevbgbright

	var lastSequence []byte

	for eof := false; !eof; {
		text, err := fullReader.ReadBytes('\x1b')

		if len(text) > 0 {
			checkEncoding, _ := utf8.DecodeLastRune(text)
			if checkEncoding == utf8.RuneError {
				streamer.dangling = append(lastSequence, text...)
				break
			}
		}

		if err == io.EOF {
			eof = true
		} else {
			text = text[:len(text)-1]
		}

		classes := []string{}

		if bold {
			classes = append(classes, "ansi-bold")
		}

		if color != "" {
			prefix := "ansi-"
			if bright {
				prefix += "bright-"
			}

			classes = append(classes, prefix+color+"-fg")
		}

		if bgcolor != "" {
			prefix := "ansi-"
			if bgbright {
				prefix += "bright-"
			}

			classes = append(classes, prefix+bgcolor+"-bg")
		}

		if len(text) > 0 {
			if color != prevcolor || bright != prevbright || bold != prevbold {
				if styled {
					writeBuf.Write([]byte(`</span>`))
					styled = false
				}
			}

			if len(classes) > 0 {
				if !styled {
					writeBuf.Write([]byte(`<span class="` + strings.Join(classes, " ") + `">`))
					styled = true
				}
			}

			prevbold = bold
			prevbright = bright
			prevcolor = color

			template.HTMLEscape(writeBuf, text)
		}

		if eof {
			if lastSequence != nil {
				streamer.dangling = lastSequence
			}

			break
		}

		bracket, err := fullReader.ReadByte()
		if err == io.EOF {
			streamer.dangling = []byte{'\x1b'}
			break
		}

		if bracket != '[' {
			writeBuf.Write([]byte{'\x1b', bracket})
			continue
		}

		codesSegment, err := fullReader.ReadBytes('m')
		if err == io.EOF {
			streamer.dangling = append([]byte("\x1b["), codesSegment...)
			break
		}

		codes := codesSegment[:len(codesSegment)-1]

		for _, code := range bytes.Split(codes, []byte(";")) {
			switch len(code) {
			case 1:
				switch code[0] {
				case '1':
					bold = true
				case '0':
					bold = false
					color = ""
					bright = false
					bgcolor = ""
					bgbright = false
				}
			case 2:
				switch code[0] {
				case '3':
					color = colorFor(code[1])
				case '9':
					color = colorFor(code[1])
					bright = true
				case '4':
					bgcolor = colorFor(code[1])
				}
			case 3:
				if code[0] == '1' && code[1] == '0' {
					bgcolor = colorFor(code[2])
					bgbright = true
				}
			}
		}

		lastSequence = append([]byte("\x1b["), codesSegment...)
	}

	if styled {
		writeBuf.Write([]byte(`</span>`))
		styled = false
	}

	_, err := writeBuf.WriteTo(streamer.destination)
	if err != nil {
		return 0, err
	}

	return len(data), nil
}

func (streamer *Writer) Close() error {
	return streamer.destination.Close()
}

func colorFor(c byte) string {
	var color string

	switch c {
	case '0':
		color = "black"
	case '1':
		color = "red"
	case '2':
		color = "green"
	case '3':
		color = "yellow"
	case '4':
		color = "blue"
	case '5':
		color = "magenta"
	case '6':
		color = "cyan"
	case '7':
		color = "white"
	}

	return color
}
