package wrappa

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"code.cloudfoundry.org/lager"
)

type CompressionHandler struct {
	Handler http.Handler
	Logger  lager.Logger
}

var gzPool = sync.Pool{
	New: func() interface{} {
		w := gzip.NewWriter(ioutil.Discard)
		return w
	},
}

type flusher interface {
	Flush() error
}

type gzipResponseWriter struct {
	io.Writer
	flusher
	http.ResponseWriter
	lager.Logger
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if w.flusher != nil {
		err := w.flusher.Flush()
		if err != nil {
			w.Logger.Info("failed-to-flush", lager.Data{"error": err.Error()})
		}
	}

	w.ResponseWriter.(http.Flusher).Flush()
}

func (c CompressionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Vary", "Accept-Encoding")

	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		c.Handler.ServeHTTP(w, r)
		return
	}

	w.Header().Set("Content-Encoding", "gzip")

	gz := gzPool.Get().(*gzip.Writer)
	defer gzPool.Put(gz)

	gz.Reset(w)
	defer gz.Close()

	c.Handler.ServeHTTP(
		&gzipResponseWriter{
			ResponseWriter: w,
			flusher:        gz,
			Writer:         gz,
			Logger:         c.Logger,
		},
		r,
	)
}
