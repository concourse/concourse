package flag

import (
	"fmt"
	"net/url"
	"strings"
)

type URLs []URL

// Can be removed once flags are deprecated
func (u *URLs) Set(value string) error {
	unparsedURLs := strings.Split(value, ",")

	var parsedURLs URLs
	for _, unparsedURL := range unparsedURLs {
		urlVal := strings.TrimRight(strings.TrimSpace(unparsedURL), "/")
		parsedURL, err := url.Parse(urlVal)
		if err != nil {
			return err
		}

		parsedURLs = append(parsedURLs, URL{parsedURL})
	}

	u = &parsedURLs

	return nil
}

// Can be removed once flags are deprecated
func (u *URLs) String() string {
	if u == nil {
		return ""
	}

	var urlsString string
	for _, parsedURL := range *u {
		urlsString = fmt.Sprintf("%s,%s", urlsString, parsedURL.String())
	}

	return urlsString
}

// Can be removed once flags are deprecated
func (u *URLs) Type() string {
	return "URLs"
}

type URL struct {
	*url.URL
}

func (u URL) MarshalYAML() (interface{}, error) {
	if u.URL == nil {
		return "", nil
	}

	return u.URL.String(), nil
}

func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	if value != "" {
		return u.Set(value)
	}

	return nil
}

// Can be removed once flags are deprecated
func (u *URL) Set(value string) error {
	value = strings.TrimRight(value, "/")
	parsedURL, err := url.Parse(value)
	if err != nil {
		return err
	}

	u.URL = parsedURL

	return nil
}

// Can be removed once flags are deprecated
func (u *URL) String() string {
	if u.URL == nil {
		return ""
	}

	return u.URL.String()
}

// Can be removed once flags are deprecated
func (u *URL) Type() string {
	return "URL"
}
