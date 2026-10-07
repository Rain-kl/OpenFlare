// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package model

import (
	"encoding/json"
	"testing"
)

func TestOAuthUserInfoIgnoresProviderID(t *testing.T) {
	for _, providerID := range []string{`"a27dfc56-07ae-4c9d-9e5c-99103bd8805f"`, `"88888"`, `88888`, `null`} {
		t.Run(providerID, func(t *testing.T) {
			payload := `{"id":` + providerID + `,"sub":"external-subject","preferred_username":"oidc-user","email":"oidc@example.com","name":"OIDC User"}`
			var info OAuthUserInfo
			if err := json.Unmarshal([]byte(payload), &info); err != nil {
				t.Fatalf("json.Unmarshal(OAuthUserInfo, id=%s) error = %v, want nil", providerID, err)
			}
			want := OAuthUserInfo{
				Sub:               "external-subject",
				PreferredUsername: "oidc-user",
				Email:             "oidc@example.com",
				Name:              "OIDC User",
			}
			if info != want {
				t.Errorf("json.Unmarshal(OAuthUserInfo, id=%s) = %+v, want %+v", providerID, info, want)
			}
		})
	}
}
