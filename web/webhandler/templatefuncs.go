package webhandler

import (
	"crypto/md5"
	"fmt"
	"net/url"
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/auth"
	"github.com/concourse/atc/web"
	"github.com/concourse/go-concourse/concourse"
	"github.com/tedsuo/rata"
)

type templateFuncs struct {
	assetIDs map[string]string
	assetsL  sync.Mutex
}

func (funcs *templateFuncs) asset(asset string) (string, error) {
	funcs.assetsL.Lock()
	defer funcs.assetsL.Unlock()

	id, found := funcs.assetIDs[asset]
	if !found {
		hash := md5.New()

		contents, err := web.Asset("public/" + asset)
		if err != nil {
			return "", err
		}

		_, err = hash.Write(contents)
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
				"team_name":     args[0].(string),
				"pipeline_name": args[1].(string),
				"job":           jobName(args[2]),
			})
		default:
			return web.Routes.CreatePathForRoute(route, rata.Params{
				"team_name":     args[0].(string),
				"pipeline_name": args[1].(string),
				"job":           jobName(args[2]),
			})
		}

	case web.GetResource:
		baseResourceURL, err := web.Routes.CreatePathForRoute(route, rata.Params{
			"team_name":     args[0].(string),
			"pipeline_name": args[1].(string),
			"resource":      resourceName(args[2]),
		})

		if err != nil {
			return "", err
		}

		if len(args) > 3 {
			page := args[3].(*concourse.Page)

			if page.Since != 0 {
				baseResourceURL += fmt.Sprintf("?since=%d", page.Since)
			} else {
				baseResourceURL += fmt.Sprintf("?until=%d", page.Until)
			}
		}

		return baseResourceURL, nil

	case web.GetBuilds:
		path, err := web.Routes.CreatePathForRoute(route, rata.Params{})
		if err != nil {
			return "", err
		}

		if len(args) > 0 {
			page := args[0].(*concourse.Page)

			if page.Since != 0 {
				path += fmt.Sprintf("?since=%d", page.Since)
			} else {
				path += fmt.Sprintf("?until=%d", page.Until)
			}
		}

		return path, nil

	case web.GetBuild:
		build := args[1].(atc.Build)
		build.JobName = jobName(args[0])
		return web.PathForBuild(build), nil

	case web.GetJoblessBuild:
		return web.PathForBuild(args[0].(atc.Build)), nil

	case web.Public:
		return web.Routes.CreatePathForRoute(route, rata.Params{
			"filename": args[0].(string),
		})

	case web.GetJob:
		baseJobURL, err := web.Routes.CreatePathForRoute(route, rata.Params{
			"team_name":     args[0].(string),
			"pipeline_name": args[1].(string),
			"job":           jobName(args[2]),
		})
		if err != nil {
			return "", err
		}

		if len(args) > 3 {
			page := args[3].(*concourse.Page)

			if page.Since != 0 {
				baseJobURL += fmt.Sprintf("?since=%d", page.Since)
			} else {
				baseJobURL += fmt.Sprintf("?until=%d", page.Until)
			}
		}

		return baseJobURL, nil

	case atc.BuildEvents:
		return atc.Routes.CreatePathForRoute(route, rata.Params{
			"build_id": fmt.Sprintf("%d", args[0].(atc.Build).ID),
		})

	case atc.EnableResourceVersion, atc.DisableResourceVersion:
		versionedResource := args[2].(atc.VersionedResource)

		return atc.Routes.CreatePathForRoute(route, rata.Params{
			"team_name":           args[0].(string),
			"pipeline_name":       args[1].(string),
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

	case web.GetBasicAuthLogIn:
		authPath, err := web.Routes.CreatePathForRoute(route, rata.Params{
			"team_name": args[0].(string),
		})
		if err != nil {
			return "", err
		}

		return authPath + "?" + url.Values{
			"redirect": {args[1].(string)},
		}.Encode(), nil
	}

	return "", fmt.Errorf("unknown route: %s", route)
}
