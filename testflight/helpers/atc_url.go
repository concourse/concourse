package helpers

import (
	"os"
)

const defaultATCURL = "http://localhost:8080"

var storedATCURL string

func AtcURL() string {
	if storedATCURL != "" {
		return storedATCURL
	}

	atcURL := os.Getenv("ATC_URL")
	if atcURL == "" {
		atcURL = defaultATCURL
	}

	storedATCURL = atcURL

	return atcURL
}

func AtcUsername() string {
	username := os.Getenv("ATC_USERNAME")
	if username == "" {
		username = "test"
	}

	return username
}

func AtcPassword() string {
	password := os.Getenv("ATC_PASSWORD")
	if password == "" {
		password = "test"
	}

	return password
}
