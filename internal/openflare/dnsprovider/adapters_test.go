// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dnsprovider

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func jsonReply(r *http.Request, body string) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

func TestAliDNSListAndWriteUseOfficialParameters(t *testing.T) {
	previous := aliHTTPClient
	t.Cleanup(func() { aliHTTPClient = previous })
	var actions []string
	aliHTTPClient = &http.Client{Transport: testTransport(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, aliEndpoint, r.URL.Host)
		params := r.URL.Query()
		action := params.Get("Action")
		version := params.Get("Version")
		for name, values := range r.Header {
			if strings.EqualFold(name, "x-acs-action") && len(values) > 0 {
				action = values[0]
			}
			if strings.EqualFold(name, "x-acs-version") && len(values) > 0 {
				version = values[0]
			}
		}
		assert.Equal(t, "2015-01-09", version)
		actions = append(actions, action)
		switch action {
		case "DescribeDomains":
			return jsonReply(r, `{"TotalCount":1,"Domains":{"Domain":[{"DomainName":"example.com"}]}}`)
		case "DescribeDomainRecords":
			assert.Equal(t, "COMBINATION", params.Get("SearchMode"))
			assert.Equal(t, "sdns", params.Get("RRKeyWord"))
			assert.Equal(t, "NS", params.Get("Type"))
			assert.Equal(t, "default", params.Get("Line"))
			return jsonReply(r, `{"TotalCount":1,"DomainRecords":{"Record":[{"RecordId":"ns1","RR":"sdns","Type":"NS","Value":"ns1.example.com","Line":"default","TTL":600}]}}`)
		case "AddDomainRecord":
			assert.Equal(t, "sdns", params.Get("RR"))
			assert.Equal(t, "example.com", params.Get("DomainName"))
			assert.Equal(t, "NS", params.Get("Type"))
			assert.Equal(t, "ns2.example.com", params.Get("Value"))
			assert.NotContains(t, params, "proxied")
			return jsonReply(r, `{"RecordId":"ns2"}`)
		}
		t.Errorf("unexpected AliDNS action %q", action)
		return jsonReply(r, `{}`)
	})}
	adapter, err := New(AliDNS, `{"access_key_id":"test-id","access_key_secret":"test-key"}`)
	require.NoError(t, err)
	zones, err := adapter.ListZones(t.Context())
	require.NoError(t, err)
	require.Equal(t, []Zone{{ID: "example.com", Name: "example.com"}}, zones)
	changes, err := ReconcileDelegation(t.Context(), adapter, zones[0], "sdns.example.com", []string{"ns1.example.com", "ns2.example.com"}, 600)
	require.NoError(t, err)
	assert.Equal(t, DelegationChanges{Created: 1}, changes)
	assert.Equal(t, []string{"DescribeDomains", "DescribeDomainRecords", "AddDomainRecord"}, actions)
}

func TestDNSPodWriteIgnoresProxyAndUsesDefaultLine(t *testing.T) {
	adapter, err := newTencentDNS(credentials{SecretID: "test-id", SecretKey: "test-key"})
	require.NoError(t, err)
	client, ok := adapter.(*tencentDNS)
	require.True(t, ok)
	client.client.WithHttpTransport(testTransport(func(r *http.Request) (*http.Response, error) {
		assert.Equal(t, "dnspod.tencentcloudapi.com", r.URL.Host)
		assert.Equal(t, "CreateRecord", r.Header.Get("X-TC-Action"))
		assert.Equal(t, "2021-03-23", r.Header.Get("X-TC-Version"))
		var payload map[string]any
		if !assert.NoError(t, json.NewDecoder(r.Body).Decode(&payload)) {
			return jsonReply(r, `{"Response":{}}`)
		}
		assert.Equal(t, "api", payload["SubDomain"])
		assert.Equal(t, "example.com", payload["Domain"])
		assert.Equal(t, defaultDNSLine, payload["RecordLine"])
		assert.Equal(t, float64(600), payload["TTL"])
		assert.NotContains(t, payload, "Proxied")
		return jsonReply(r, `{"Response":{"RecordId":23,"RequestId":"test"}}`)
	}))
	record, err := adapter.CreateRecord(t.Context(), "example.com", RecordInput{Type: "A", Name: "api.example.com", Content: "192.0.2.1", TTL: 1, Proxied: true})
	require.NoError(t, err)
	assert.Equal(t, "23", record.ID)
	assert.False(t, record.Proxied)
}
