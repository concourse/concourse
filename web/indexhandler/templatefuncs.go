package indexhandler

import (
	"crypto/md5"
	"fmt"
	"sync"

	"github.com/gobuffalo/packr"
)

type templateFuncs struct {
	assetIDs map[string]string
	assetsL  sync.Mutex
}

func (funcs *templateFuncs) asset(asset string) (string, error) {
	funcs.assetsL.Lock()
	defer funcs.assetsL.Unlock()

	box := packr.NewBox("../public")

	id, found := funcs.assetIDs[asset]
	if !found {
		hash := md5.New()

		contents, err := box.MustBytes(asset)
		if err != nil {
			return "", err
		}

		_, err = hash.Write(contents)
		if err != nil {
			return "", err
		}

		id = fmt.Sprintf("%x", hash.Sum(nil))
	}

	return fmt.Sprintf("/public/%s?id=%s", asset, id), nil
}
