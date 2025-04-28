package cors

import (
	"net/http"

	"go.acuvity.ai/bahamut"
)

// HandleCORS handles CORS for browsers.
func HandleCORS(w http.ResponseWriter, req *http.Request, corsPolicy *bahamut.CORSPolicy) bool {

	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Xss-Protection", "1; mode=block")
	w.Header().Set("Cache-Control", "private, no-transform")

	if corsPolicy == nil {
		return true
	}

	if req.Method == http.MethodOptions {
		corsPolicy.Inject(w.Header(), req.Header.Get("Origin"), true)
		w.WriteHeader(http.StatusNoContent)
		return false
	}

	corsPolicy.Inject(w.Header(), req.Header.Get("Origin"), false)

	return true
}
