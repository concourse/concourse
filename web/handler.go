package web

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
)

type templateData struct{}

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

	err := h.template.Execute(w, templateData{})
	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
	}
}
