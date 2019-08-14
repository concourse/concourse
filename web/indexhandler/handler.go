package indexhandler

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/gobuffalo/packr"
)

type templateData struct {
	CSRFToken string
	AuthToken string
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

	box := packr.NewBox("../public")

	src, err := box.MustBytes("index.html")
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

	err := h.template.Execute(w, templateData{
		CSRFToken: r.FormValue("csrf_token"),
		AuthToken: r.Header.Get("Authorization"),
	})

	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
	}

}
