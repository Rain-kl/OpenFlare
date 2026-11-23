// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

// Package performance defines the Pingora tuning policy contract.
package performance

import (
	"encoding/json"
	"fmt"
	"maps"
)

// Settings holds resolved Pingora tuning values for a node.
type Settings map[string]any

// NodePolicy is sent only to Pingora nodes that support performance policies.
// Null settings restore the runtime defaults.
type NodePolicy struct {
	Settings Settings `json:"settings"`
}

// Parameter describes one editable field and its supported values.
type Parameter struct {
	Key     string   `json:"key"`
	Group   string   `json:"group"`
	Default any      `json:"default"`
	Min     *int     `json:"min,omitempty"`
	Max     *int     `json:"max,omitempty"`
	Choices []string `json:"choices,omitempty"`
}

func integer(group, key string, value, minValue, maxValue int) Parameter {
	return Parameter{Key: key, Group: group, Default: value, Min: &minValue, Max: &maxValue}
}

func toggle(group, key string, value bool) Parameter {
	return Parameter{Key: key, Group: group, Default: value}
}

// Parameters defines the supported values and bounds shown in the policy editor.
//
//nolint:mnd // These numbers are the parameter defaults and bounds.
var Parameters = []Parameter{
	integer("runtime", "threads", 1, 1, 256),
	// The existing wire key now enables automatic sizing for both workers and the upstream pool.
	toggle("runtime", "threads_auto", false),
	toggle("runtime", "work_stealing", true),
	integer("runtime", "listener_tasks_per_fd", 1, 1, 32),
	integer("runtime", "upstream_keepalive_pool_size", 128, 1, 65536),
	integer("runtime", "max_retries", 3, 1, 16),
	toggle("runtime", "runtime_enable_alt_timer", false),
	integer("runtime", "fast_timeout_to_tokio_threshold_seconds", 900, 1, 86400),
	toggle("runtime", "runtime_metrics_poll_time_histogram", false),
	integer("upstream", "connection_timeout", 3, 0, 86400),
	integer("upstream", "total_connection_timeout", 8, 0, 86400),
	integer("upstream", "read_timeout", 30, 0, 86400),
	integer("upstream", "write_timeout", 30, 0, 86400),
	integer("upstream", "idle_timeout", 60, 0, 86400),
	toggle("upstream", "upstream_tcp_keepalive", false),
	integer("upstream", "upstream_tcp_recv_buf", 0, 0, 16777216),
	toggle("upstream", "verify_cert", true),
	toggle("upstream", "verify_hostname", true),
	{Key: "upstream_alpn", Group: "http2", Default: "h1", Choices: []string{"h1", "h2h1", "h2"}},
	integer("http2", "max_h2_streams", 16, 1, 1024),
	integer("http2", "h2_ping_interval", 0, 0, 86400),
	integer("http2", "h2_stream_window_size", 8388608, 1, 2147483647),
	integer("http2", "h2_connection_window_size", 8388608, 1, 2147483647),
	integer("http2", "max_concurrent_streams", 100, 1, 4096),
	integer("http2", "initial_window_size", 1048576, 1, 2147483647),
	integer("http2", "initial_connection_window_size", 1048576, 65535, 2147483647),
	integer("http2", "max_frame_size", 16384, 16384, 16777215),
	integer("http2", "max_header_list_size", 32768, 1024, 1048576),
	integer("http2", "header_table_size", 4096, 0, 65536),
	integer("http2", "max_send_buffer_size", 262144, 16384, 16777216),
	integer("tls", "upstream_connect_offload_threadpools", 0, 0, 16),
	integer("tls", "upstream_connect_offload_thread_per_pool", 1, 1, 16),
	integer("tls", "downstream_tls_offload_threadpools", 0, 0, 16),
	integer("tls", "downstream_tls_offload_thread_per_pool", 1, 1, 16),
	integer("listener", "keepalive_request_limit", 0, 0, 1000000),
	integer("listener", "h2_idle_timeout", 0, 0, 86400),
	toggle("listener", "h2c", false),
	toggle("listener", "so_reuseport", false),
	integer("listener", "tcp_fastopen", 0, 0, 65535),
	toggle("listener", "listener_tcp_keepalive", false),
	integer("listener", "tcp_snd_buf", 0, 0, 16777216),
	integer("listener", "tcp_recv_buf", 0, 0, 16777216),
	toggle("listener", "ipv6_only", false),
	integer("listener", "dscp", 0, 0, 63),
	toggle("cache", "cache_enabled", true),
	integer("cache", "cache_capacity_mib", 128, 16, 4096),
	integer("cache", "cache_disk_capacity_mib", 0, 0, 1048576),
	integer("cache", "cache_disk_inactive_seconds", 172800, 60, 2592000),
	integer("cache", "cache_disk_reading_max", 32, 1, 1024),
	integer("cache", "cache_disk_writing_max", 16, 1, 256),
	integer("cache", "cache_max_object_mib", 16, 1, 1024),
	integer("cache", "cache_lock_wait_timeout", 5, 1, 60),
	integer("cache", "cache_max_lock_retries", 1, 0, 16),
}

// Normalize fills omitted parameters and rejects values the runtime cannot use.
func Normalize(input map[string]json.RawMessage) (Settings, error) {
	settings := make(Settings, len(Parameters))
	unrecognized := maps.Clone(input)
	for _, parameter := range Parameters {
		settings[parameter.Key] = parameter.Default
		raw, ok := input[parameter.Key]
		if !ok {
			continue
		}
		delete(unrecognized, parameter.Key)
		value, err := parseParameter(parameter, raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", parameter.Key, err)
		}
		settings[parameter.Key] = value
	}
	for key := range unrecognized {
		return nil, fmt.Errorf("unknown OpenFlare Edge parameter %q", key)
	}
	if settings["runtime_enable_alt_timer"] == true && settings["work_stealing"] == false {
		return nil, fmt.Errorf("runtime_enable_alt_timer requires work_stealing")
	}
	if settings["verify_hostname"] == true && settings["verify_cert"] == false {
		return nil, fmt.Errorf("verify_hostname requires verify_cert")
	}
	if total, connect := settings["total_connection_timeout"].(int), settings["connection_timeout"].(int); total > 0 && total < connect {
		return nil, fmt.Errorf("total_connection_timeout must be at least connection_timeout")
	}
	capacityKey := "cache_capacity_mib"
	if settings["cache_disk_capacity_mib"].(int) > 0 {
		capacityKey = "cache_disk_capacity_mib"
	}
	if settings["cache_max_object_mib"].(int) > settings[capacityKey].(int) {
		return nil, fmt.Errorf("cache_max_object_mib must be at most %s", capacityKey)
	}
	return settings, nil
}

func parseParameter(parameter Parameter, raw json.RawMessage) (any, error) {
	switch parameter.Default.(type) {
	case int:
		var value *int
		if err := json.Unmarshal(raw, &value); err != nil || value == nil || *value < *parameter.Min || *value > *parameter.Max {
			return nil, fmt.Errorf("expected an integer between %d and %d", *parameter.Min, *parameter.Max)
		}
		return *value, nil
	case bool:
		var value *bool
		if err := json.Unmarshal(raw, &value); err != nil || value == nil {
			return nil, fmt.Errorf("expected a boolean")
		}
		return *value, nil
	default:
		var value string
		if err := json.Unmarshal(raw, &value); err == nil {
			for _, choice := range parameter.Choices {
				if choice == value {
					return value, nil
				}
			}
		}
		return nil, fmt.Errorf("expected one of %v", parameter.Choices)
	}
}

// Decode resolves the serialized settings of a saved policy.
func Decode(encoded string) (Settings, error) {
	var input map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &input); err != nil {
		return nil, err
	}
	return Normalize(input)
}
