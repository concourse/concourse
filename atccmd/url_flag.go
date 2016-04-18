package atccmd

import (
	"fmt"
	"net/url"
	"strings"
)

type URLFlag struct {
	url *url.URL
}

func (u *URLFlag) UnmarshalFlag(value string) error {
	value = normalizeURL(value)
	parsedURL, err := url.Parse(value)

	if err != nil {
		return err
	}

	if parsedURL.Scheme == "" {
		return fmt.Errorf("missing scheme in '%s'", value)
	}

	u.url = parsedURL

	return nil
}

func (u URLFlag) String() string {
	if u.url == nil {
		return ""
	}

	return u.url.String()
}

func (u URLFlag) URL() *url.URL {
	return u.url
}

func normalizeURL(urlIn string) string {
	return strings.TrimRight(urlIn, "/")
}
