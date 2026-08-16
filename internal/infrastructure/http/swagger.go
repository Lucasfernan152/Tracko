package httpapi

import (
	"net/http"

	"github.com/swaggest/swgui/v5emb"
)

func OpenAPIHandler(spec []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	}
}

func SwaggerUIHandler() http.Handler {
	return v5emb.New(
		"Tracko API",
		"/openapi.yaml",
		"/swagger/",
	)
}
