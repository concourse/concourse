package main

import "net/url"

type URLFlag struct {
	*url.URL
}

func (u *URLFlag) UnmarshalFlag(value string) error {
	parsedURL, err := url.Parse(value)
	if err != nil {
		return err
	}

	u.URL = parsedURL

	return nil
}

func (u URLFlag) String() string {
	if u.URL == nil {
		return ""
	}

	return u.URL.String()
}
