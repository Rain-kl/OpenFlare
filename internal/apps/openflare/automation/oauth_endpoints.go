package automation

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/model"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type oauthHandler func(*gin.Context, instance)

var redirectHost = regexp.MustCompile(`^[A-Za-z0-9.-]+(:[0-9]{1,5})?$`)

const (
	oauthCode        = "code"
	publicClientAuth = "none"
	consentRead      = "read"
	consentOperate   = "operate"
)

func withInstance(handler oauthHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		site, err := loadInstance(c.Request.Context())
		switch {
		case errors.Is(err, errDisabled):
			c.Status(http.StatusNotFound)
		case err != nil:
			logger.ErrorF(c.Request.Context(), "Load automation configuration: %v", err)
			oauthError(c, http.StatusServiceUnavailable, "temporarily_unavailable")
		default:
			c.Header("Cache-Control", "no-store")
			c.Header("Referrer-Policy", "no-referrer")
			handler(c, site)
		}
	}
}

// AuthorizationMetadata publishes the OAuth server discovery document.
func AuthorizationMetadata() gin.HandlerFunc {
	return withInstance(func(c *gin.Context, site instance) {
		c.JSON(http.StatusOK, gin.H{
			"issuer":                                     site.origin,
			"authorization_endpoint":                     site.origin + authorizationPath,
			"token_endpoint":                             site.origin + tokenPath,
			"registration_endpoint":                      site.origin + registerPath,
			"revocation_endpoint":                        site.origin + revokePath,
			"response_types_supported":                   []string{oauthCode},
			"grant_types_supported":                      []string{grantCode, grantRefresh},
			"code_challenge_methods_supported":           []string{"S256"},
			"token_endpoint_auth_methods_supported":      []string{publicClientAuth},
			"revocation_endpoint_auth_methods_supported": []string{publicClientAuth},
			"scopes_supported":                           availableScopes(),
		})
	})
}

// ProtectedResourceMetadata maps the requested transport to its OAuth audience.
func ProtectedResourceMetadata(mcp bool) gin.HandlerFunc {
	return withInstance(func(c *gin.Context, site instance) {
		resource := site.apiResource()
		if mcp {
			resource = site.mcpResource()
		}
		c.JSON(http.StatusOK, gin.H{
			"resource": resource, "authorization_servers": []string{site.origin},
			"scopes_supported": availableScopes(), "bearer_methods_supported": []string{"header"},
		})
	})
}

type publicClient struct {
	Name        string `json:"client_name"`
	RedirectURI string `json:"redirect_uri"`
}

type registrationRequest struct {
	ClientName        string   `json:"client_name"`
	RedirectURIs      []string `json:"redirect_uris"`
	TokenEndpointAuth string   `json:"token_endpoint_auth_method"`
}

func registerClient(c *gin.Context, site instance) {
	var registration registrationRequest
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOAuthBody)
	if err := json.NewDecoder(c.Request.Body).Decode(&registration); err != nil ||
		len(registration.RedirectURIs) != 1 || !validMCPRedirect(registration.RedirectURIs[0]) {
		oauthError(c, http.StatusBadRequest, "invalid_redirect_uri")
		return
	}
	name := strings.TrimSpace(registration.ClientName)
	const maxClientNameLength = 100
	if len(name) > maxClientNameLength || strings.ContainsFunc(name, unicode.IsControl) ||
		(registration.TokenEndpointAuth != "" && registration.TokenEndpointAuth != publicClientAuth) {
		oauthError(c, http.StatusBadRequest, "invalid_client_metadata")
		return
	}
	if name == "" {
		name = "MCP client"
	}
	clientID, err := site.pack("automation-mcp-client:"+site.origin, publicClient{Name: name, RedirectURI: registration.RedirectURIs[0]})
	if err != nil {
		oauthInternalError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"client_id": clientID, "client_name": name, "redirect_uris": registration.RedirectURIs,
		"token_endpoint_auth_method": publicClientAuth, "grant_types": []string{grantCode, grantRefresh},
		"response_types": []string{oauthCode}, "scope": strings.Join(availableScopes(), " "),
	})
}

func validMCPRedirect(raw string) bool {
	if validLoopbackRedirect(raw) {
		return true
	}
	const maxRedirectLength = 768
	if len(raw) > maxRedirectLength || strings.ContainsAny(raw, "\\%") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Opaque != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.Path == "" {
		return false
	}
	return redirectHost.MatchString(parsed.Host) && strings.Contains(parsed.Hostname(), ".")
}

func validLoopbackRedirect(raw string) bool {
	const maxRedirectLength = 768
	if len(raw) == 0 || len(raw) > maxRedirectLength || strings.ContainsAny(raw, "\\%") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "http" || parsed.User != nil || parsed.Opaque != "" ||
		parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/") {
		return false
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil || port == 0 {
		return false
	}
	host := parsed.Hostname()
	return host == "localhost" || net.ParseIP(host).IsLoopback()
}

func (site instance) client(clientID, redirect string) (publicClient, bool) {
	if clientID == cliClientID {
		valid := validLoopbackRedirect(redirect)
		if valid {
			uri, _ := url.Parse(redirect)
			valid = uri.Path == "/callback" && uri.Hostname() == "127.0.0.1"
		}
		return publicClient{Name: "OpenFlare CLI", RedirectURI: redirect}, valid
	}
	var client publicClient
	if !site.unpack("automation-mcp-client:"+site.origin, clientID, &client) {
		return publicClient{}, false
	}
	return client, client.RedirectURI == redirect
}

type authorizationRequest struct {
	ClientID    string `json:"client_id"`
	RedirectURI string `json:"redirect_uri"`
	Scope       string `json:"scope"`
	State       string `json:"state"`
	Challenge   string `json:"challenge"`
	Expires     int64  `json:"exp"`
	UserID      uint64 `json:"user_id,omitempty"`
	Nonce       string `json:"nonce,omitempty"`
}

func readAuthorization(site instance, values url.Values) (authorizationRequest, publicClient, bool) {
	request := authorizationRequest{
		ClientID: values.Get("client_id"), RedirectURI: values.Get("redirect_uri"),
		State: values.Get("state"), Challenge: values.Get("code_challenge"),
		Expires: time.Now().Add(codeLifetime).Unix(),
	}
	client, validClient := site.client(request.ClientID, request.RedirectURI)
	scope, validScope := normalizeScopes(values.Get("scope"))
	challenge, err := base64.RawURLEncoding.Strict().DecodeString(request.Challenge)
	resource := site.clientResource(request.ClientID)
	const maxStateLength = 1024
	if !validClient || !validScope || err != nil || len(challenge) != sha256Size ||
		len(request.State) == 0 || len(request.State) > maxStateLength ||
		values.Get("response_type") != oauthCode || values.Get("code_challenge_method") != "S256" ||
		!matchesResource(values, resource) || duplicateValues(values) {
		return authorizationRequest{}, publicClient{}, false
	}
	request.Scope = scope
	return request, client, true
}

const sha256Size = 32

func matchesResource(values url.Values, expected string) bool {
	resources, present := values["resource"]
	return !present || (len(resources) == 1 && resources[0] == expected)
}

func duplicateValues(values url.Values) bool {
	for _, items := range values {
		if len(items) != 1 {
			return true
		}
	}
	return false
}

func authorize(c *gin.Context, site instance) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	request, client, ok := readAuthorization(site, values)
	if !ok {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	user, err := browserAdmin(c)
	if err != nil {
		handleConsentLogin(c, err)
		return
	}
	cookie, err := c.Request.Cookie(config.Config.App.SessionCookieName)
	if err != nil {
		oauthError(c, http.StatusUnauthorized, "login_required")
		return
	}
	request.UserID, request.Nonce = user.ID, randomCredential("")
	intent, err := site.pack(site.consentPurpose(cookie.Value), request)
	if err != nil {
		oauthInternalError(c, err)
		return
	}
	showConsent(c, site, client, request, intent)
}

func handleConsentLogin(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errLoginNeeded):
		target := c.Request.URL.RequestURI()
		c.Redirect(http.StatusFound, "/login?"+url.Values{"callbackUrl": {target}}.Encode())
	case isInvalidCredential(err):
		oauthError(c, http.StatusForbidden, "access_denied")
	default:
		oauthInternalError(c, err)
	}
}

func (site instance) consentPurpose(cookie string) string {
	return "automation-consent:" + site.origin + ":" + cookie
}

func parseOAuthForm(c *gin.Context) (url.Values, bool) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxOAuthBody)
	if err := c.Request.ParseForm(); err != nil || c.Request.URL.RawQuery != "" || duplicateValues(c.Request.PostForm) {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return nil, false
	}
	return c.Request.PostForm, true
}

func confirmConsent(c *gin.Context, site instance) {
	values, ok := parseOAuthForm(c)
	if !ok {
		return
	}
	user, err := browserAdmin(c)
	if err != nil || c.GetHeader("Origin") != site.origin {
		oauthError(c, http.StatusForbidden, "access_denied")
		return
	}
	cookie, cookieErr := c.Request.Cookie(config.Config.App.SessionCookieName)
	intent := values.Get("intent")
	var request authorizationRequest
	now := time.Now().UTC()
	if cookieErr != nil || !site.unpack(site.consentPurpose(cookie.Value), intent, &request) ||
		request.UserID != user.ID || request.Expires <= now.Unix() {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	handleConsentDecision(c, site, request, intent, values.Get("decision"), now)
}

func handleConsentDecision(c *gin.Context, site instance, request authorizationRequest, intent, decision string, now time.Time) {
	redirect, err := url.Parse(request.RedirectURI)
	if err != nil {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	query := url.Values{"state": {request.State}}
	if decision == consentRead || decision == consentOperate {
		scope := request.Scope
		if decision == consentRead {
			scope = readScopes(scope)
		}
		if scope == "" {
			oauthError(c, http.StatusBadRequest, "invalid_scope")
			return
		}
		code := site.sign("automation-code", intent)
		row := model.AutomationCode{
			Hash: digest(code), UserID: request.UserID, ClientID: request.ClientID,
			RedirectURI: request.RedirectURI, Resource: site.clientResource(request.ClientID),
			Scope: scope, Challenge: request.Challenge, ExpiresAt: time.Unix(request.Expires, 0).UTC(),
		}
		if err := repository.CreateAutomationCode(c.Request.Context(), row, now); err != nil {
			oauthInternalError(c, err)
			return
		}
		query.Set(oauthCode, code)
	} else {
		query.Set("error", "access_denied")
	}
	redirect.RawQuery = query.Encode()
	c.Redirect(http.StatusSeeOther, redirect.String())
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func issueToken(c *gin.Context, site instance) {
	values, ok := parseOAuthForm(c)
	if !ok {
		return
	}
	if c.GetHeader("Authorization") != "" || values.Get("client_secret") != "" {
		oauthError(c, http.StatusBadRequest, "invalid_client")
		return
	}
	clientID := values.Get("client_id")
	resource := site.clientResource(clientID)
	if clientID == "" || !matchesResource(values, resource) {
		oauthError(c, http.StatusBadRequest, "invalid_target")
		return
	}
	now := time.Now().UTC()
	var result tokenResponse
	var err error
	switch values.Get("grant_type") {
	case grantCode:
		result, err = exchangeCode(c.Request, values, clientID, resource, now)
	case grantRefresh:
		result, err = exchangeRefresh(c.Request, values, clientID, resource, now)
	default:
		oauthError(c, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	switch {
	case isInvalidCredential(err):
		oauthError(c, http.StatusBadRequest, "invalid_grant")
	case err != nil:
		oauthInternalError(c, err)
	default:
		c.JSON(http.StatusOK, result)
	}
}

func exchangeCode(request *http.Request, values url.Values, clientID, resource string, now time.Time) (tokenResponse, error) {
	ctx := request.Context()
	code, err := repository.GetAutomationCode(ctx, digest(values.Get(oauthCode)))
	if err != nil {
		return tokenResponse{}, err
	}
	verifier := values.Get("code_verifier")
	if code.Used || !code.ExpiresAt.After(now) || code.ClientID != clientID || code.Resource != resource ||
		code.RedirectURI != values.Get("redirect_uri") || !verifiesChallenge(verifier, code.Challenge) {
		return tokenResponse{}, errInvalidAuth
	}
	if _, err := validAdmin(ctx, code.UserID); err != nil {
		return tokenResponse{}, err
	}
	grant := model.AutomationGrant{
		ID: uuid.NewString(), UserID: code.UserID, ClientID: clientID, Scope: code.Scope,
		Resource: resource, RefreshExpiresAt: now.Add(refreshLifetime),
	}
	grant, result := grantTokens(grant, now)
	return result, repository.ExchangeAutomationCode(ctx, code.Hash, grant, now)
}

func verifiesChallenge(verifier, expected string) bool {
	const minVerifier, maxVerifier = 43, 128
	if len(verifier) < minVerifier || len(verifier) > maxVerifier || strings.ContainsFunc(verifier, func(symbol rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._~", symbol)
	}) {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	encoded := base64.RawURLEncoding.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(encoded), []byte(expected)) == 1
}

func exchangeRefresh(request *http.Request, values url.Values, clientID, resource string, now time.Time) (tokenResponse, error) {
	ctx := request.Context()
	previous := digest(values.Get("refresh_token"))
	grant, err := repository.GetAutomationRefreshGrant(ctx, previous, clientID, resource, now)
	if err != nil {
		return tokenResponse{}, err
	}
	if scope := values.Get("scope"); scope != "" && scope != grant.Scope {
		return tokenResponse{}, errInvalidAuth
	}
	if _, err := validAdmin(ctx, grant.UserID); err != nil {
		return tokenResponse{}, err
	}
	grant, result := grantTokens(grant, now)
	return result, repository.RotateAutomationGrant(ctx, previous, grant, now)
}

func grantTokens(grant model.AutomationGrant, now time.Time) (model.AutomationGrant, tokenResponse) {
	access, refresh := randomCredential("ofa_"), randomCredential("ofr_")
	grant.AccessHash, grant.RefreshHash = digest(access), digest(refresh)
	grant.AccessExpiresAt = now.Add(accessLifetime)
	if grant.RefreshExpiresAt.Before(grant.AccessExpiresAt) {
		grant.AccessExpiresAt = grant.RefreshExpiresAt
	}
	return grant, tokenResponse{
		AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(grant.AccessExpiresAt.Sub(now) / time.Second),
		RefreshToken: refresh, Scope: grant.Scope,
	}
}

func revokeToken(c *gin.Context, _ instance) {
	values, ok := parseOAuthForm(c)
	if !ok {
		return
	}
	token, clientID := values.Get("token"), values.Get("client_id")
	if token == "" || clientID == "" {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	if err := repository.RevokeAutomationToken(c.Request.Context(), clientID, digest(token)); err != nil {
		oauthInternalError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

func oauthError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}

func oauthInternalError(c *gin.Context, err error) {
	logger.ErrorF(c.Request.Context(), "Automation OAuth request failed: %v", err)
	oauthError(c, http.StatusInternalServerError, "server_error")
}

// RegisterOAuth mounts authorization, registration, token exchange, and revocation endpoints.
func RegisterOAuth(group *gin.RouterGroup) {
	group.GET("/authorize", withInstance(authorize))
	group.POST("/authorize", withInstance(confirmConsent))
	group.POST("/register", withInstance(registerClient))
	group.POST("/token", withInstance(issueToken))
	group.POST("/revoke", withInstance(revokeToken))
}
