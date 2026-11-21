package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

func TestSiteCommandUsesDefaultOrNamedConnection(t *testing.T) {
	const instanceID = "of_cli_test"
	var captured *http.Request
	var body []byte
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		captured = request
		body, _ = io.ReadAll(request.Body)
		writer.Header().Set(automationcontract.InstanceHeader, instanceID)
		_, _ = io.WriteString(writer, `{"error_msg":"","data":{"state":"saved_on_controller"}}`)
	}))
	t.Cleanup(server.Close)
	previous := httpClient
	httpClient = server.Client()
	t.Cleanup(func() { httpClient = previous })
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	keyring.MockInit()
	require.NoError(t, storeProfile("production", connectionProfile{
		Origin: server.URL, Prefix: "/api", InstanceID: instanceID,
		Token: oauthTokens{AccessToken: "test-access", ExpiresAt: time.Now().Add(time.Minute)},
	}))

	invoke := func(arguments ...string) (string, error) {
		command := rootCommand()
		var output bytes.Buffer
		command.SetOut(&output)
		command.SetErr(&output)
		command.SetArgs(arguments)
		err := command.ExecuteContext(t.Context())
		return output.String(), err
	}
	args := []string{"site", "create", "--site-name", "Example", "--enable-https", "false", "--domain-record-ids", `["42"]`, "--upstreams", `["https://origin.example.com"]`}
	_, err := invoke(args...)
	require.ErrorContains(t, err, `profile "default" is not signed in`)
	require.NoError(t, storeProfile(defaultProfile, connectionProfile{
		Origin: server.URL, Prefix: "/api", InstanceID: instanceID,
		Token: oauthTokens{AccessToken: "default-access", ExpiresAt: time.Now().Add(time.Minute)},
	}))
	_, err = invoke(append(append([]string{}, args...), "--profile", "production", "--site-name", "Duplicate")...)
	require.ErrorContains(t, err, "only be specified once")
	_, err = invoke(append(append([]string{}, args...), "--profile", "production", "--profile", "staging")...)
	require.ErrorContains(t, err, "only be specified once")
	require.Nil(t, captured)
	output, err := invoke(args...)
	require.NoError(t, err)
	require.JSONEq(t, `{"state":"saved_on_controller"}`, output)
	require.Equal(t, http.MethodPost, captured.Method)
	require.Equal(t, "/api/v1/automation/sites", captured.URL.Path)
	require.Equal(t, instanceID, captured.Header.Get(automationcontract.InstanceHeader))
	require.Equal(t, "Bearer default-access", captured.Header.Get("Authorization"))
	require.NotEmpty(t, captured.Header.Get(automationcontract.ContractHeader))
	require.JSONEq(t, `{"site_name":"Example","enable_https":false,"domain_record_ids":["42"],"upstreams":["https://origin.example.com"]}`, string(body))
	_, err = invoke(append(args, "--profile", "production")...)
	require.NoError(t, err)
	require.Equal(t, "Bearer test-access", captured.Header.Get("Authorization"))
}
