package ansistream

import (
	"bytes"
	"io"
	"strings"
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
	prevbright := false
	prevcolor := ""

	bold := prevbold
	bright := prevbright
	color := prevcolor

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

		if bright {
			classes = append(classes, "ansi-bright")
		}

		if color != "" {
			classes = append(classes, "ansi-"+color)
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

			writeBuf.Write(text)
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
				case '0':
					bold = false
					bright = false
					color = ""
				case '1':
					bold = true
				}

			case 2:
				if code[0] == '9' {
					bright = true
				} else {
					bright = false
				}

				switch code[1] {
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
