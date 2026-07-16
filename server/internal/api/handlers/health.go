package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/subham12r/reso/internal/services"
)

func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(services.Health())
	})
}
