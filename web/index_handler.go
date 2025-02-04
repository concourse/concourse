package web

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"sync"

	"code.cloudfoundry.org/lager/v3"
	"github.com/concourse/concourse/atc"
)

type assetCache struct {
	fs      fs.FS
	cache   map[string]string
	cacheMu sync.RWMutex
	dynamic bool // true if using filesystem that can change
}

func newAssetCache(fs fs.FS, dynamic bool) *assetCache {
	return &assetCache{
		fs:      fs,
		cache:   make(map[string]string),
		dynamic: dynamic,
	}
}

func (ac *assetCache) asset(path string) (string, error) {
	if !ac.dynamic {
		// Try reading from cache first for embedded assets
		ac.cacheMu.RLock()
		if hash, ok := ac.cache[path]; ok {
			ac.cacheMu.RUnlock()
			return fmt.Sprintf("/public/%s?id=%s", path, hash), nil
		}
		ac.cacheMu.RUnlock()
	}

	// Calculate hash
	contents, err := fs.ReadFile(ac.fs, path)
	if err != nil {
		return "", fmt.Errorf("read asset %s: %w", path, err)
	}

	sum := sha256.Sum256(contents)
	hash := fmt.Sprintf("%x", sum[:4]) // First 8 hex characters

	// Only cache if not in dynamic mode
	if !ac.dynamic {
		ac.cacheMu.Lock()
		ac.cache[path] = hash
		ac.cacheMu.Unlock()
	}

	return fmt.Sprintf("/public/%s?id=%s", path, hash), nil
}

func IndexHandler(logger lager.Logger, publicFS fs.FS, live bool) http.Handler {
	assets := newAssetCache(publicFS, live)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Session("index")

		funcs := template.FuncMap{
			"asset": assets.asset,
			"jsonMarshal": func(v interface{}) (template.JS, error) {
				payload, err := json.Marshal(v)
				if err != nil {
					return "", err
				}
				return template.JS(payload), nil
			},
		}

		t, err := template.New("web").Funcs(funcs).ParseFS(publicFS, "index.html")
		if err != nil {
			log.Error("failed-to-parse-templates", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		err = t.ExecuteTemplate(w, "index.html", indexTemplateData{
			CSRFToken:    r.FormValue("csrf_token"),
			AuthToken:    r.Header.Get("Authorization"),
			FeatureFlags: atc.FeatureFlags(),
		})
		if err != nil {
			log.Error("failed-to-build-template", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	})
}

type indexTemplateData struct {
	CSRFToken    string
	AuthToken    string
	FeatureFlags map[string]bool
}
