// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package extpoints

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"time"
)

// Resolve computes the effective value of every declared key. Priority is, in order:
// an explicit environment override, an auto-enable trigger, the configuration file,
// then the declared default. Resolution is idempotent; later declarations resolve lazily.
func (r *ConfigRegistry) Resolve() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.src == nil {
		return ErrConfigNoSource
	}

	var errs []error
	for _, key := range r.order {
		if _, done := r.values[key]; done {
			continue
		}
		if err := r.resolveLocked(key); err != nil {
			errs = append(errs, err)
		}
	}
	r.resolved = true

	return errors.Join(errs...)
}

// Resolved reports whether Resolve has already run.
func (r *ConfigRegistry) Resolved() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resolved
}

// resolveLocked computes the effective value of one key. The caller must hold r.mu.
func (r *ConfigRegistry) resolveLocked(key string) error {
	d, ok := r.decls[key]
	if !ok {
		return fmt.Errorf("%w: %s", ErrConfigUnknownKey, key)
	}

	if d.env != "" {
		if raw, found := r.src.LookupEnv(d.env); found {
			value, err := convertValue(raw, d.typ)
			if err != nil {
				return fmt.Errorf("%w: key %q from environment %s: %w", ErrConfigType, key, d.env, err)
			}
			r.values[key], r.origins[key] = value, OriginEnv
			return nil
		}
	}

	if d.autoEnable != "" && d.typ.Kind() == reflect.Bool {
		if _, found := r.src.LookupEnv(d.autoEnable); found {
			r.values[key], r.origins[key] = true, OriginAutoEnable
			return nil
		}
	}

	if raw, found := r.src.Lookup(key); found {
		value, err := convertValue(raw, d.typ)
		if err != nil {
			return fmt.Errorf("%w: key %q from %s: %w", ErrConfigType, key, r.src.Describe(), err)
		}
		r.values[key], r.origins[key] = value, OriginFile
		return nil
	}

	if d.def != "" {
		value, err := convertValue(d.def, d.typ)
		if err != nil {
			return fmt.Errorf("%w: default %q for key %q: %w", ErrConfigType, d.def, key, err)
		}
		r.values[key], r.origins[key] = value, OriginDefault
		return nil
	}

	r.values[key] = reflect.New(d.typ).Elem().Interface()
	r.origins[key] = ""
	return nil
}

// Bind resolves the tagged fields of target and assigns them in place. Prefixes that
// were never declared self-register, so only gates need DeclareConfig.
func (r *ConfigRegistry) Bind(prefix string, target any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.src == nil {
		return ErrConfigNoSource
	}
	if !r.resolved {
		return fmt.Errorf("%w: Bind(%q, %T) ran before App.Prepare", ErrConfigNotResolved, prefix, target)
	}

	elem, err := bindingStruct(target, prefix)
	if err != nil {
		return err
	}
	fields, err := walkConfigFields(elem.Type(), prefix)
	if err != nil {
		return err
	}

	for _, f := range fields {
		if _, declared := r.decls[f.key]; !declared {
			if err := r.addDecl("bind:"+prefix, f); err != nil {
				return err
			}
		}
		if _, done := r.values[f.key]; !done {
			if err := r.resolveLocked(f.key); err != nil {
				return err
			}
		}
	}

	return assignFields(elem, fields, r.values)
}

// assignFields writes resolved values into a freshly walked target struct.
func assignFields(elem reflect.Value, fields []configField, values map[string]any) error {
	for _, f := range fields {
		field := elem.FieldByName(fieldNameForPath(elem.Type(), f.path))
		if !field.IsValid() || !field.CanSet() {
			return fmt.Errorf("%w: field for key %q is not settable", ErrConfigTarget, f.key)
		}

		value := reflect.ValueOf(values[f.key])
		if !value.Type().AssignableTo(field.Type()) {
			return fmt.Errorf("%w: key %q resolves to %s, field expects %s",
				ErrConfigType, f.key, value.Type(), field.Type())
		}
		field.Set(value)
	}
	return nil
}

// Entries returns the effective configuration as redacted, key-sorted entries.
func (r *ConfigRegistry) Entries() []ConfigEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	keys := append([]string(nil), r.order...)
	sort.Strings(keys)

	out := make([]ConfigEntry, 0, len(keys))
	for _, key := range keys {
		d := r.decls[key]
		if _, done := r.values[key]; !done && r.src != nil {
			_ = r.resolveLocked(key)
		}
		out = append(out, ConfigEntry{
			Key:      d.key,
			PluginID: d.pluginID,
			Env:      d.env,
			Origin:   r.origins[key],
			Value:    formatEntryValue(r.values[key], d.secret),
		})
	}
	return out
}

// formatEntryValue renders one effective value for diagnostics, masking secrets.
func formatEntryValue(value any, secret bool) string {
	if secret {
		return RedactedValue
	}
	if value == nil {
		return ""
	}
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return ""
		}
		return fmt.Sprint(rv.Elem().Interface())
	}
	return fmt.Sprint(value)
}

// Value returns the resolved value for key, lazily resolving a declared key that has
// not been computed yet. Missing and unresolvable keys report false rather than an
// error so gates and diagnostics can keep using the fallback accessors.
func (r *ConfigRegistry) Value(key string) (any, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, done := r.values[key]; !done {
		if r.src == nil {
			return nil, false
		}
		if _, declared := r.decls[key]; !declared {
			return nil, false
		}
		if err := r.resolveLocked(key); err != nil {
			return nil, false
		}
	}

	value, ok := r.values[key]
	return value, ok
}

// String returns the string value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) String(key, fallback string) string {
	if value, ok := r.Value(key); ok {
		if converted, err := convertString(value); err == nil {
			return converted.(string)
		}
	}
	return fallback
}

// Bool returns the boolean value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) Bool(key string, fallback bool) bool {
	if value, ok := r.Value(key); ok {
		if converted, err := convertBool(value); err == nil {
			return converted.(bool)
		}
	}
	return fallback
}

// Int returns the int value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) Int(key string, fallback int) int {
	if value, ok := r.Value(key); ok {
		converted, err := convertNumeric(value, reflect.TypeFor[int](), signedNumbers)
		if err == nil {
			return converted.(int)
		}
	}
	return fallback
}

// Duration returns the time.Duration value of key or fallback when absent or mismatched.
func (r *ConfigRegistry) Duration(key string, fallback time.Duration) time.Duration {
	if value, ok := r.Value(key); ok {
		if converted, err := convertDuration(value); err == nil {
			return converted.(time.Duration)
		}
	}
	return fallback
}

// Strings returns the []string value of key, or nil when absent.
func (r *ConfigRegistry) Strings(key string) []string {
	value, ok := r.Value(key)
	if !ok {
		return nil
	}

	converted, err := convertSlice(value, reflect.TypeFor[[]string]())
	if err != nil {
		return nil
	}

	list, _ := converted.([]string)
	return list
}

// WasSet reports whether an environment variable is present, regardless of its value.
func (r *ConfigRegistry) WasSet(envName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.src == nil {
		return false
	}
	_, found := r.src.LookupEnv(envName)
	return found
}

// Origin reports where a key's effective value came from; "" means the zero value.
func (r *ConfigRegistry) Origin(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.origins[key]
}
