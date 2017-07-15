package pygmentize

import (
	"bytes"
	"encoding/xml"
	"os"
	"os/exec"

	"github.com/vito/booklit"
)

type highlightedHTML struct {
	Pre struct {
		Code string `xml:",innerxml"`
	} `xml:"pre"`
}

func Inline(language string, content string) (booklit.Content, error) {
	html := new(bytes.Buffer)

	pygmentize := exec.Command("pygmentize", "-l", language, "-f", "html", "-O", "encoding=utf-8")
	pygmentize.Stdin = bytes.NewBufferString(content)
	pygmentize.Stdout = html
	pygmentize.Stderr = os.Stderr
	err := pygmentize.Run()
	if err != nil {
		return nil, err
	}

	var hl highlightedHTML
	err = xml.Unmarshal(html.Bytes(), &hl)
	if err != nil {
		return nil, err
	}

	return booklit.String(hl.Pre.Code), nil
}

func Block(language string, content string) (booklit.Content, error) {
	hl, err := Inline(language, content)
	if err != nil {
		return nil, err
	}

	return booklit.Styled{
		Style:   "highlighted-block",
		Content: hl,
	}, nil
}
