package staticregistry

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

type Registry struct {
	ImageTarball       string
	RawResourceTarball string
}

func (*Registry) UbuntuImages(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`[{"id": "ubuntu-id", "checksum": "some-checksum"}]`))
}

func (*Registry) RawResourceImages(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`[{"id": "raw-resource-id", "checksum": "some-checksum"}]`))
}

func (*Registry) UbuntuTags(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"latest": "ubuntu-id"}`))
}

func (*Registry) RawResourceTags(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"latest": "raw-resource-id"}`))
}

func (*Registry) UbuntuAncestry(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`["ubuntu-layer"]`))
}

func (*Registry) RawResourceAncestry(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`["raw-resource-layer"]`))
}

func (registry *Registry) UbuntuLayerJSON(w http.ResponseWriter, r *http.Request) {
	tarball, err := os.Stat(registry.ImageTarball)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Add("X-Docker-Size", fmt.Sprintf("%d", tarball.Size()))

	w.Write([]byte(`{"id":"ubuntu-layer"}`))
}

func (registry *Registry) RawResourceLayerJSON(w http.ResponseWriter, r *http.Request) {
	tarball, err := os.Stat(registry.RawResourceTarball)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.Header().Add("X-Docker-Size", fmt.Sprintf("%d", tarball.Size()))

	w.Write([]byte(`{"id":"raw-resource-layer"}`))
}

func (registry *Registry) UbuntuLayerTarball(w http.ResponseWriter, r *http.Request) {
	tarball, err := os.Open(registry.ImageTarball)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	defer tarball.Close()

	io.Copy(w, tarball)
}

func (registry *Registry) RawResourceLayerTarball(w http.ResponseWriter, r *http.Request) {
	tarball, err := os.Open(registry.RawResourceTarball)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	defer tarball.Close()

	io.Copy(w, tarball)
}
