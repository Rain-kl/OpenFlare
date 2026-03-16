package service

import (
	"encoding/json"
	"errors"
	"net/url"
	"openflare/model"
	"regexp"
	"strings"
)

var proxyHeaderKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

type ProxyRouteCustomHeaderInput struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ProxyRouteInput struct {
	Domain        string                        `json:"domain"`
	OriginURL     string                        `json:"origin_url"`
	OriginHost    string                        `json:"origin_host"`
	Enabled       bool                          `json:"enabled"`
	EnableHTTPS   bool                          `json:"enable_https"`
	CertID        *uint                         `json:"cert_id"`
	RedirectHTTP  bool                          `json:"redirect_http"`
	CustomHeaders []ProxyRouteCustomHeaderInput `json:"custom_headers"`
	Remark        string                        `json:"remark"`
}

func ListProxyRoutes() ([]*model.ProxyRoute, error) {
	return model.ListProxyRoutes()
}

func CreateProxyRoute(input ProxyRouteInput) (*model.ProxyRoute, error) {
	route, err := buildProxyRoute(nil, input)
	if err != nil {
		return nil, err
	}
	if err = route.Insert(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("域名已存在")
		}
		return nil, err
	}
	return route, nil
}

func UpdateProxyRoute(id uint, input ProxyRouteInput) (*model.ProxyRoute, error) {
	route, err := model.GetProxyRouteByID(id)
	if err != nil {
		return nil, err
	}
	route, err = buildProxyRoute(route, input)
	if err != nil {
		return nil, err
	}
	if err = route.Update(); err != nil {
		if isUniqueConstraintError(err) {
			return nil, errors.New("域名已存在")
		}
		return nil, err
	}
	return route, nil
}

func DeleteProxyRoute(id uint) error {
	route, err := model.GetProxyRouteByID(id)
	if err != nil {
		return err
	}
	return route.Delete()
}

func buildProxyRoute(route *model.ProxyRoute, input ProxyRouteInput) (*model.ProxyRoute, error) {
	domain := strings.ToLower(strings.TrimSpace(input.Domain))
	originURL := strings.TrimSpace(input.OriginURL)
	originHost := strings.TrimSpace(input.OriginHost)
	remark := strings.TrimSpace(input.Remark)
	customHeaders, err := normalizeCustomHeaders(input.CustomHeaders)
	if err != nil {
		return nil, err
	}
	customHeadersJSON, err := json.Marshal(customHeaders)
	if err != nil {
		return nil, err
	}
	if domain == "" {
		return nil, errors.New("域名不能为空")
	}
	if strings.Contains(domain, "://") || strings.Contains(domain, "/") {
		return nil, errors.New("域名格式不合法")
	}
	if err := validateOriginURL(originURL); err != nil {
		return nil, err
	}
	if err := validateOriginHost(originHost); err != nil {
		return nil, err
	}
	if !input.EnableHTTPS {
		input.RedirectHTTP = false
		input.CertID = nil
	}
	if input.EnableHTTPS {
		if input.CertID == nil || *input.CertID == 0 {
			return nil, errors.New("启用 HTTPS 时必须选择证书")
		}
		if _, err := model.GetTLSCertificateByID(*input.CertID); err != nil {
			return nil, errors.New("所选证书不存在")
		}
	}
	if input.RedirectHTTP && !input.EnableHTTPS {
		return nil, errors.New("仅启用 HTTPS 后才能开启 HTTP 重定向")
	}
	if route == nil {
		route = &model.ProxyRoute{}
	}
	route.Domain = domain
	route.OriginURL = originURL
	route.OriginHost = originHost
	route.Enabled = input.Enabled
	route.EnableHTTPS = input.EnableHTTPS
	route.CertID = input.CertID
	route.RedirectHTTP = input.RedirectHTTP
	route.CustomHeaders = string(customHeadersJSON)
	route.Remark = remark
	return route, nil
}

func normalizeCustomHeaders(headers []ProxyRouteCustomHeaderInput) ([]ProxyRouteCustomHeaderInput, error) {
	if len(headers) == 0 {
		return []ProxyRouteCustomHeaderInput{}, nil
	}
	normalized := make([]ProxyRouteCustomHeaderInput, 0, len(headers))
	for _, header := range headers {
		key := strings.TrimSpace(header.Key)
		value := strings.TrimSpace(header.Value)
		if key == "" && value == "" {
			continue
		}
		if key == "" {
			return nil, errors.New("自定义请求头名称不能为空")
		}
		if !proxyHeaderKeyPattern.MatchString(key) {
			return nil, errors.New("自定义请求头名称格式不合法")
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("自定义请求头不能包含换行")
		}
		normalized = append(normalized, ProxyRouteCustomHeaderInput{
			Key:   key,
			Value: value,
		})
	}
	return normalized, nil
}

func decodeStoredCustomHeaders(raw string) ([]ProxyRouteCustomHeaderInput, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return []ProxyRouteCustomHeaderInput{}, nil
	}
	var headers []ProxyRouteCustomHeaderInput
	if err := json.Unmarshal([]byte(text), &headers); err != nil {
		return nil, errors.New("自定义请求头配置格式不合法")
	}
	return normalizeCustomHeaders(headers)
}

func validateOriginURL(raw string) error {
	if raw == "" {
		return errors.New("源站地址不能为空")
	}
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New("源站地址格式不合法")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("源站地址必须以 http:// 或 https:// 开头")
	}
	if parsed.Host == "" {
		return errors.New("源站地址格式不合法")
	}
	return nil
}

func validateOriginHost(raw string) error {
	if raw == "" {
		return nil
	}
	if strings.ContainsAny(raw, "/\\ \t\r\n") || strings.Contains(raw, "://") {
		return errors.New("回源主机名格式不合法")
	}
	parsed, err := url.Parse("//" + raw)
	if err != nil || parsed.Host == "" || parsed.Host != raw {
		return errors.New("回源主机名格式不合法")
	}
	if parsed.Hostname() == "" {
		return errors.New("回源主机名格式不合法")
	}
	return nil
}

func isUniqueConstraintError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique")
}
