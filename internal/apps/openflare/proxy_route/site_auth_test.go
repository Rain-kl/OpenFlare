// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package proxy_route

import (
	"testing"

	"github.com/Rain-kl/Wavelet/internal/model"
)

func TestApplyProxyRouteAuthPreservesAndRotatesKey(t *testing.T) {
	ttl := 24
	input := Input{EnableHTTPS: true, BasicAuthUsername: "visitor", BasicAuthPassword: "secret", SiteAuthTTLHours: &ttl}
	var route model.ProxyRoute
	if err := applyProxyRouteAuth(&route, &input); err != nil {
		t.Fatal(err)
	}
	key := route.SiteAuthKey
	if route.SiteAuthTTLHours != ttl || len(key) != 64 {
		t.Fatal("identity verification must store the requested lifetime and a random signing key")
	}

	route.BasicAuthUsername, route.BasicAuthPassword = input.BasicAuthUsername, input.BasicAuthPassword
	legacyInput := Input{EnableHTTPS: true}
	if err := applyProxyRouteAuth(&route, &legacyInput); err != nil {
		t.Fatal(err)
	}
	if route.SiteAuthTTLHours != ttl || route.SiteAuthKey != key || legacyInput.BasicAuthPassword != input.BasicAuthPassword {
		t.Error("an unrelated update from a legacy client must preserve identity verification")
	}

	input.BasicAuthPassword = "new-password"
	if err := applyProxyRouteAuth(&route, &input); err != nil {
		t.Fatal(err)
	}
	if route.SiteAuthKey == key {
		t.Error("changing the password must invalidate previously issued cookies")
	}
}
