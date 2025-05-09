package oauth

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

func Forward(baseURL string, client *http.Client, w http.ResponseWriter, req *http.Request, path string) func() {

	u := fmt.Sprintf("%s/%s", baseURL, strings.TrimLeft(path, "/"))

	slog.Debug("OAuth: Forwarding OAuth call", "method", req.Method, "target", u)

	breq, err := http.NewRequestWithContext(req.Context(), req.Method, u, req.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to make request: %s", err), http.StatusInternalServerError)
		return func() {}
	}

	breq.URL.RawQuery = req.URL.Query().Encode()
	breq.Header = req.Header.Clone()

	resp, err := client.Do(breq) // nolint: bodyclose
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return func() {}
	}

	for k, vs := range resp.Header {

		for _, v := range vs {

			if k == "Origin" || strings.HasPrefix(k, "Access-Control-") {
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
