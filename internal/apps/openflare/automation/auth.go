// Package automation serves the OpenFlare CLI API, MCP tools, and OAuth flow.
package automation

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/oauth"
	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/Rain-kl/Wavelet/internal/infra/config"
	db "github.com/Rain-kl/Wavelet/internal/infra/persistence"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	authorizationPath = automationcontract.OAuthPath + "/authorize"
	tokenPath         = automationcontract.OAuthPath + "/token"
	registerPath      = automationcontract.OAuthPath + "/register"
	revokePath        = automationcontract.OAuthPath + "/revoke"
	metadataPath      = "/.well-known/oauth-protected-resource"
	issuerPath        = "/.well-known/oauth-authorization-server"
	connectPath       = "/automation/connect"
	cliClientID       = "openflare-cli"
	grantCode         = "authorization_code"
	grantRefresh      = "refresh_token"
	codeLifetime      = 5 * time.Minute
	accessLifetime    = 15 * time.Minute
	refreshLifetime   = 30 * 24 * time.Hour
	maxOAuthBody      = 16 << 10
	maxSignedPayload  = 8 << 10
)

var (
	errDisabled    = errors.New("automation is disabled")
	errInvalidAuth = errors.New("the automation credential is invalid or expired")
	errLoginNeeded = errors.New("a browser sign-in is required")
)

type instance struct {
	origin string
	secret string
}

func loadInstance(ctx context.Context) (instance, error) {
	setting, err := repository.GetSystemConfigByKey(ctx, model.ConfigKeyAutomationPublicURL)
	if errors.Is(err, gorm.ErrRecordNotFound) || err == nil && strings.TrimSpace(setting.Value) == "" {
		return instance{}, errDisabled
	}
	if err != nil {
		return instance{}, err
	}
	origin, err := automationcontract.HTTPSOrigin(setting.Value)
	if err != nil {
		return instance{}, err
	}
	secret := config.Config.App.SessionSecret
	const minSessionSecretLength = 32
	if len(secret) < minSessionSecretLength {
		return instance{}, errors.New("automation requires a session secret at least 32 bytes long")
	}
	return instance{origin: origin, secret: secret}, nil
}

func (site instance) id() string {
	return "of_" + site.sign("automation-instance", site.origin)[:22]
}

func (site instance) mcpResource() string { return site.origin + automationcontract.MCPPath }

func (site instance) apiResource() string {
	return site.origin + config.Config.App.APIPrefix + automationcontract.APISuffix
}

func (site instance) clientResource(clientID string) string {
	if clientID == cliClientID {
		return site.apiResource()
	}
	return site.mcpResource()
}

func (site instance) sign(purpose, input string) string {
	mac := hmac.New(sha256.New, []byte(site.secret))
	mac.Write([]byte(purpose))
	mac.Write([]byte{0})
	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (site instance) pack(purpose string, payload any) (string, error) {
	content, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(content)
	return encoded + "." + site.sign(purpose, encoded), nil
}

func (site instance) unpack(purpose, value string, dest any) bool {
	if len(value) > maxSignedPayload {
		return false
	}
	payload, signature, ok := strings.Cut(value, ".")
	if !ok || !hmac.Equal([]byte(site.sign(purpose, payload)), []byte(signature)) {
		return false
	}
	content, err := base64.RawURLEncoding.DecodeString(payload)
	return err == nil && json.Unmarshal(content, dest) == nil
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func randomCredential(prefix string) string {
	return prefix + base64.RawURLEncoding.EncodeToString(randBytes())
}

func randBytes() []byte {
	const tokenSize = 32
	value := make([]byte, tokenSize)
	rand.Read(value)
	return value
}

type identity struct {
	userID     uint64
	grantID    string
	scope      string
	instanceID string
}

func (actor identity) permits(scope string) bool {
	return slices.Contains(strings.Fields(actor.scope), scope)
}

var availableScopes = sync.OnceValue(func() []string {
	var scopes []string
	for _, operation := range automationcontract.Operations() {
		if !slices.Contains(scopes, operation.Scope) {
			scopes = append(scopes, operation.Scope)
		}
	}
	slices.Sort(scopes)
	return scopes
})

func normalizeScopes(raw string) (string, bool) {
	available := availableScopes()
	if raw == "" {
		return strings.Join(available, " "), true
	}
	scopes := strings.Fields(raw)
	for _, scope := range scopes {
		if !slices.Contains(available, scope) {
			return "", false
		}
	}
	slices.Sort(scopes)
	return strings.Join(slices.Compact(scopes), " "), len(scopes) != 0
}

func readScopes(scope string) string {
	fields := strings.Fields(scope)
	return strings.Join(slices.DeleteFunc(fields, func(value string) bool { return !strings.HasSuffix(value, ":read") }), " ")
}

func validAdmin(ctx context.Context, userID uint64) (model.User, error) {
	user, err := repository.GetActiveUserByID(db.WithPrimaryReads(ctx), userID)
	if err != nil {
		return model.User{}, err
	}
	if !user.IsAdmin || user.Username == "system" {
		return model.User{}, errInvalidAuth
	}
	return user, nil
}

func browserAdmin(c *gin.Context) (model.User, error) {
	if c.GetHeader("Authorization") != "" || c.GetHeader("X-Access-Token") != "" {
		return model.User{}, errInvalidAuth
	}
	if oauth.GetUserIDFromContext(c) == 0 {
		return model.User{}, errLoginNeeded
	}
	account, err := oauth.GetUserFromRequest(c)
	if err != nil {
		return model.User{}, errLoginNeeded
	}
	user, err := validAdmin(c.Request.Context(), account.ID)
	if err == nil && user.Password != account.Password {
		return model.User{}, errInvalidAuth
	}
	return user, err
}

func authenticate(request *http.Request, site instance, resource string) (identity, error) {
	headers := request.Header.Values("Authorization")
	if len(headers) != 1 {
		return identity{}, errInvalidAuth
	}
	fields := strings.Fields(headers[0])
	if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") || !strings.HasPrefix(fields[1], "ofa_") {
		return identity{}, errInvalidAuth
	}
	ctx := request.Context()
	grant, err := repository.GetAutomationAccessGrant(ctx, digest(fields[1]), resource, time.Now().UTC())
	if err != nil {
		return identity{}, err
	}
	if _, err := validAdmin(ctx, grant.UserID); err != nil {
		return identity{}, err
	}
	return identity{userID: grant.UserID, grantID: grant.ID, scope: grant.Scope, instanceID: site.id()}, nil
}

func isInvalidCredential(err error) bool {
	return errors.Is(err, errInvalidAuth) || errors.Is(err, gorm.ErrRecordNotFound)
}
