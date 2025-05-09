package backend

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

type OAuth2Provider interface {
	BaseURL() string
	Client() *http.Client
}

func handleOAuth(c any, w http.ResponseWriter, req *http.Request, path string) func() {

	cl, ok := c.(OAuth2Provider)
	if !ok {
		http.Error(w, "Not Found", http.StatusNotFound)
		return func() {}
	}

	u := fmt.Sprintf("%s/%s", cl.BaseURL(), strings.TrimLeft(path, "/"))

	slog.Info("Forwarding OAuth2 call", "method", req.Method, "target", u)

	breq, err := http.NewRequestWithContext(req.Context(), req.Method, u, req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to make request: %s", err), http.StatusInternalServerError)
		return func() {}
	}

	breq.URL.RawQuery = req.URL.Query().Encode()
	breq.Header = req.Header.Clone()

	resp, err := cl.Client().Do(breq) // nolint: bodyclose
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return func() {}
	}

	for k, vs := range resp.Header {
		for _, v := range vs {

			if k == "Origin" {
				continue
			}

			if strings.HasPrefix(k, "Access-Control-") {
				continue
			}

			w.Header().Add(k, v)
		}
	}

	w.WriteHeader(resp.StatusCode)

	if resp.Body != nil {
		_, _ = io.Copy(w, resp.Body)
		return func() { _ = resp.Body.Close() }
	}

	return func() {}
}
