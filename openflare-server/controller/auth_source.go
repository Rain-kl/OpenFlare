package controller

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/rain-kl/openflare/openflare-server/common"
	"github.com/rain-kl/openflare/openflare-server/common/response"
	"github.com/rain-kl/openflare/openflare-server/controller/bind"
	"github.com/rain-kl/openflare/openflare-server/model"
	"github.com/rain-kl/openflare/openflare-server/service"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

const pendingExternalAccountSessionKey = "pending_external_account"

type authSourceTogglePayload struct {
	IsActive bool `json:"is_active"`
}

type authSourcePayload struct {
	Name               string `json:"name"`
	Type               string `json:"type"`
	DisplayName        string `json:"display_name"`
	IsActive           bool   `json:"is_active"`
	ClientID           string `json:"client_id"`
	ClientSecret       string `json:"client_secret"`
	OpenIDDiscoveryURL string `json:"openid_discovery_url"`
	Scopes             string `json:"scopes"`
	IconURL            string `json:"icon_url"`
}

func (payload authSourcePayload) toModel() model.AuthSource {
	return model.AuthSource{
		Name:               payload.Name,
		Type:               payload.Type,
		DisplayName:        payload.DisplayName,
		IsActive:           payload.IsActive,
		ClientID:           payload.ClientID,
		ClientSecret:       payload.ClientSecret,
		OpenIDDiscoveryURL: payload.OpenIDDiscoveryURL,
		Scopes:             payload.Scopes,
		IconURL:            payload.IconURL,
	}
}

func ListAuthSources(c *gin.Context) {
	sources, err := model.GetAuthSources()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, sources)
}

func CreateAuthSource(c *gin.Context) {
	var payload authSourcePayload
	if err := bind.DecodeJSONBody(c.Request.Body, &payload); err != nil {
		response.RespondBadRequest(c, "无效的参数")
		return
	}
	source := payload.toModel()
	if err := model.CreateAuthSource(&source); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	source.Sanitize()
	response.RespondSuccess(c, source)
}

func UpdateAuthSource(c *gin.Context) {
	id, err := parseAuthSourceID(c)
	if err != nil {
		response.RespondBadRequest(c, err.Error())
		return
	}
	var payload authSourcePayload
	if err := bind.DecodeJSONBody(c.Request.Body, &payload); err != nil {
		response.RespondBadRequest(c, "无效的参数")
		return
	}
	source := payload.toModel()
	source.ID = id
	keepSecret := strings.TrimSpace(source.ClientSecret) == ""
	if err := model.UpdateAuthSource(&source, keepSecret); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	updated, err := model.GetAuthSourceByID(id)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	updated.Sanitize()
	response.RespondSuccess(c, updated)
}

func DeleteAuthSource(c *gin.Context) {
	id, err := parseAuthSourceID(c)
	if err != nil {
		response.RespondBadRequest(c, err.Error())
		return
	}
	if err := model.DeleteAuthSource(id); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

func ToggleAuthSource(c *gin.Context) {
	id, err := parseAuthSourceID(c)
	if err != nil {
		response.RespondBadRequest(c, err.Error())
		return
	}
	var payload authSourceTogglePayload
	if err := bind.DecodeJSONBody(c.Request.Body, &payload); err != nil {
		response.RespondBadRequest(c, "无效的参数")
		return
	}
	if err := model.ToggleAuthSource(id, payload.IsActive); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

func OAuthAuthorize(c *gin.Context) {
	source, err := getAuthSourceFromRoute(c)
	if err != nil {
		response.RespondBadRequest(c, err.Error())
		return
	}
	if !source.IsActive {
		response.RespondFailure(c, "认证源未启用")
		return
	}
	if err := source.Validate(); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	state, err := service.GenerateOAuthState()
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	session := sessions.Default(c)
	session.Set(oauthStateSessionKey(source.ID), state)
	if err := session.Save(); err != nil {
		response.RespondFailure(c, "无法保存授权状态，请重试")
		return
	}
	redirectURL := oauthFrontendCallbackURL(c, source.ID)
	authorizeURL, err := service.BuildAuthorizeURL(c.Request.Context(), source, redirectURL, state)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, gin.H{"authorize_url": authorizeURL})
}

func OAuthCallback(c *gin.Context) {
	source, err := getAuthSourceFromRoute(c)
	if err != nil {
		response.RespondBadRequest(c, err.Error())
		return
	}
	if !source.IsActive {
		response.RespondFailure(c, "认证源未启用")
		return
	}
	session := sessions.Default(c)
	expectedState, _ := session.Get(oauthStateSessionKey(source.ID)).(string)
	state := c.Query("state")
	if expectedState == "" || state == "" || state != expectedState {
		response.RespondFailure(c, "授权状态无效，请重新登录")
		return
	}
	session.Delete(oauthStateSessionKey(source.ID))
	if err := session.Save(); err != nil {
		response.RespondFailure(c, "无法更新授权状态，请重试")
		return
	}
	if oauthError := c.Query("error"); oauthError != "" {
		description := c.Query("error_description")
		if description == "" {
			description = oauthError
		}
		response.RespondFailure(c, description)
		return
	}

	profile, err := service.ExchangeOAuthProfile(c.Request.Context(), source, c.Query("code"), oauthFrontendCallbackURL(c, source.ID))
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	var currentUserID *int
	if currentUser := currentUserFromOpenFlareToken(c); currentUser != nil {
		currentUserID = &currentUser.Id
	}
	result, pending, err := service.CompleteOAuthLogin(source, profile, currentUserID)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	if pending != nil {
		raw, err := json.Marshal(pending)
		if err != nil {
			response.RespondFailure(c, err.Error())
			return
		}
		session.Set(pendingExternalAccountSessionKey, string(raw))
		if err := session.Save(); err != nil {
			response.RespondFailure(c, "无法保存待绑定账号，请重试")
			return
		}
		response.RespondSuccess(c, result)
		return
	}
	if result.User != nil {
		cleanUser, err := setLoginToken(result.User)
		if err != nil {
			response.RespondFailure(c, "无法保存会话信息，请重试")
			return
		}
		result.User = cleanUser
	}
	response.RespondSuccess(c, result)
}

func LinkExistingOAuthAccount(c *gin.Context) {
	session := sessions.Default(c)
	raw, _ := session.Get(pendingExternalAccountSessionKey).(string)
	if raw == "" {
		response.RespondFailure(c, "待绑定第三方账号已失效，请重新登录")
		return
	}
	var pending service.PendingExternalAccount
	if err := json.Unmarshal([]byte(raw), &pending); err != nil {
		response.RespondFailure(c, "待绑定第三方账号无效，请重新登录")
		return
	}
	var input service.LinkExistingRequest
	if err := bind.DecodeJSONBody(c.Request.Body, &input); err != nil {
		response.RespondBadRequest(c, "无效的参数")
		return
	}
	user, err := service.LinkPendingExternalAccount(&pending, input)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	session.Delete(pendingExternalAccountSessionKey)
	if err := session.Save(); err != nil {
		response.RespondFailure(c, "无法更新会话信息，请重试")
		return
	}
	cleanUser, err := setLoginToken(user)
	if err != nil {
		response.RespondFailure(c, "无法保存会话信息，请重试")
		return
	}
	response.RespondSuccess(c, service.OAuthCallbackResult{Status: "linked", User: cleanUser})
}

func ListExternalAccounts(c *gin.Context) {
	userID := c.GetInt("id")
	accounts, err := model.ListExternalAccountsByUserID(userID)
	if err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccess(c, accounts)
}

func DeleteExternalAccount(c *gin.Context) {
	rawID := strings.TrimSpace(c.Param("id"))
	parsedID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || parsedID == 0 {
		response.RespondBadRequest(c, "绑定记录 ID 无效")
		return
	}
	if err := model.DeleteExternalAccountForUser(uint(parsedID), c.GetInt("id")); err != nil {
		response.RespondFailure(c, err.Error())
		return
	}
	response.RespondSuccessMessage(c, "")
}

func parseAuthSourceID(c *gin.Context) (uint, error) {
	raw := c.Param("source_id")
	if raw == "" {
		raw = c.Param("id")
	}
	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || parsed == 0 {
		return 0, fmt.Errorf("认证源 ID 无效")
	}
	return uint(parsed), nil
}

func getAuthSourceFromRoute(c *gin.Context) (*model.AuthSource, error) {
	raw := strings.TrimSpace(c.Param("source"))
	if raw == "" {
		raw = strings.TrimSpace(c.Param("source_id"))
	}
	if raw == "" {
		raw = strings.TrimSpace(c.Param("id"))
	}
	if raw == "" {
		return nil, fmt.Errorf("认证源不能为空")
	}
	if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil && parsed > 0 {
		source, err := model.GetAuthSourceByID(uint(parsed))
		if err != nil {
			return nil, err
		}
		return source, nil
	}
	source, err := model.GetAuthSourceByName(raw)
	if err != nil {
		return nil, err
	}
	return source, nil
}

func oauthStateSessionKey(sourceID uint) string {
	return fmt.Sprintf("oauth_state_%d", sourceID)
}

func oauthFrontendCallbackURL(c *gin.Context, sourceID uint) string {
	base := strings.TrimRight(common.ServerAddress, "/")
	if base == "" {
		scheme := "http"
		if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
			scheme = "https"
		}
		host := c.Request.Host
		if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
			host = forwardedHost
		}
		base = scheme + "://" + host
	}
	source, err := model.GetAuthSourceByID(sourceID)
	sourceName := strconv.FormatUint(uint64(sourceID), 10)
	if err == nil && strings.TrimSpace(source.Name) != "" {
		sourceName = source.Name
	}
	callback, _ := url.JoinPath(base, "oauth", sourceName)
	return callback
}
