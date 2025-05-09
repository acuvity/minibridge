package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/gofrs/uuid"
	"github.com/pkg/browser"
	"go.acuvity.ai/minibridge/pkgs/info"
)

type RegistrationRequest struct {
	RedirectURI         []string `json:"redirect_uris"`
	ClientName          string   `json:"client_name"`
	ClientURI           string   `json:"client_uri,omitempty"`
	TokenEndpointMethod string   `json:"token_endpoint_auth_method"`
	LogoURI             string   `json:"logo_uri,omitempty"`
	ResponseTypes       []string `json:"response_types,omitempty"`
	GrantTypes          []string `json:"grant_types,omitempty"`
}

type RegistrationResponse struct {
	ClientID             string `json:"client_id,omitempty"`
	RegistrationClientID string `json:"registration_client_uri,omitempty"`
	ClientIDIssuedAt     int    `json:"client_id_issued_at,omitempty"`
}

type Credentials struct {
	ClientID     string `json:"client_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Refresh performs a refresh of the access token using the RefreshToken in the gviven Creds
func Refresh(ctx context.Context, backendURL string, cl *http.Client, int info.Info, rt Credentials) (t Credentials, err error) {

	u := fmt.Sprintf("%s/oauth2/token", backendURL)

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {rt.RefreshToken},
		"client_id":     {rt.ClientID},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer([]byte(form.Encode())))
	if err != nil {
		return t, fmt.Errorf("unable to make token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := cl.Do(req)
	if err != nil {
		return t, fmt.Errorf("unable to send token request: %w", err)
	}
	defer func(r *http.Response) { _ = r.Body.Close() }(resp)

	data, err := io.ReadAll(resp.Body)
	if err != nil && !errors.Is(err, io.EOF) {
		return t, fmt.Errorf("unable to read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return t, fmt.Errorf("invalid token response status code: %s (%s)", resp.Status, string(data))
	}

	if err := json.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("unable to unmarshal token response body: %w", err)
	}

	t.ClientID = rt.ClientID

	return t, nil
}

func Dance(ctx context.Context, backendURL string, cl *http.Client, inf info.Info) (t Credentials, err error) {

	redirectURI := "http://127.0.0.1:9977/callback"
	clientID := ""
	state := uuid.Must(uuid.NewV7()).String()

	if inf.OAuthRegister {

		oreq := RegistrationRequest{
			ClientName:          "minibridge",
			ClientURI:           "https://github.com/acuvity/minibridge",
			TokenEndpointMethod: "none",
			RedirectURI:         []string{redirectURI},
			GrantTypes:          []string{"authorization_code", "refresh_token"},
			ResponseTypes:       []string{"code"},
		}

		data, err := json.MarshalIndent(oreq, "", "  ")
		if err != nil {
			return t, fmt.Errorf("unable to marshal registration request: %w", err)
		}

		u := fmt.Sprintf("%s/oauth2/register", backendURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(data))
		if err != nil {
			return t, fmt.Errorf("unable to build registration request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := cl.Do(req)
		if err != nil {
			return t, fmt.Errorf("unable to send registration request: %w", err)
		}
		defer func(r *http.Response) { _ = r.Body.Close() }(resp)

		data, err = io.ReadAll(resp.Body)
		if err != nil && !errors.Is(err, io.EOF) {
			return t, fmt.Errorf("unable to read registration response body: %w", err)
		}

		if resp.StatusCode != http.StatusCreated {
			return t, fmt.Errorf("invalid registration response status code: %s (%s)", resp.Status, string(data))
		}

		oresp := RegistrationResponse{}
		if err := json.Unmarshal(data, &oresp); err != nil {
			return t, fmt.Errorf("unable to unmarshal registration response body: %w", err)
		}

		clientID = oresp.ClientID
	}

	values := url.Values{
		"response_type": {"code"},
		"client_id":     {clientID},
		"redirect_uri":  {redirectURI},
		"state":         {state},
	}

	u := fmt.Sprintf("%s/authorize?%s", inf.Server, values.Encode())
	if err := browser.OpenURL(u); err != nil {
		fmt.Println("Open the following URL in your browser:", u)
	}

	codeCh := make(chan string, 1)

	server := &http.Server{
		ReadHeaderTimeout: 3 * time.Second,
		Addr:              "127.0.0.1:9977",
	}

	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

		codeCh <- req.URL.Query().Get("code")

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(fmt.Appendf([]byte{}, successBody, inf.Server))

		sctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()

		_ = server.Shutdown(sctx)
	})

	go func() {
		if err := server.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Unable to start oauth callback server", err)
				return
			}
		}
	}()

	var code string
	select {
	case code = <-codeCh:
	case <-ctx.Done():
		return t, ctx.Err()
	case <-time.After(10 * time.Minute):
		return t, fmt.Errorf("oauth timeout")
	}

	u = fmt.Sprintf("%s/oauth2/token", backendURL)

	form := url.Values{
		"grant_type":   {"authorization_code"},
		"client_id":    {clientID},
		"code":         {code},
		"redirect_uri": {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer([]byte(form.Encode())))
	if err != nil {
		return t, fmt.Errorf("unable to make token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := cl.Do(req)
	if err != nil {
		return t, fmt.Errorf("unable to send token request: %w", err)
	}
	defer func(r *http.Response) { _ = r.Body.Close() }(resp)

	data, err := io.ReadAll(resp.Body)
	if err != nil && !errors.Is(err, io.EOF) {
		return t, fmt.Errorf("unable to read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return t, fmt.Errorf("invalid token response status code: %s (%s)", resp.Status, string(data))
	}

	if err := json.Unmarshal(data, &t); err != nil {
		return t, fmt.Errorf("unable to unmarshal token response body: %w", err)
	}

	t.ClientID = clientID

	return t, nil
}
