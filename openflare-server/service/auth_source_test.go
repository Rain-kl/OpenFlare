package service

import (
	"testing"

	"github.com/rain-kl/openflare/openflare-server/model"
)

func TestCompleteOAuthLoginRequiresLinkWhenRegistrationDisabled(t *testing.T) {
	setupServiceTestDB(t)

	source := createTestAuthSource(t)
	result, pending, err := CompleteOAuthLogin(source, &OAuthProfile{
		ExternalID:       "external-1",
		ExternalUsername: "external-user",
		DisplayName:      "External User",
		Email:            "external@example.com",
	}, nil)
	if err != nil {
		t.Fatalf("CompleteOAuthLogin failed: %v", err)
	}
	if result.Status != "link_required" || pending == nil {
		t.Fatalf("expected link_required with pending account, got %#v pending=%#v", result, pending)
	}

	user, err := LinkPendingExternalAccount(pending, LinkExistingRequest{
		Username: "root",
		Password: "123456",
	})
	if err != nil {
		t.Fatalf("LinkPendingExternalAccount failed: %v", err)
	}
	if user.Username != "root" {
		t.Fatalf("expected root user, got %s", user.Username)
	}
	account, err := model.FindExternalAccount(source.ID, "external-1")
	if err != nil {
		t.Fatalf("expected external account to be linked: %v", err)
	}
	if account.UserID != user.Id {
		t.Fatalf("expected external account user %d, got %d", user.Id, account.UserID)
	}
}

func createTestAuthSource(t *testing.T) *model.AuthSource {
	t.Helper()
	source := &model.AuthSource{
		Name:               "test-oidc",
		Type:               model.AuthSourceTypeOIDC,
		DisplayName:        "Test OIDC",
		ClientID:           "client-id",
		ClientSecret:       "client-secret",
		Scopes:             "openid profile email",
		OpenIDDiscoveryURL: "https://idp.example.com/.well-known/openid-configuration",
	}
	if err := model.CreateAuthSource(source); err != nil {
		t.Fatalf("CreateAuthSource failed: %v", err)
	}
	return source
}
