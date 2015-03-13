package web

import (
	"crypto/md5"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/concourse/atc"
	"github.com/concourse/atc/db"
	"github.com/concourse/atc/web/routes"
	"github.com/tedsuo/rata"
)

type templateFuncs struct {
	assetsDir string
	assetIDs  map[string]string
	assetsL   sync.Mutex
}

func (funcs templateFuncs) asset(asset string) (string, error) {
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

func (funcs templateFuncs) url(handler string, args ...interface{}) (string, error) {
	switch handler {
	case routes.TriggerBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job": jobName(args[0]),
		})

	case routes.GetBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"job":   jobName(args[0]),
			"build": args[1].(db.Build).Name,
		})

	case routes.AbortBuild:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"build_id": fmt.Sprintf("%d", args[0].(db.Build).ID),
		})

	case routes.Public:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{
			"filename": args[0].(string),
		})

	case atc.BuildEvents:
		return atc.Routes.CreatePathForRoute(handler, rata.Params{
			"build_id": fmt.Sprintf("%d", args[0].(db.Build).ID),
		})

	case atc.EnableResourceVersion, atc.DisableResourceVersion:
		return atc.Routes.CreatePathForRoute(handler, rata.Params{
			"version_id": fmt.Sprintf("%d", args[0].(db.SavedVersionedResource).ID),
		})

	case routes.LogIn:
		return routes.Routes.CreatePathForRoute(handler, rata.Params{})

	case atc.DownloadCLI:
		path, err := atc.Routes.CreatePathForRoute(handler, rata.Params{})
		if err != nil {
			return "", err
		}

		return path + "?" + url.Values{
			"platform": {args[0].(string)},
			"arch":     {args[1].(string)},
		}.Encode(), nil

	default:
		return "", fmt.Errorf("unknown route: %s", handler)
	}
}

func jobName(x interface{}) string {
	switch v := x.(type) {
	case string:
		return v
	default:
		return x.(atc.JobConfig).Name
	}
}
