package api

import (
	"encoding/json"
	"net/http"
	"regexp"

	"code.cloudfoundry.org/lager/v3"
	"github.com/tedsuo/rata"
	"go.opentelemetry.io/otel/propagation"

	"github.com/concourse/concourse/v8/tracing"
	"github.com/concourse/concourse/v8/worker/baggageclaim"
	"github.com/concourse/concourse/v8/worker/baggageclaim/volume"
)

func NewHandler(
	logger lager.Logger,
	strategizer volume.Strategizer,
	volumeRepo volume.Repository,
	p2pInterfacePattern *regexp.Regexp,
	p2pInterfaceFamily int,
	p2pStreamPort uint16,
) (http.Handler, error) {
	volumeServer := NewVolumeServer(
		logger.Session("volume-server"),
		strategizer,
		volumeRepo,
		p2pInterfacePattern,
		p2pInterfaceFamily,
		p2pStreamPort,
	)

	handlers := rata.Handlers{
		baggageclaim.CreateVolume:            http.HandlerFunc(volumeServer.CreateVolume),
		baggageclaim.CreateVolumeAsync:       http.HandlerFunc(volumeServer.CreateVolumeAsync),
		baggageclaim.CreateVolumeAsyncCancel: http.HandlerFunc(volumeServer.CreateVolumeAsyncCancel),
		baggageclaim.CreateVolumeAsyncCheck:  http.HandlerFunc(volumeServer.CreateVolumeAsyncCheck),
		baggageclaim.ListVolumes:             http.HandlerFunc(volumeServer.ListVolumes),
		baggageclaim.GetVolume:               http.HandlerFunc(volumeServer.GetVolume),
		baggageclaim.SetProperty:             http.HandlerFunc(volumeServer.SetProperty),
		baggageclaim.GetPrivileged:           http.HandlerFunc(volumeServer.GetPrivileged),
		baggageclaim.SetPrivileged:           http.HandlerFunc(volumeServer.SetPrivileged),
		baggageclaim.StreamIn:                http.HandlerFunc(volumeServer.StreamIn),
		baggageclaim.StreamOut:               http.HandlerFunc(volumeServer.StreamOut),
		baggageclaim.GetP2pUrl:               http.HandlerFunc(volumeServer.GetP2pUrl),
		baggageclaim.StreamP2pOut:            http.HandlerFunc(volumeServer.StreamP2pOut),
		baggageclaim.DestroyVolume:           http.HandlerFunc(volumeServer.DestroyVolume),
		baggageclaim.DestroyVolumes:          http.HandlerFunc(volumeServer.DestroyVolumes),
		baggageclaim.CleanupOrphanedVolumes:  http.HandlerFunc(volumeServer.CleanupOrphanedVolumes),
	}

	router, err := rata.NewRouter(baggageclaim.Routes, handlers)
	if err != nil {
		return nil, err
	}

	return tracingMiddleware(router), nil
}

// tracingMiddleware puts the trace context from the request headers into the
// request context, so that the handler's spans continue the caller's trace.
func tracingMiddleware(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := tracing.Extract(req.Context(), propagation.HeaderCarrier(req.Header))
		handler.ServeHTTP(w, req.WithContext(ctx))
	})
}

type ErrorResponse struct {
	Message string `json:"error"`
}

func RespondWithError(w http.ResponseWriter, err error, statusCode ...int) {
	var code int

	if len(statusCode) > 0 {
		code = statusCode[0]
	} else {
		code = http.StatusInternalServerError
	}

	w.WriteHeader(code)
	errResponse := ErrorResponse{Message: err.Error()}
	json.NewEncoder(w).Encode(errResponse)
}
