package publichandler

import (
	"net/http"

	"github.com/elazarl/go-bindata-assetfs"

	"github.com/concourse/atc/web"
)

func NewHandler() (http.Handler, error) {
	publicFS := &assetfs.AssetFS{
		Asset:     web.Asset,
		AssetDir:  web.AssetDir,
		AssetInfo: web.AssetInfo,
	}

	return CacheNearlyForever(http.FileServer(publicFS)), nil
}
