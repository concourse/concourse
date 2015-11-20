package webhandler

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web"
	"github.com/concourse/go-concourse/concourse"
	"github.com/tedsuo/rata"
)

type templateFuncs struct {
	assetsDir string
	assetIDs  map[string]string
	assetsL   sync.Mutex
}

func (funcs *templateFuncs) asset(asset string) (string, error) {
	funcs.assetsL.Lock()
	defer funcs.assetsL.Unlock()

	id, found := funcs.assetIDs[asset]
	if !found {
		hash := md5.New()

		file, err := os.Open(filepath.Join(funcs.assetsDir, asset))
		if err != nil {
			return "", err
		}

		_, err = io.Copy(hash, file)
		if err != nil {
			return "", err
		}

		id = fmt.Sprintf("%x", hash.Sum(nil))
	}

	return funcs.url("Public", asset+"?id="+id)
}

func (funcs *templateFuncs) url(route string, args ...interface{}) (string, error) {
	return PathFor(route, args...)
}

func (funcs *templateFuncs) withRedirect(authURLStr string, redirect string) string {
	authURL, err := url.Parse(authURLStr)
	if err != nil {
		return "<malformed>"
	}

	withRedirect := authURL.Query()
	withRedirect["redirect"] = []string{redirect}
	authURL.RawQuery = withRedirect.Encode()

	return authURL.String()
}

func jobName(x interface{}) string {
	switch v := x.(type) {
	case string:
		return v
	case atc.Job:
		return v.Name
	case atc.JobConfig:
		return v.Name
	default:
		panic(fmt.Sprintf("unexpected arg type"))
	}
}

func resourceName(x interface{}) string {
	switch v := x.(type) {
	case string:
		return v
	case atc.Resource:
		return v.Name
	default:
		panic(fmt.Sprintf("unexpected arg type"))
	}
}

func PathFor(route string, args ...interface{}) (string, error) {
	switch route {
	case web.TriggerBuild:
		switch args[1].(type) {
		case atc.Job:
			return web.Routes.CreatePathForRoute(route, rata.Params{
				"pipeline_name": args[0].(string),
				"job":           jobName(args[1]),
			})
		default:
			return web.Routes.CreatePathForRoute(route, rata.Params{
				"pipeline_name": args[0].(string),
				"job":           jobName(args[1]),
			})
		}

	case web.GetResource:
		baseResourceURL, err := web.Routes.CreatePathForRoute(route, rata.Params{
			"pipeline_name": args[0].(string),
			"resource":      resourceName(args[1]),
		})

		if err != nil {
			return "", err
		}

		if len(args) > 2 {
			page := args[2].(*concourse.Page)

			if page.Since != 0 {
				baseResourceURL += fmt.Sprintf("?since=%d", page.Since)
			} else {
				baseResourceURL += fmt.Sprintf("?until=%d", page.Until)
			}
		}

		return baseResourceURL, nil

	case web.GetBuild:
		switch args[1].(type) {
		case atc.Build:
			build := args[1].(atc.Build)
			build.JobName = jobName(args[0])
			return web.PathForBuildNew(build), nil
		default:
			build := args[1].(db.Build)
			build.JobName = jobName(args[0])
			return web.PathForBuild(build), nil
		}
	case web.GetJoblessBuild:
		return web.PathForBuild(args[0].(db.Build)), nil

	case web.Public:
		return web.Routes.CreatePathForRoute(route, rata.Params{
			"filename": args[0].(string),
		})

	case web.GetJob:
		baseJobURL, err := web.Routes.CreatePathForRoute(route, rata.Params{
			"pipeline_name": args[0].(string),
			"job":           jobName(args[1]),
		})
		if err != nil {
			return "", err
		}

		if len(args) > 2 {
			page := args[2].(*concourse.Page)

			if page.Since != 0 {
				baseJobURL += fmt.Sprintf("?since=%d", page.Since)
			} else {
				baseJobURL += fmt.Sprintf("?until=%d", page.Until)
			}
		}

		return baseJobURL, nil

	case atc.BuildEvents:
		switch args[0].(type) {
		case atc.Build:
			return atc.Routes.CreatePathForRoute(route, rata.Params{
				"build_id": fmt.Sprintf("%d", args[0].(atc.Build).ID),
			})
		default:
			return atc.Routes.CreatePathForRoute(route, rata.Params{
				"build_id": fmt.Sprintf("%d", args[0].(db.Build).ID),
			})
		}

	case atc.EnableResourceVersion, atc.DisableResourceVersion:
		versionedResource := args[1].(atc.VersionedResource)

		return atc.Routes.CreatePathForRoute(route, rata.Params{
			"pipeline_name":       args[0].(string),
			"resource_name":       fmt.Sprintf("%s", versionedResource.Resource),
			"resource_version_id": fmt.Sprintf("%d", versionedResource.ID),
		})

	case web.LogIn:
		return web.Routes.CreatePathForRoute(route, rata.Params{})

	case atc.DownloadCLI:
		path, err := atc.Routes.CreatePathForRoute(route, rata.Params{})
		if err != nil {
			return "", err
		}

		return path + "?" + url.Values{
			"platform": {args[0].(string)},
			"arch":     {args[1].(string)},
		}.Encode(), nil

	case auth.OAuthBegin:
		authPath, err := auth.OAuthRoutes.CreatePathForRoute(route, rata.Params{
			"provider": args[0].(string),
		})
		if err != nil {
			return "", err
		}

		return authPath + "?" + url.Values{
			"redirect": {args[1].(string)},
		}.Encode(), nil

	case web.BasicAuth:
		authPath, err := web.Routes.CreatePathForRoute(route, rata.Params{})
		if err != nil {
			return "", err
		}

		return authPath + "?" + url.Values{
			"redirect": {args[0].(string)},
		}.Encode(), nil

	default:
		return "", fmt.Errorf("unknown route: %s", route)
	}
}
