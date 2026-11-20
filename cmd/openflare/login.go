package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"time"

	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/spf13/cobra"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const (
	callbackPath = "/callback"
	loginTimeout = 5 * time.Minute
)

func authCommand(options *cliOptions) *cobra.Command {
	var origin, prefix string
	auth := &cobra.Command{Use: "auth", Short: "Manage browser-authorized CLI connections"}
	login := &cobra.Command{
		Use: "login", Short: "Sign in to OpenFlare in a browser", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return loginProfile(command, options.profile, origin, prefix)
		},
	}
	login.Flags().StringVar(&origin, "server", "", "OpenFlare HTTPS origin")
	login.Flags().StringVar(&prefix, "api-prefix", "/api", "Configured OpenFlare API prefix")
	if err := login.MarkFlagRequired("server"); err != nil {
		panic(err)
	}
	logout := &cobra.Command{
		Use: "logout", Short: "Revoke and remove the saved grant", Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return logoutProfile(command.Context(), options.profile)
		},
	}
	auth.AddCommand(login, logout)
	return auth
}

func loginProfile(command *cobra.Command, name, rawOrigin, prefix string) error {
	origin, err := automationcontract.HTTPSOrigin(rawOrigin)
	if err != nil {
		return err
	}
	if !apiPrefix.MatchString(prefix) {
		return errors.New("invalid API prefix; expected a path such as /api")
	}
	profile := connectionProfile{Origin: origin, Prefix: prefix}
	ctx, cancel := context.WithTimeout(command.Context(), loginTimeout)
	defer cancel()
	endpoints, err := profile.discovery(ctx)
	if err != nil {
		return err
	}
	verifier, state := oauth2.GenerateVerifier(), oauth2.GenerateVerifier()
	callback, err := startCallback(ctx, state)
	if err != nil {
		return err
	}
	defer closeBody(callback.server)
	authConfig := oauth2.Config{
		ClientID: cliClientID, RedirectURL: callback.url,
		Endpoint: oauth2.Endpoint{AuthURL: endpoints.Authorization},
	}
	authorizationURL := authConfig.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier), oauth2.SetAuthURLParam("resource", profile.resource()))
	_, _ = fmt.Fprintf(command.ErrOrStderr(), "Complete browser authorization at %s\n", authorizationURL)
	openBrowser(ctx, authorizationURL)
	profile.Token, err = completeAuthorization(ctx, callback, endpoints.Token, profile.resource(), verifier)
	if err != nil {
		return err
	}
	data, err := profile.call(ctx, http.MethodGet, "/connection", "", nil)
	if err != nil {
		return err
	}
	var result struct {
		InstanceID string `json:"instance_id"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.InstanceID == "" {
		return errors.New("server did not return an instance ID")
	}
	profile.InstanceID = result.InstanceID
	if err := saveLogin(ctx, name, profile); err != nil {
		return err
	}
	return json.NewEncoder(command.OutOrStdout()).Encode(data)
}

type callbackResult struct {
	code string
	err  error
}

type browserCallback struct {
	server *http.Server
	url    string
	reply  chan callbackResult
}

func startCallback(ctx context.Context, state string) (browserCallback, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return browserCallback{}, err
	}
	host := listener.Addr().String()
	reply := make(chan callbackResult, 1)
	router := http.NewServeMux()
	router.HandleFunc("GET "+callbackPath, func(writer http.ResponseWriter, request *http.Request) {
		if request.Host != host || request.URL.Query().Get("state") != state {
			http.Error(writer, "Invalid authorization callback.", http.StatusBadRequest)
			return
		}
		writer.Header().Set("Cache-Control", "no-store")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		result := callbackResult{code: request.URL.Query().Get("code")}
		message := "Authorization received. Return to the OpenFlare CLI."
		if result.code == "" {
			result.err = errors.New("browser authorization was denied or returned no code")
			message = "Authorization was not completed. Return to the OpenFlare CLI."
		}
		select {
		case reply <- result:
			_, _ = fmt.Fprintln(writer, message)
		default:
			http.Error(writer, "This callback has already been received.", http.StatusConflict)
		}
	})
	server := &http.Server{Handler: router, ReadHeaderTimeout: requestTimeout, WriteTimeout: requestTimeout}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case reply <- callbackResult{err: err}:
			default:
			}
		}
	}()
	return browserCallback{server: server, url: "http://" + host + callbackPath, reply: reply}, nil
}

func completeAuthorization(ctx context.Context, callback browserCallback, tokenEndpoint, resource, verifier string) (oauthTokens, error) {
	select {
	case <-ctx.Done():
		return oauthTokens{}, ctx.Err()
	case result := <-callback.reply:
		if result.err != nil {
			return oauthTokens{}, result.err
		}
		return requestTokens(ctx, tokenEndpoint, url.Values{
			clientIDField: {cliClientID}, "grant_type": {"authorization_code"},
			"code": {result.code}, "redirect_uri": {callback.url}, "code_verifier": {verifier}, "resource": {resource},
		})
	}
}

//nolint:gosec // The verified HTTPS authorization URL is passed as one argument without a shell.
func openBrowser(ctx context.Context, address string) {
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "/usr/bin/open", address)
	case "windows":
		command = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", address)
	default:
		command = exec.CommandContext(ctx, "xdg-open", address)
	}
	go func() { _ = command.Run() }()
}

func saveLogin(ctx context.Context, name string, profile connectionProfile) error {
	lock, err := profileLock(ctx, name)
	if err != nil {
		return err
	}
	defer closeBody(lock)
	current, err := keyring.Get(keyringService, name)
	if err == nil {
		var previous connectionProfile
		if err := json.Unmarshal([]byte(current), &previous); err != nil {
			return err
		}
		if previous.InstanceID != profile.InstanceID || previous.resource() != profile.resource() {
			return errors.New("profile is linked to another instance; choose a new profile name")
		}
		if err := previous.revoke(ctx); err != nil {
			return err
		}
	} else if !errors.Is(err, keyring.ErrNotFound) {
		return err
	}
	return storeProfile(name, profile)
}

func logoutProfile(ctx context.Context, name string) error {
	lock, err := profileLock(ctx, name)
	if err != nil {
		return err
	}
	defer closeBody(lock)
	profile, err := loadProfile(name)
	if err != nil {
		return err
	}
	if err := profile.revoke(ctx); err != nil {
		return err
	}
	return keyring.Delete(keyringService, name)
}
