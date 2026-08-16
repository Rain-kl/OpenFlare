// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package geoip resolves geographic information for IP addresses.
package geoip

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"
	"unicode"

	ristretto "github.com/dgraph-io/ristretto/v2"
)

// CurrentProvider is the active GeoIP backend used by package-level helpers.
var CurrentProvider Service
var geoCache *providerCache
var providerMutex sync.RWMutex
var providerFactory = newProvider

// Supported GeoIP provider identifiers.
const (
	// ProviderDisabled disables GeoIP lookups.
	ProviderDisabled = "disabled"
	ProviderMaxMind  = "mmdb"
	ProviderIPAPI    = "ip-api"
	ProviderGeoJS    = "geojs"
	ProviderIPInfo   = "ipinfo"

	defaultGeoCacheDuration = 48 * time.Hour
	isoCountryCodeLength    = 2
	geoipDataDirPerm        = 0o750
)

// GeoInfo contains normalized geographic metadata for an IP address.
type GeoInfo struct {
	ISOCode   string
	Name      string
	Latitude  *float64
	Longitude *float64
}

func init() {
	CurrentProvider = &EmptyProvider{}
	geoCache = newProviderCache(defaultGeoCacheDuration)
}

// Service defines the core GeoIP lookup operations implemented by providers.
type Service interface {
	Name() string
	GetGeoInfo(ip net.IP) (*GeoInfo, error)
	UpdateDatabase() error
	Close() error
}

type cachedGeoInfo struct {
	info      *GeoInfo
	expiresAt time.Time
}

type providerCache struct {
	items    *ristretto.Cache[string, cachedGeoInfo]
	duration time.Duration
}

func newProviderCache(duration time.Duration) *providerCache {
	items, err := ristretto.NewCache(&ristretto.Config[string, cachedGeoInfo]{
		NumCounters: 1e5,
		MaxCost:     2e4,
		BufferItems: 64,
	})
	if err != nil {
		panic(err)
	}
	return &providerCache{
		items:    items,
		duration: duration,
	}
}

func (c *providerCache) Get(key string) (*GeoInfo, bool) {
	entry, ok := c.items.Get(key)
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		c.items.Del(key)
		return nil, false
	}
	return entry.info, true
}

func (c *providerCache) Set(key string, info *GeoInfo) {
	c.items.Set(key, cachedGeoInfo{
		info:      info,
		expiresAt: time.Now().Add(c.duration),
	}, 1)
	c.items.Wait()
}

func (c *providerCache) Flush() {
	c.items.Clear()
}

// GetRegionUnicodeEmoji returns the regional indicator emoji for a two-letter ISO code.
func GetRegionUnicodeEmoji(isoCode string) string {
	if len(isoCode) != isoCountryCodeLength {
		return ""
	}
	isoCode = strings.ToUpper(isoCode)

	if !unicode.IsLetter(rune(isoCode[0])) || !unicode.IsLetter(rune(isoCode[1])) {
		return ""
	}

	rune1 := 0x1F1E6 + (rune(isoCode[0]) - 'A')
	rune2 := 0x1F1E6 + (rune(isoCode[1]) - 'A')
	return string(rune1) + string(rune2)
}

// InitGeoIP configures the active GeoIP provider.
func InitGeoIP(provider string) {
	providerName := normalizeProvider(provider)
	nextProvider, err := providerFactory(providerName)
	if err != nil {
		slog.Error("initialize GeoIP provider failed", "provider", providerName, "error", err)
		nextProvider = &EmptyProvider{}
	}
	setProvider(nextProvider)
	if providerName == ProviderDisabled {
		slog.Info("GeoIP provider disabled")
		return
	}
	slog.Info("GeoIP provider configured", "provider", CurrentProvider.Name())
}

// GetGeoInfo looks up geographic information for ip using the active provider.
func GetGeoInfo(ip net.IP) (*GeoInfo, error) {
	if ip == nil {
		return nil, errors.New("IP address cannot be nil")
	}
	provider := getProvider()
	cacheKey := provider.Name() + ":" + ip.String()

	if cachedInfo, found := geoCache.Get(cacheKey); found {
		return cachedInfo, nil
	}

	info, err := provider.GetGeoInfo(ip)
	if err == nil && info != nil {
		geoCache.Set(cacheKey, info)
	}
	return info, err
}

// LookupGeoInfoWithProvider looks up geographic information using a temporary provider.
func LookupGeoInfoWithProvider(providerName string, ip net.IP) (*GeoInfo, error) {
	if ip == nil {
		return nil, errors.New("IP address cannot be nil")
	}

	provider, err := providerFactory(normalizeProvider(providerName))
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := provider.Close(); closeErr != nil {
			slog.Warn("close temporary GeoIP provider failed", "provider", provider.Name(), "error", closeErr)
		}
	}()

	return provider.GetGeoInfo(ip)
}

// UpdateDatabase refreshes the active provider database and clears cached lookups.
func UpdateDatabase() error {
	err := getProvider().UpdateDatabase()
	if err == nil {
		geoCache.Flush()
		slog.Info("GeoIP cache cleared due to database update.")
	}
	return err
}

// IsValidProvider reports whether provider names a supported GeoIP backend.
func IsValidProvider(provider string) bool {
	switch normalizeProvider(provider) {
	case ProviderDisabled, ProviderMaxMind, ProviderIPAPI, ProviderGeoJS, ProviderIPInfo:
		return true
	default:
		return false
	}
}

func normalizeProvider(provider string) string {
	normalized := strings.TrimSpace(strings.ToLower(provider))
	if normalized == "" {
		return ProviderDisabled
	}
	return normalized
}

func newProvider(provider string) (Service, error) {
	switch provider {
	case ProviderDisabled:
		return &EmptyProvider{}, nil
	case ProviderMaxMind:
		return NewMaxMindGeoIPService()
	case ProviderIPAPI:
		return NewIPAPIService()
	case ProviderGeoJS:
		return NewGeoJSService()
	case ProviderIPInfo:
		return NewIPInfoService()
	default:
		return nil, fmt.Errorf("unsupported GeoIP provider %q", provider)
	}
}

func setProvider(provider Service) {
	providerMutex.Lock()
	previous := CurrentProvider
	CurrentProvider = provider
	providerMutex.Unlock()
	geoCache.Flush()
	if previous != nil && previous != provider {
		if err := previous.Close(); err != nil {
			slog.Warn("close previous GeoIP provider failed", "error", err)
		}
	}
}

func getProvider() Service {
	providerMutex.RLock()
	defer providerMutex.RUnlock()
	if CurrentProvider == nil {
		return &EmptyProvider{}
	}
	return CurrentProvider
}

func float64Pointer(value float64) *float64 {
	return &value
}

// ProviderFactoryForTest returns the provider factory used by package helpers.
func ProviderFactoryForTest() func(string) (Service, error) {
	return providerFactory
}

// SetProviderFactoryForTest replaces the provider factory for tests.
func SetProviderFactoryForTest(factory func(string) (Service, error)) {
	if factory == nil {
		providerFactory = newProvider
		return
	}
	providerFactory = factory
}
