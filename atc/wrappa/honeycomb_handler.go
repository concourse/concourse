package wrappa

import (
	"net/http"

	"github.com/honeycombio/beeline-go/wrappers/common"
	"github.com/honeycombio/libhoney-go"
)

type HoneycombHandler struct {
	Handler http.Handler
	Client  *libhoney.Client
}

func (handler HoneycombHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ev := handler.Client.NewEvent()
	defer ev.Send()

	for k, v := range common.GetRequestProps(r) {
		ev.AddField(k, v)
	}

	wrappedWriter := common.NewResponseWriter(w)

	handler.Handler.ServeHTTP(wrappedWriter.Wrapped, r)

	if wrappedWriter.Status == 0 {
		wrappedWriter.Status = 200
	}
	ev.AddField("response.status_code", wrappedWriter.Status)
}
