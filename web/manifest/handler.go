package manifest

import (
	"encoding/json"
	"net/http"
)

type manifest struct {
	Schema          string         `json:"$schema,omitempty"`
	ShortName       string         `json:"short_name"`
	Name            string         `json:"name"`
	Description     string         `json:"description,omitempty"`
	StartURL        string         `json:"start_url"`
	Orientation     string         `json:"orientation,omitempty"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color,omitempty"`
	Display         string         `json:"display"`
	ServiceWorker   *serviceWorker `json:"serviceworker,omitempty"`
	Icons           []icon         `json:"icons"`
}

type serviceWorker struct {
	Src      string `json:"src"`
	Scope    string `json:"scope"`
	UseCache bool   `json:"use_cache,omitempty"`
}

type icon struct {
	Platform string `json:"platform,omitempty"`
	Purpose  string `json:"purpose,omitempty"`
	Src      string `json:"src"`
	Type     string `json:"type,omitempty"`
	Sizes    string `json:"sizes"`
}

// Handler provides a way to serve Web App Manifests.
// A web app manifest is a simple JSON file that controls how our app appears to
// the user in areas where they would expect to see apps (for example, a mobile device's
// home screen), direct what the user can launch, and define its appearance at launch.
//
// For further details see:
// * https://www.w3.org/TR/appmanifest/
// * https://developer.mozilla.org/en-US/docs/Web/Manifest
// * https://developers.google.com/web/fundamentals/engage-and-retain/web-app-manifest/
type Handler interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

type handler struct {
	m manifest
}

// NewHandler creates a new handler that is able to serve web app manifest.
func NewHandler() Handler {
	return &handler{
		m: manifest{
			Schema:          "http://json.schemastore.org/web-manifest#",
			ShortName:       "Concourse",
			Name:            "Concourse",
			Description:     "CI that scales with your project.",
			StartURL:        "/",
			Orientation:     "any",
			BackgroundColor: "#000000",
			ThemeColor:      "#000000",
			Display:         "standalone",
			Icons: []icon{
				{
					Src:   "public/images/favicon.png",
					Type:  "image/png",
					Sizes: "32x32",
				},
				{
					Src:   "public/images/favicon-144x144.png",
					Type:  "image/png",
					Sizes: "144x144",
				},
				{
					Src:   "public/images/favicon.svg",
					Type:  "image/svg+xml",
					Sizes: "256x256 512x512 1024x1024",
				},
			},
		},
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/manifest+json")
	jsonEncoder := *json.NewEncoder(w)
	if err := jsonEncoder.Encode(h.m); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
