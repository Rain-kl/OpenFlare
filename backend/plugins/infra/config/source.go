// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package config adapts viper to the kernel configuration source contract. It is a
// runtime adapter rather than a core.Plugin: it owns no routes, services or tasks and
// therefore never appears in app.Use. Keeping viper here preserves the micro-kernel
// rule against importing concrete runtime dependencies.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/viper"
)

// DefaultBaseFileName is the default base configuration file path looked up relative to the workspace.
const DefaultBaseFileName = "manifest/config/config.default.yaml"

// DefaultOverrideFileName is the default user override configuration file path looked up relative to the workspace.
const DefaultOverrideFileName = "manifest/config/config.yaml"

// DefaultFileName is kept for backwards compatibility.
const DefaultFileName = DefaultOverrideFileName

// EnvOnlyOrigin is reported by Describe when no configuration file was loaded.
const EnvOnlyOrigin = "<env only>"

// maxSearchDepth bounds the upward directory walk so a misconfigured working directory
// cannot make the loader scan the whole filesystem.
const maxSearchDepth = 5

// Option configures a Source.
type Option func(*Source)

// WithPath pins the configuration file, bypassing CONFIG_PATH and the upward search.
func WithPath(path string) Option {
	return func(s *Source) {
		s.path = path
		s.pinned = true
	}
}

// WithDefaultPath pins the default base configuration file path.
func WithDefaultPath(path string) Option {
	return func(s *Source) {
		s.defaultPath = path
	}
}

// Source implements core.ConfigSource over a configuration file plus the process environment.
type Source struct {
	v             *viper.Viper
	path          string
	defaultPath   string
	pinned        bool
	defaultFound  bool
	overrideFound bool
	found         bool
}

// NewSource loads configuration files. By default, it loads manifest/config/config.default.yaml
func (s *Source) resolvePaths() {
	if s.pinned {
		return
	}
	if s.defaultPath == "" {
		s.defaultPath, _ = findExistingUpward(DefaultBaseFileName)
	}
	if s.path == "" {
		s.path = os.Getenv("CONFIG_PATH")
	}
	if s.path == "" {
		s.path, _ = findExistingUpward(DefaultOverrideFileName)
	}
}

func readConfigFile(v *viper.Viper, path string, merge bool) (bool, error) {
	if path == "" {
		return false, nil
	}

	v.SetConfigFile(path)
	var err error
	if merge {
		err = v.MergeInConfig()
	} else {
		err = v.ReadInConfig()
	}

	switch {
	case err == nil:
		return true, nil
	case isNotFound(err):
		return false, nil
	default:
		if _, statErr := os.Stat(path); statErr == nil { //nolint:gosec // path is vetted by bounded upward search or config options
			return false, fmt.Errorf("infra/config: read %s: %w", path, err)
		}
		return false, nil
	}
}

// NewSource loads configuration files. By default, it loads manifest/config/config.default.yaml
// and merges manifest/config/config.yaml (or CONFIG_PATH) on top if present. A missing file is
// not an error: the source then serves environment values only.
func NewSource(opts ...Option) (*Source, error) {
	s := &Source{}
	for _, opt := range opts {
		opt(s)
	}
	s.resolvePaths()

	v := viper.New()

	var err error
	s.defaultFound, err = readConfigFile(v, s.defaultPath, false)
	if err != nil {
		return nil, err
	}

	s.overrideFound, err = readConfigFile(v, s.path, s.defaultFound)
	if err != nil {
		return nil, err
	}

	s.found = s.defaultFound || s.overrideFound
	s.v = v
	return s, nil
}

// isNotFound reports whether the loader failed only because the file is absent.
func isNotFound(err error) bool {
	var notFound viper.ConfigFileNotFoundError
	return errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist)
}

// Lookup returns the raw value stored at a dotted path, or false when the file was not
// loaded or the path is absent. Declared defaults therefore stay distinguishable from
// values explicitly set to a zero.
func (s *Source) Lookup(path string) (any, bool) {
	if !s.found || !s.v.IsSet(path) {
		return nil, false
	}
	return s.v.Get(path), true
}

// LookupEnv reads a process environment variable.
func (s *Source) LookupEnv(name string) (string, bool) {
	return os.LookupEnv(name)
}

// Describe returns the loaded file path, or EnvOnlyOrigin when running on environment values.
func (s *Source) Describe() string {
	if !s.found {
		return EnvOnlyOrigin
	}
	if s.overrideFound {
		return s.path
	}
	return s.defaultPath
}

// findExistingUpward searches upward from the working directory for a relative file path.
func findExistingUpward(relativeFilePath string) (string, bool) {
	if _, err := os.Stat(relativeFilePath); err == nil {
		return relativeFilePath, true
	}

	dir := "."
	for range maxSearchDepth {
		dir += "/.."
		path := dir + "/" + relativeFilePath
		if _, err := os.Stat(path); err == nil {
			return path, true
		}
	}
	return "", false
}
