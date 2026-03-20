package service

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

func normalizeOriginAddress(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

func validateOriginAddress(address string) error {
	if address == "" {
		return errors.New("源站地址不能为空")
	}
	if strings.Contains(address, "://") || strings.ContainsAny(address, "/?#") {
		return errors.New("源站地址格式不合法")
	}
	if strings.HasPrefix(address, "[") || strings.HasSuffix(address, "]") {
		return errors.New("源站地址无需包含 IPv6 方括号")
	}
	if ip := net.ParseIP(address); ip != nil {
		return nil
	}
	if len(address) > 253 {
		return errors.New("源站地址格式不合法")
	}
	labels := strings.Split(address, ".")
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return errors.New("源站地址格式不合法")
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return errors.New("源站地址格式不合法")
		}
		for _, r := range label {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
				continue
			}
			return errors.New("源站地址格式不合法")
		}
	}
	return nil
}

func normalizeOriginName(name string, address string) string {
	normalized := strings.TrimSpace(name)
	if normalized != "" {
		return normalized
	}
	return address
}

func normalizeOriginPort(raw string) (string, error) {
	port := strings.TrimSpace(raw)
	if port == "" {
		return "", errors.New("端口不能为空")
	}
	value, err := strconv.Atoi(port)
	if err != nil || value < 1 || value > 65535 {
		return "", errors.New("端口格式不合法")
	}
	return strconv.Itoa(value), nil
}

func normalizeOriginScheme(raw string) (string, error) {
	scheme := strings.ToLower(strings.TrimSpace(raw))
	switch scheme {
	case "http", "https":
		return scheme, nil
	default:
		return "", errors.New("源站协议仅支持 http 或 https")
	}
}

func normalizeOriginURI(raw string) (string, error) {
	uri := strings.TrimSpace(raw)
	if uri == "" {
		return "", nil
	}
	if strings.Contains(uri, "://") {
		return "", errors.New("源站路径不能包含协议")
	}
	if !strings.HasPrefix(uri, "/") && !strings.HasPrefix(uri, "?") {
		return "", errors.New("源站路径需以 / 或 ? 开头")
	}
	return uri, nil
}

func formatOriginHost(address string, port string) string {
	if ip := net.ParseIP(address); ip != nil && strings.Contains(address, ":") {
		return net.JoinHostPort(address, port)
	}
	return net.JoinHostPort(address, port)
}

func buildOriginURLFromParts(
	scheme string,
	address string,
	port string,
	uri string,
) (string, error) {
	normalizedScheme, err := normalizeOriginScheme(scheme)
	if err != nil {
		return "", err
	}
	normalizedAddress := normalizeOriginAddress(address)
	if err := validateOriginAddress(normalizedAddress); err != nil {
		return "", err
	}
	normalizedPort, err := normalizeOriginPort(port)
	if err != nil {
		return "", err
	}
	normalizedURI, err := normalizeOriginURI(uri)
	if err != nil {
		return "", err
	}

	parsed := &url.URL{
		Scheme: normalizedScheme,
		Host:   formatOriginHost(normalizedAddress, normalizedPort),
	}
	if normalizedURI != "" {
		if strings.HasPrefix(normalizedURI, "?") {
			parsed.RawQuery = strings.TrimPrefix(normalizedURI, "?")
		} else {
			pathQuery := strings.SplitN(normalizedURI, "?", 2)
			parsed.Path = pathQuery[0]
			if len(pathQuery) > 1 {
				parsed.RawQuery = pathQuery[1]
			}
		}
	}
	return parsed.String(), nil
}

func extractOriginAddress(rawURL string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("源站地址格式不合法: %w", err)
	}
	address := normalizeOriginAddress(parsed.Hostname())
	if err := validateOriginAddress(address); err != nil {
		return "", err
	}
	return address, nil
}

func rewriteOriginURLAddress(rawURL string, newAddress string) (string, error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("源站地址格式不合法: %w", err)
	}
	address := normalizeOriginAddress(newAddress)
	if err := validateOriginAddress(address); err != nil {
		return "", err
	}
	port := parsed.Port()
	if port == "" {
		return "", errors.New("源站地址缺少端口")
	}
	parsed.Host = formatOriginHost(address, port)
	return parsed.String(), nil
}

func splitOriginURL(rawURL string) (scheme string, address string, port string, uri string, err error) {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return "", "", "", "", err
	}
	scheme = parsed.Scheme
	address = normalizeOriginAddress(parsed.Hostname())
	port = parsed.Port()
	uri = parsed.EscapedPath()
	if uri == "" {
		uri = parsed.Path
	}
	if parsed.RawQuery != "" {
		if uri == "" {
			uri = "?" + parsed.RawQuery
		} else {
			uri = uri + "?" + parsed.RawQuery
		}
	}
	return scheme, address, port, uri, nil
}
