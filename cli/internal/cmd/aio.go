package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/pkg/browser"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.acuvity.ai/minibridge/pkgs/backend"
	"go.acuvity.ai/minibridge/pkgs/frontend"
	"go.acuvity.ai/minibridge/pkgs/memconn"
	"golang.org/x/sync/errgroup"
)

var fAIO = pflag.NewFlagSet("aio", pflag.ExitOnError)

func init() {

	initSharedFlagSet()

	fAIO.StringP("listen", "l", "", "listen address of the bridge for incoming connections. If unset, stdio is used.")
	fAIO.String("endpoint-mcp", "/mcp", "when using HTTP, sets the endpoint to send messages (proto 2025-03-26).")
	fAIO.String("endpoint-messages", "/message", "when using HTTP, sets the endpoint to post messages (proto 2024-11-05).")
	fAIO.String("endpoint-sse", "/sse", "when using HTTP, sets the endpoint to connect to the event stream (proto 2024-11-05).")

	AIO.Flags().AddFlagSet(fAIO)
	AIO.Flags().AddFlagSet(fPolicer)
	AIO.Flags().AddFlagSet(fTLSServer)
	AIO.Flags().AddFlagSet(fHealth)
	AIO.Flags().AddFlagSet(fProfiler)
	AIO.Flags().AddFlagSet(fCORS)
	AIO.Flags().AddFlagSet(fAgentAuth)
	AIO.Flags().AddFlagSet(fSBOM)
	AIO.Flags().AddFlagSet(fMCP)
}

var AIO = &cobra.Command{
	Use:              "aio [flags] -- command [args...]",
	Short:            "Start an all-in-one minibridge frontend and backend",
	Args:             cobra.MinimumNArgs(1),
	SilenceUsage:     true,
	SilenceErrors:    true,
	TraverseChildren: true,

	RunE: func(cmd *cobra.Command, args []string) (err error) {

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		listen := viper.GetString("listen")
		mcpEndpoint := viper.GetString("endpoint-mcp")
		sseEndpoint := viper.GetString("endpoint-sse")
		messageEndpoint := viper.GetString("endpoint-messages")

		agentAuth, err := makeAgentAuth()
		if err != nil {
			return fmt.Errorf("unable to build auth: %w", err)
		}

		policer, penforce, err := makePolicer()
		if err != nil {
			return fmt.Errorf("unable to make policer: %w", err)
		}

		sbom, err := makeSBOM()
		if err != nil {
			return fmt.Errorf("unable to make hashes: %w", err)
		}

		tracer, err := makeTracer(ctx, "aio")
		if err != nil {
			return fmt.Errorf("unable to configure tracer: %w", err)
		}

		corsPolicy := makeCORSPolicy()

		mcpClient, err := makeMCPClient(args)
		if err != nil {
			return fmt.Errorf("unable to create MCP client: %w", err)
		}

		mm := startHealthServer(ctx)

		listener := memconn.NewListener()
		defer func() { _ = listener.Close() }()

		var eg errgroup.Group

		var mbackend backend.Backend
		eg.Go(func() error {

			defer cancel()

			slog.Info("Minibridge backend configured")

			mbackend = backend.NewWebSocket("self", nil, mcpClient,
				backend.OptListener(listener),
				backend.OptPolicer(policer),
				backend.OptPolicerEnforce(penforce),
				backend.OptDumpStderrOnError(viper.GetString("log-format") != "json"),
				backend.OptSBOM(sbom),
				backend.OptMetricsManager(mm),
				backend.OptTracer(tracer),
			)

			return mbackend.Start(ctx)
		})

		eg.Go(func() error {

			defer cancel()

			var proxy frontend.Frontend

			frontendServerTLSConfig, err := tlsConfigFromFlags(fTLSServer)
			if err != nil {
				return err
			}

			dialer := func(ctx context.Context, network, addr string) (net.Conn, error) {
				return listener.DialContext(cmd.Context(), "127.0.0.1:443")
			}

			if listen != "" {

				slog.Info("Minibridge frontend configured",
					"mcp", mcpEndpoint,
					"sse", sseEndpoint,
					"messages", messageEndpoint,
					"agent-token", agentAuth != nil,
					"mode", "http",
					"server-tls", frontendServerTLSConfig != nil,
					"server-mtls", mtlsMode(frontendServerTLSConfig),
					"listen", listen,
				)

				proxy = frontend.NewHTTP(listen, "ws://self/ws", frontendServerTLSConfig, nil,
					frontend.OptHTTPBackendDialer(dialer),
					frontend.OptHTTPMCPEndpoint(mcpEndpoint),
					frontend.OptHTTPSSEEndpoint(sseEndpoint),
					frontend.OptHTTPMessageEndpoint(messageEndpoint),
					frontend.OptHTTPAgentAuth(agentAuth),
					frontend.OptHTTPAgentTokenPassthrough(true),
					frontend.OptHTTPCORSPolicy(corsPolicy),
					frontend.OptHTTPMetricsManager(mm),
					frontend.OptHTTPTracer(tracer),
				)
			} else {

				slog.Info("Minibridge frontend configured",
					"mode", "stdio",
				)

				proxy = frontend.NewStdio("ws://self/ws", nil,
					frontend.OptStdioBackendDialer(dialer),
					frontend.OptStdioRetry(false),
					frontend.OptStioAgentAuth(agentAuth),
					frontend.OptStdioTracer(tracer),
				)
			}

			time.Sleep(300 * time.Millisecond)

			if err = proxy.Start(ctx); errors.Is(err, frontend.ErrAuthRequired) {

				if p, ok := mcpClient.(backend.OAuth2Provider); ok {

					mboauth := mbackend.(backend.OAuth2Provider)
					cl := &http.Client{
						Transport: &http.Transport{
							DialContext: func(ctx context.Context, net string, addr string) (net.Conn, error) {
								return listener.DialContext(ctx, "127.0.0.1:7987")
							},
						},
					}

					oreq := oauthRegistration{
						ClientName:          "minibridge",
						ClientURI:           "https://github.com/acuvity/minibridge",
						RedirectURI:         []string{"http://127.0.0.1:9977/callback"},
						TokenEndpointMethod: "none",
						GrantTypes:          []string{"authorization_code", "refresh_token"},
						ResponseTypes:       []string{"code"},
					}

					data, err := json.MarshalIndent(oreq, "", "  ")
					if err != nil {
						return err
					}

					u := fmt.Sprintf("%s/oauth2/register", mboauth.BaseURL())
					req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer(data))
					if err != nil {
						return err
					}
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("Accept", "application/json")

					resp, err := cl.Do(req)
					if err != nil {
						return err
					}

					data, err = io.ReadAll(resp.Body)
					if err != nil {
						return err
					}

					if err := json.Unmarshal(data, &oreq); err != nil {
						return err
					}

					values := url.Values{
						"response_type": {"code"},
						"client_id":     {oreq.ClientID},
						"redirect_uri":  {"http://127.0.0.1:9977/callback"},
					}
					u = fmt.Sprintf("%s/authorize?%s", p.BaseURL(), values.Encode())
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
						sctx, cancel := context.WithTimeout(ctx, 1*time.Second)
						defer cancel()
						server.Shutdown(sctx)
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
						return nil
					case <-time.After(10 * time.Minute):
						return fmt.Errorf("oauth timeout")
					}

					u = fmt.Sprintf("%s/oauth2/token", mboauth.BaseURL())

					form := url.Values{
						"grant_type":   {"authorization_code"},
						"client_id":    {oreq.ClientID},
						"code":         {code},
						"redirect_uri": {"http://127.0.0.1:9977/callback"},
					}

					req, err = http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewBuffer([]byte(form.Encode())))
					if err != nil {
						return err
					}
					req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
					req.Header.Set("Accept", "application/json")

					resp, err = cl.Do(req)
					if err != nil {
						return err
					}

					data, err = io.ReadAll(resp.Body)
					if err != nil {
						return err
					}

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("invalid status code: %s (%s)", resp.Status, string(data))
					}

					ex := oauthExchange{}
					if err := json.Unmarshal(data, &ex); err != nil {
						return err
					}

					fmt.Println("OAUTH DANCE COMPLETE: next time use", ex.AccessToken)
					return nil

				}

				return err
			}

			return err
		})

		return eg.Wait()
	},
}

type oauthRegistration struct {
	RedirectURI         []string `json:"redirect_uris"`
	ClientName          string   `json:"client_name"`
	ClientURI           string   `json:"client_uri,omitempty"`
	TokenEndpointMethod string   `json:"token_endpoint_auth_method"`
	LogoURI             string   `json:"logo_uri,omitempty"`
	ResponseTypes       []string `json:"response_types,omitempty"`
	GrantTypes          []string `json:"grant_types,omitempty"`

	ClientID             string `json:"client_id,omitempty"`
	RegistrationClientID string `json:"registration_client_uri,omitempty"`
	ClientIDIssuedAt     int    `json:"client_id_issued_at,omitempty"`
}

type oauthExchange struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}
