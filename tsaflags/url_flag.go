package tsaflags

import "net/url"

type URLFlag struct {
	url *url.URL
}

func (u *URLFlag) UnmarshalFlag(value string) error {
	parsedURL, err := url.Parse(value)
	if err != nil {
		return err
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
