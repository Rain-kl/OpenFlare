package service

import (
	"errors"
	"net"
	"strings"

	"github.com/rain-kl/openflare/openflare-server/utils/geoip"
)

type GeoIPLookupView struct {
	Provider  string   `json:"provider"`
	IP        string   `json:"ip"`
	ISOCode   string   `json:"iso_code"`
	Name      string   `json:"name"`
	Latitude  *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
}

func LookupGeoIP(provider string, rawIP string) (*GeoIPLookupView, error) {
	trimmedProvider := strings.TrimSpace(provider)
	if !geoip.IsValidProvider(trimmedProvider) {
		return nil, errors.New("归属方式仅支持 disabled、mmdb、ip-api、geojs、ipinfo")
	}

	trimmedIP := strings.TrimSpace(rawIP)
	if trimmedIP == "" {
		return nil, errors.New("IP 不能为空")
	}
	parsedIP := net.ParseIP(trimmedIP)
	if parsedIP == nil {
		return nil, errors.New("IP 格式无效")
	}

	info, err := geoip.LookupGeoInfoWithProvider(trimmedProvider, parsedIP)
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, errors.New("未获取到 IP 归属结果")
	}

	return &GeoIPLookupView{
		Provider:  trimmedProvider,
		IP:        parsedIP.String(),
		ISOCode:   info.ISOCode,
		Name:      info.Name,
		Latitude:  info.Latitude,
		Longitude: info.Longitude,
	}, nil
}
