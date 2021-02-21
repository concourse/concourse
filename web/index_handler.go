package web

import (
	"crypto/md5"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"

	"code.cloudfoundry.org/lager"
)

func IndexHandler(logger lager.Logger, publicFS fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Session("index")

		tfuncs := &indexTemplateFuncs{
			publicFS: publicFS,
			assetIDs: map[string]string{},
		}

		funcs := template.FuncMap{
			"asset": tfuncs.asset,
		}

		t, err := template.New("web").Funcs(funcs).ParseFS(publicFS, "index.html")
		if err != nil {
			log.Error("failed-to-parse-templates", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = t.ExecuteTemplate(w, "index.html", indexTemplateData{
			CSRFToken: r.FormValue("csrf_token"),
			AuthToken: r.Header.Get("Authorization"),
		})
		if err != nil {
			log.Error("failed-to-build-template", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

type indexTemplateData struct {
	CSRFToken string
	AuthToken string
}

type indexTemplateFuncs struct {
	publicFS fs.FS

	assetIDs map[string]string
	assetsL  sync.Mutex
}

func (funcs *indexTemplateFuncs) asset(asset string) (string, error) {
	funcs.assetsL.Lock()
	defer funcs.assetsL.Unlock()

	id, found := funcs.assetIDs[asset]
	if !found {
		hash := md5.New()

		contents, err := fs.ReadFile(funcs.publicFS, asset)
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
