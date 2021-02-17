package docs

import (
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/vito/booklit"
)

func (p Plugin) Frame(src booklit.Content, optionalHeight ...string) booklit.Content {
	height := "300px"
	if len(optionalHeight) > 0 {
		height = optionalHeight[0]
	}

	return booklit.Styled{
		Style:   "frame",
		Content: booklit.Empty,
		Partials: booklit.Partials{
			"URL":    src,
			"Height": booklit.String(height),
		},
	}
}

func (p Plugin) RemoteCodeblock(language string, url string) (booklit.Content, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("remote codeblock for %s failed: %s", url, resp.Status)
	}

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return p.Codeblock(language, booklit.String(content)), nil
}
