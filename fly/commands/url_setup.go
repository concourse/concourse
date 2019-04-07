package commands

import (
	"net/url"
	"os"
	"strings"
)

type UrlSetupOptions struct {
	Url string `short:"u" long:"url" description:"URL for the team, pipeline, job, build, or container to target"`
}

var UrlSetup UrlSetupOptions

func (options *UrlSetupOptions) SetInEnvironment() error {
	if options.Url != "" {
		u, err := url.Parse(options.Url)
		if err != nil {
			return err
		}
		urlMap := parseUrlPath(u.Path)

		if os.Getenv("TEAM") == "" && urlMap["teams"] != "" {
			os.Setenv("TEAM", urlMap["teams"])
		}
		if os.Getenv("PIPELINE") == "" && urlMap["pipelines"] != "" {
			os.Setenv("PIPELINE", urlMap["pipelines"])
		}
		if os.Getenv("JOB") == "" && urlMap["pipelines"] != "" && urlMap["jobs"] != "" {
			os.Setenv("JOB", urlMap["pipelines"]+"/"+urlMap["jobs"])
		}
		if os.Getenv("BUILD") == "" && urlMap["builds"] != "" {
			os.Setenv("BUILD", urlMap["builds"])
		}
		if os.Getenv("RESOURCE") == "" && urlMap["pipelines"] != "" && urlMap["resources"] != "" {
			os.Setenv("RESOURCE", urlMap["pipelines"]+"/"+urlMap["resources"])
		}
		if os.Getenv("RESOURCE_TYPE") == "" && urlMap["pipelines"] != ""  && urlMap["resource-types"] != "" {
			os.Setenv("RESOURCE_TYPE", urlMap["pipelines"]+"/"+urlMap["resource-types"])
		}
	}

	return nil
}

func parseUrlPath(urlPath string) map[string]string {
	pathWithoutFirstSlash := strings.Replace(urlPath, "/", "", 1)
	urlComponents := strings.Split(pathWithoutFirstSlash, "/")
	urlMap := make(map[string]string)

	for i := 0; i < len(urlComponents)/2; i++ {
		keyIndex := i * 2
		valueIndex := keyIndex + 1
		urlMap[urlComponents[keyIndex]] = urlComponents[valueIndex]
	}

	return urlMap
}
