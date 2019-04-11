package indexhandler

import (
	"html/template"
	"net/http"

	"code.cloudfoundry.org/lager"
	"github.com/concourse/concourse/atc/api/auth"
	"github.com/gobuffalo/packr"
)

var InstanceName = ""

type templateData struct {
	CSRFToken    string
	AuthToken    string
	InstanceName string
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

	// csrfToken passed after logging in. Its validity is verified on server
	// based on auth token in Cookie.
	csrfToken := r.FormValue("csrf_token")

	authToken := ""
	authCookie, _ := r.Cookie(auth.AuthCookieName)
	if authCookie != nil {
		authToken = authCookie.Value
	}

	err := h.template.Execute(w, templateData{
		CSRFToken:    csrfToken,
		AuthToken:    authToken,
		InstanceName: InstanceName,
	})

	if err != nil {
		log.Fatal("failed-to-build-template", err, lager.Data{})
		w.WriteHeader(http.StatusInternalServerError)
	}

}
