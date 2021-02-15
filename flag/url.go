package flag

import (
	"net/url"
	"strings"
)

type URL struct {
	*url.URL
}

func (u *URL) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var value string
	err := unmarshal(&value)
	if err != nil {
		return err
	}

	return u.Set(value)
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
