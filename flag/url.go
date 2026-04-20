package flag

import (
	"fmt"
	"net/url"
	"strings"
)

type URL struct {
	*url.URL
}

func (u *URL) UnmarshalFlag(value string) error {
	value = normalizeURL(value)
	parsedURL, err := url.Parse(value)

	if err != nil {
		return err
	}

	// localhost URLs that do not start with http:// are interpreted
	// with `localhost` as the Scheme, not the Host
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("missing scheme in '%s'", value)
	}

	u.URL = parsedURL

	return nil
}

func (u URL) String() string {
	if u.URL == nil {
		return ""
	}

	return u.URL.String()
}

func normalizeURL(urlIn string) string {
	return strings.TrimRight(urlIn, "/")
}
