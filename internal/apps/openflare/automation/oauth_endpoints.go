package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Rain-kl/Wavelet/internal/infra/config"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
)

type oauthHandler func(*gin.Context, instance)

var redirectHost = regexp.MustCompile(`^[A-Za-z0-9.-]+(:[0-9]{1,5})?$`)

const (
	oauthCode        = "code"
	publicClientAuth = "none"
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
	if err != nil || parsed.Scheme != schemeHTTP || parsed.User != nil || parsed.Opaque != "" ||
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

type consentIntent struct {
	Query   string `json:"query"`
	UserID  uint64 `json:"user_id"`
	Nonce   string `json:"nonce"`
	Expires int64  `json:"exp"`
}

const (
	consentPagePath  = "/automation/authorize"
	grantDeadlineKey = "grant_deadline"
)

func (site instance) oauthServer(redirect string) (fosite.OAuth2Provider, *oauthStorage) {
	cfg := &fosite.Config{
		GlobalSecret:          []byte(site.sign("automation-oauth", site.origin)),
		AuthorizeCodeLifespan: codeLifetime, AccessTokenLifespan: accessLifetime,
		RefreshTokenLifespan: refreshLifetime, RefreshTokenScopes: []string{},
		EnforcePKCE: true, TokenURL: site.origin + tokenPath,
	}
	storage := &oauthStorage{site: site, cliRedirect: redirect}
	return compose.Compose(cfg, storage, compose.NewOAuth2HMACStrategy(cfg),
		compose.OAuth2AuthorizeExplicitFactory, compose.OAuth2PKCEFactory,
		compose.OAuth2RefreshTokenGrantFactory, compose.OAuth2TokenIntrospectionFactory,
		compose.OAuth2TokenRevocationFactory), storage
}

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

func authorizationRequest(c *gin.Context, site instance, rawQuery string) (fosite.OAuth2Provider, fosite.AuthorizeRequester, error) {
	const maxAuthorizationQuery = 3600
	values, err := url.ParseQuery(rawQuery)
	if err != nil || len(rawQuery) > maxAuthorizationQuery || duplicateValues(values) ||
		!matchesResource(values, site.clientResource(values.Get("client_id"))) ||
		values.Get("code_challenge_method") != "S256" || values.Get("code_challenge") == "" ||
		values.Get("redirect_uri") == "" || (values.Get("response_mode") != "" && values.Get("response_mode") != "query") {
		return nil, nil, fosite.ErrInvalidRequest
	}
	provider, _ := site.oauthServer(values.Get("redirect_uri"))
	resource := site.clientResource(values.Get("client_id"))
	values.Set("audience", resource)
	if values.Get("scope") == "" {
		values.Set("scope", strings.Join(availableScopes(), " "))
	}
	request := c.Request.Clone(c.Request.Context())
	request.Method = http.MethodGet
	request.URL.RawQuery = values.Encode()
	request.Form, request.PostForm = nil, nil
	request.Body = http.NoBody
	authorization, err := provider.NewAuthorizeRequest(request.Context(), request)
	return provider, authorization, err
}

func authorize(c *gin.Context, site instance) {
	if _, _, err := authorizationRequest(c, site, c.Request.URL.RawQuery); err != nil {
		redirectConsentError(c, "invalid_request")
		return
	}
	user, err := browserAdmin(c)
	if err != nil {
		handleConsentLogin(c, err)
		return
	}
	cookie, err := c.Request.Cookie(config.Config.App.SessionCookieName)
	if err != nil {
		redirectConsentError(c, "login_required")
		return
	}
	intent, err := site.pack(site.consentPurpose(cookie.Value), consentIntent{
		Query: c.Request.URL.RawQuery, UserID: user.ID,
		Nonce: randomCredential(""), Expires: time.Now().Add(codeLifetime).Unix(),
	})
	if err != nil {
		oauthInternalError(c, err)
		return
	}
	c.Redirect(http.StatusFound, consentPagePath+"#"+url.Values{"intent": {intent}}.Encode())
}

func redirectConsentError(c *gin.Context, code string) {
	c.Redirect(http.StatusSeeOther, consentPagePath+"?"+url.Values{"error": {code}}.Encode())
}

func handleConsentLogin(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errLoginNeeded):
		target := c.Request.URL.RequestURI()
		c.Redirect(http.StatusFound, "/login?"+url.Values{"callbackUrl": {target}}.Encode())
	case isInvalidCredential(err):
		redirectConsentError(c, "access_denied")
	default:
		oauthInternalError(c, err)
	}
}

func (site instance) consentPurpose(cookie string) string {
	return "automation-consent:" + site.origin + ":" + cookie
}

func loadConsent(c *gin.Context, site instance, signed string, userID uint64) (consentIntent, bool) {
	var intent consentIntent
	cookie, err := c.Request.Cookie(config.Config.App.SessionCookieName)
	valid := err == nil && site.unpack(site.consentPurpose(cookie.Value), signed, &intent) &&
		intent.UserID == userID && intent.Expires > time.Now().UTC().Unix()
	return intent, valid
}

type consentInfoResponse struct {
	ClientName  string   `json:"client_name"`
	RedirectURI string   `json:"redirect_uri"`
	Scopes      []string `json:"scopes"`
}

func consentInfo(c *gin.Context, site instance) {
	user, err := browserAdmin(c)
	if err != nil {
		browserAPIAuthError(c, err)
		return
	}
	values, ok := parseOAuthForm(c)
	if !ok {
		return
	}
	intent, ok := loadConsent(c, site, values.Get("intent"), user.ID)
	if !ok {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	_, request, err := authorizationRequest(c, site, intent.Query)
	if err != nil {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	client, ok := site.client(request.GetClient().GetID(), request.GetRedirectURI().String())
	if !ok {
		oauthError(c, http.StatusBadRequest, "invalid_request")
		return
	}
	c.JSON(http.StatusOK, consentInfoResponse{
		ClientName: client.Name, RedirectURI: request.GetRedirectURI().String(), Scopes: request.GetRequestedScopes(),
	})
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
	intent, ok := loadConsent(c, site, values.Get("intent"), user.ID)
	if !ok {
		redirectConsentError(c, "invalid_request")
		return
	}
	provider, request, err := authorizationRequest(c, site, intent.Query)
	if err != nil {
		redirectConsentError(c, "invalid_request")
		return
	}
	ctx := c.Request.Context()
	if values.Get("decision") == "cancel" {
		provider.WriteAuthorizeError(ctx, c.Writer, request, fosite.ErrAccessDenied)
		return
	}
	scopes, ok := selectedScopes(values.Get("scope"), request.GetRequestedScopes())
	if values.Get("decision") != "authorize" || !ok {
		redirectConsentError(c, "invalid_scope")
		return
	}
	for _, scope := range scopes {
		request.GrantScope(scope)
	}
	request.GrantAudience(site.clientResource(request.GetClient().GetID()))
	request.SetID(intent.Nonce)
	issueAuthorizationCode(c, provider, request, user.ID)
}

func selectedScopes(raw string, allowed []string) ([]string, bool) {
	scopes := strings.Fields(raw)
	for _, scope := range scopes {
		if !slices.Contains(allowed, scope) {
			return nil, false
		}
	}
	slices.Sort(scopes)
	return slices.Compact(scopes), len(scopes) != 0
}

func rollbackOAuth(ctx context.Context) {
	if err := repository.RollbackAutomationTransaction(ctx); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logger.ErrorF(ctx, "Rollback automation transaction: %v", err)
	}
}

func issueAuthorizationCode(c *gin.Context, provider fosite.OAuth2Provider, request fosite.AuthorizeRequester, userID uint64) {
	ctx, err := repository.BeginAutomationTransaction(c.Request.Context())
	if err != nil {
		oauthInternalError(c, err)
		return
	}
	defer rollbackOAuth(ctx)
	if err := repository.PruneAutomationSessions(ctx, time.Now().UTC()); err != nil {
		oauthInternalError(c, err)
		return
	}
	session := &fosite.DefaultSession{
		Subject: strconv.FormatUint(userID, 10),
		Extra:   map[string]any{grantDeadlineKey: time.Now().UTC().Add(refreshLifetime).Format(time.RFC3339Nano)},
	}
	reply, err := provider.NewAuthorizeResponse(ctx, request, session)
	if err != nil {
		provider.WriteAuthorizeError(ctx, c.Writer, request, err)
		return
	}
	if err := repository.CommitAutomationTransaction(ctx); err != nil {
		oauthInternalError(c, err)
		return
	}
	provider.WriteAuthorizeResponse(ctx, c.Writer, request, reply)
}

func issueToken(c *gin.Context, site instance) {
	values, ok := parseOAuthForm(c)
	if !ok {
		return
	}
	if c.GetHeader("Authorization") != "" || values.Has("client_secret") || values.Has("client_assertion") {
		oauthError(c, http.StatusBadRequest, "invalid_client")
		return
	}
	if !matchesResource(values, site.clientResource(values.Get("client_id"))) {
		oauthError(c, http.StatusBadRequest, "invalid_target")
		return
	}
	provider, storage := site.oauthServer(values.Get("redirect_uri"))
	ctx, err := repository.BeginAutomationTransaction(c.Request.Context())
	if err != nil {
		oauthInternalError(c, err)
		return
	}
	defer rollbackOAuth(ctx)
	request, err := provider.NewAccessRequest(ctx, c.Request, &fosite.DefaultSession{})
	if err != nil {
		// Fosite can revoke a token family when a used code or refresh token is presented.
		if storage.revokeAccess && storage.revokeRefresh {
			if commitErr := repository.CommitAutomationTransaction(ctx); commitErr != nil {
				oauthInternalError(c, commitErr)
				return
			}
		}
		provider.WriteAccessError(ctx, c.Writer, request, err)
		return
	}
	if _, err := oauthSubject(ctx, request.GetSession()); err != nil {
		provider.WriteAccessError(ctx, c.Writer, request, fosite.ErrInvalidGrant)
		return
	}
	if !boundTokenLifetimes(request.GetSession()) {
		provider.WriteAccessError(ctx, c.Writer, request, fosite.ErrInvalidGrant)
		return
	}
	reply, err := provider.NewAccessResponse(ctx, request)
	if err != nil {
		provider.WriteAccessError(ctx, c.Writer, request, err)
		return
	}
	if err := repository.CommitAutomationTransaction(ctx); err != nil {
		oauthInternalError(c, err)
		return
	}
	provider.WriteAccessResponse(ctx, c.Writer, request, reply)
}

func boundTokenLifetimes(session fosite.Session) bool {
	oauthSession, ok := session.(*fosite.DefaultSession)
	if !ok {
		return false
	}
	rawDeadline, _ := oauthSession.Extra[grantDeadlineKey].(string)
	deadline, err := time.Parse(time.RFC3339Nano, rawDeadline)
	if err != nil || !deadline.After(time.Now().UTC()) {
		return false
	}
	for _, use := range []fosite.TokenType{fosite.AccessToken, fosite.RefreshToken} {
		if session.GetExpiresAt(use).After(deadline) {
			session.SetExpiresAt(use, deadline)
		}
	}
	return true
}

func oauthSubject(ctx context.Context, session fosite.Session) (uint64, error) {
	id, err := strconv.ParseUint(session.GetSubject(), 10, 64)
	if err != nil {
		return 0, errInvalidAuth
	}
	_, err = validAdmin(ctx, id)
	return id, err
}

func revokeToken(c *gin.Context, site instance) {
	if _, ok := parseOAuthForm(c); !ok {
		return
	}
	provider, _ := site.oauthServer("")
	ctx, err := repository.BeginAutomationTransaction(c.Request.Context())
	if err != nil {
		oauthInternalError(c, err)
		return
	}
	defer rollbackOAuth(ctx)
	err = provider.NewRevocationRequest(ctx, c.Request)
	if err == nil {
		if commitErr := repository.CommitAutomationTransaction(ctx); commitErr != nil {
			oauthInternalError(c, commitErr)
			return
		}
	}
	provider.WriteRevocationResponse(ctx, c.Writer, err)
}

func oauthError(c *gin.Context, status int, code string) {
	c.AbortWithStatusJSON(status, gin.H{"error": code})
}

func oauthInternalError(c *gin.Context, err error) {
	logger.ErrorF(c.Request.Context(), "Automation OAuth request failed: %v", err)
	oauthError(c, http.StatusInternalServerError, "server_error")
}

// RegisterOAuth mounts OAuth discovery companions and the browser consent exchange.
func RegisterOAuth(group *gin.RouterGroup) {
	group.GET("/authorize", withInstance(authorize))
	group.POST("/authorize", withInstance(confirmConsent))
	group.POST("/consent", withInstance(consentInfo))
	group.POST("/register", withInstance(registerClient))
	group.POST("/token", withInstance(issueToken))
	group.POST("/revoke", withInstance(revokeToken))
}
