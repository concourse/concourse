package publichandler

import (
	"net/http"

	"github.com/concourse/web/bindata"
	"github.com/elazarl/go-bindata-assetfs"
)

func NewHandler() (http.Handler, error) {
	publicFS := &assetfs.AssetFS{
		Asset:     bindata.Asset,
		AssetDir:  bindata.AssetDir,
		AssetInfo: bindata.AssetInfo,
	}

	return CacheNearlyForever(http.FileServer(publicFS)), nil
}
