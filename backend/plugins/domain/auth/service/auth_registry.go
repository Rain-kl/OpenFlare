// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package service implements domain business services and orchestration for the auth plugin.
package service

import (
	"Wavelet/core/contracts"
	"sync"
)

// AuthRegistryImpl implements contracts.AuthRegistry.
type AuthRegistryImpl struct {
	mu        sync.RWMutex
	providers map[string]contracts.OAuthProvider
}

// NewAuthRegistry creates a new AuthRegistryImpl.
func NewAuthRegistry() *AuthRegistryImpl {
	return &AuthRegistryImpl{
		providers: make(map[string]contracts.OAuthProvider),
	}
}

// RegisterOAuthProvider registers an OAuthProvider by name.
func (r *AuthRegistryImpl) RegisterOAuthProvider(name string, provider contracts.OAuthProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// GetOAuthProvider retrieves an OAuthProvider by name.
func (r *AuthRegistryImpl) GetOAuthProvider(name string) (contracts.OAuthProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// ListOAuthProviders lists all registered provider names.
func (r *AuthRegistryImpl) ListOAuthProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]string, 0, len(r.providers))
	for name := range r.providers {
		res = append(res, name)
	}
	return res
}
