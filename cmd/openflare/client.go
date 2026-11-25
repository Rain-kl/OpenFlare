// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/gofrs/flock"
	"github.com/zalando/go-keyring"
)

const (
	keyringService    = "net.arctel.openflare.cli"
	cliClientID       = "openflare-cli"
	clientIDField     = "client_id"
	refreshTokenField = "refresh_token"
	requestTimeout    = 20 * time.Second
	tokenRefreshSkew  = 30 * time.Second
	responseLimit     = 4 << 20
	lockDirectoryMode = 0o700
)

var profileName = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
var apiPrefix = regexp.MustCompile(`^(/[a-zA-Z0-9_-]+)+$`)

var httpClient = &http.Client{
	Timeout: requestTimeout,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type connectionProfile struct {
	Origin     string      `json:"origin"`
	Prefix     string      `json:"api_prefix"`
	InstanceID string      `json:"instance_id"`
	Token      oauthTokens `json:"token"`
}

type oauthTokens struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type declaredEndpoints struct {
	Issuer        string `json:"issuer"`
	Authorization string `json:"authorization_endpoint"`
	Token         string `json:"token_endpoint"`
	Revocation    string `json:"revocation_endpoint"`
}

func (connection connectionProfile) resource() string {
	return connection.Origin + connection.Prefix + automationcontract.APISuffix
}

func (connection connectionProfile) discovery(ctx context.Context) (declaredEndpoints, error) {
	var endpoints declaredEndpoints
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, connection.Origin+"/.well-known/oauth-authorization-server", nil)
	if err != nil {
		return endpoints, err
	}
	if err := oauthRequest(request, &endpoints); err != nil {
		return endpoints, err
	}
	if endpoints.Issuer != connection.Origin {
		return endpoints, errors.New("OAuth issuer differs from the configured origin")
	}
	for _, endpoint := range []string{endpoints.Authorization, endpoints.Token, endpoints.Revocation} {
		if !validEndpoint(connection.Origin, endpoint) {
			return endpoints, errors.New("server advertises an invalid or cross-origin OAuth endpoint")
		}
	}
	return endpoints, nil
}

func validEndpoint(origin, endpoint string) bool {
	parsed, err := url.Parse(endpoint)
	return err == nil && parsed.User == nil && parsed.Opaque == "" && len(parsed.Path) > 1 &&
		parsed.Fragment == "" && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Scheme+"://"+parsed.Host == origin
}

func requestTokens(ctx context.Context, endpoint string, values url.Values) (oauthTokens, error) {
	var tokens oauthTokens
	request, err := formRequest(ctx, endpoint, values)
	if err != nil {
		return tokens, err
	}
	if err := oauthRequest(request, &tokens); err != nil {
		return tokens, err
	}
	if !strings.EqualFold(tokens.TokenType, "Bearer") || tokens.AccessToken == "" || tokens.RefreshToken == "" ||
		tokens.ExpiresIn <= 0 || tokens.ExpiresIn > int64(time.Hour/time.Second) {
		return tokens, errors.New("server returned an invalid OAuth token response")
	}
	tokens.ExpiresAt = time.Now().Add(time.Duration(tokens.ExpiresIn) * time.Second)
	return tokens, nil
}

func formRequest(ctx context.Context, endpoint string, values url.Values) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(values.Encode()))
	if err == nil {
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return request, err
}

func oauthRequest(request *http.Request, result any) error {
	reply, err := httpClient.Do(request)
	if err != nil {
		return err
	}
	defer closeBody(reply.Body)
	if reply.StatusCode != http.StatusOK {
		var failure struct {
			Code string `json:"error"`
		}
		_ = json.NewDecoder(io.LimitReader(reply.Body, responseLimit)).Decode(&failure)
		return fmt.Errorf("OAuth request failed (HTTP %d %s)", reply.StatusCode, failure.Code)
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(reply.Body, responseLimit)).Decode(result)
}

func (connection connectionProfile) revoke(ctx context.Context) error {
	endpoints, err := connection.discovery(ctx)
	if err != nil {
		return err
	}
	request, err := formRequest(ctx, endpoints.Revocation, url.Values{
		clientIDField: {cliClientID}, "token": {connection.Token.RefreshToken}, "token_type_hint": {refreshTokenField},
	})
	if err != nil {
		return err
	}
	return oauthRequest(request, nil)
}

func (connection connectionProfile) call(ctx context.Context, method, path, revision string, body json.RawMessage) (json.RawMessage, error) {
	request, err := http.NewRequestWithContext(ctx, method, connection.resource()+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", "Bearer "+connection.Token.AccessToken)
	if method != http.MethodGet {
		request.Header.Set(automationcontract.InstanceHeader, connection.InstanceID)
		request.Header.Set(automationcontract.ContractHeader, revision)
		if len(body) > 0 {
			request.Header.Set("Content-Type", "application/json")
		}
	}
	reply, err := httpClient.Do(request)
	if err != nil {
		if method != http.MethodGet {
			return nil, fmt.Errorf("OUTCOME_UNKNOWN: verify the resource before retrying: %w", err)
		}
		return nil, err
	}
	defer closeBody(reply.Body)
	var envelope struct {
		Error string          `json:"error_msg"`
		Data  json.RawMessage `json:"data"`
	}
	decodeErr := json.NewDecoder(io.LimitReader(reply.Body, responseLimit)).Decode(&envelope)
	if reply.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenFlare HTTP %d %s: %s (request %s)", reply.StatusCode,
			reply.Header.Get("X-OpenFlare-Error-Code"), envelope.Error, reply.Header.Get("X-OpenFlare-Request-Id"))
	}
	if connection.InstanceID != "" && reply.Header.Get(automationcontract.InstanceHeader) != connection.InstanceID {
		return nil, errors.New("the server instance ID differs from the saved profile")
	}
	if decodeErr != nil {
		return nil, decodeErr
	}
	if envelope.Error != "" || len(envelope.Data) == 0 {
		return nil, fmt.Errorf("invalid API response: %s", envelope.Error)
	}
	return envelope.Data, nil
}

func closeBody(body io.Closer) { _ = body.Close() }

func loadProfile(name string) (connectionProfile, error) {
	var profile connectionProfile
	data, err := keyring.Get(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return profile, fmt.Errorf("profile %q is not signed in; run openflare auth login --help", name)
	}
	if err != nil {
		return profile, fmt.Errorf("read system credential store: %w", err)
	}
	if err := json.Unmarshal([]byte(data), &profile); err != nil {
		return profile, err
	}
	if origin, err := automationcontract.HTTPOrigin(profile.Origin); err != nil || origin != profile.Origin ||
		!apiPrefix.MatchString(profile.Prefix) || profile.InstanceID == "" {
		return profile, errors.New("the stored connection profile is invalid")
	}
	return profile, nil
}

func storeProfile(name string, profile connectionProfile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	return keyring.Set(keyringService, name, string(data))
}

func profileLock(ctx context.Context, name string) (*flock.Flock, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	directory := filepath.Join(cache, "openflare", "locks")
	if err := os.MkdirAll(directory, lockDirectoryMode); err != nil {
		return nil, err
	}
	lock := flock.New(filepath.Join(directory, name+".lock"))
	const retryWait = 100 * time.Millisecond
	lockCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	locked, err := lock.TryLockContext(lockCtx, retryWait)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, errors.New("could not lock the connection profile")
	}
	return lock, nil
}

func activeConnection(ctx context.Context, name string) (connectionProfile, error) {
	lock, err := profileLock(ctx, name)
	if err != nil {
		return connectionProfile{}, err
	}
	defer closeBody(lock)
	profile, err := loadProfile(name)
	if err != nil || time.Until(profile.Token.ExpiresAt) > tokenRefreshSkew {
		return profile, err
	}
	endpoints, err := profile.discovery(ctx)
	if err != nil {
		return profile, err
	}
	tokens, err := requestTokens(ctx, endpoints.Token, url.Values{
		clientIDField: {cliClientID}, "grant_type": {refreshTokenField},
		refreshTokenField: {profile.Token.RefreshToken}, "resource": {profile.resource()},
	})
	if err != nil {
		return profile, fmt.Errorf("refresh profile %q: %w; sign in again if the grant has expired", name, err)
	}
	profile.Token = tokens
	return profile, storeProfile(name, profile)
}
