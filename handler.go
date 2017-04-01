package web

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
)

type templateData struct {
	CSRFToken string
}

type handler struct {
	logger   lager.Logger
	template *template.Template
}

func NewHandler(logger lager.Logger) (http.Handler, error) {
	tfuncs := &templateFuncs{
		assetIDs: map[string]string{},
	}

	funcs := template.FuncMap{
		"asset": tfuncs.asset,
	}

	src, err := Asset("index.html")
	if err != nil {
		return nil, err
	}

	t, err := template.New("index").Funcs(funcs).Parse(string(src))
	if err != nil {
		return nil, err
	}

	return &handler{
		logger:   logger,
		template: t,
	}, nil
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := h.logger.Session("index")

	// csrfToken passed after logging in. Its validity is verified on server
	// based on auth token in Cookie.
	csrfToken := r.FormValue("csrf_token")
	err := h.template.Execute(w, templateData{
		CSRFToken: csrfToken,
	})

	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
	}

}
