package helpers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/concourse/concourse/atc"
)

func HandleBadRequest(w http.ResponseWriter, errorMessages ...string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	WriteSaveConfigResponse(w, atc.SaveConfigResponse{
		Errors: errorMessages,
	})
}

func WriteSaveConfigResponse(w http.ResponseWriter, saveConfigResponse atc.SaveConfigResponse) {
	err := json.NewEncoder(w).Encode(saveConfigResponse)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "failed to generate error response: %s", err)
		return
	}
}
