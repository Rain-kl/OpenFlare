// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package performance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPolicyDefaultsAndValidation(t *testing.T) {
	defaults, err := Normalize(nil)
	require.NoError(t, err)
	require.Len(t, defaults, len(Parameters))
	require.Equal(t, 1, defaults["threads"])
	require.Equal(t, 128, defaults["upstream_keepalive_pool_size"])
	require.Equal(t, false, defaults["verify_cert"])
	require.Equal(t, false, defaults["verify_hostname"])
	require.Equal(t, 0, defaults["cache_disk_capacity_mib"])
	require.Equal(t, 172800, defaults["cache_disk_inactive_seconds"])
	require.Equal(t, 32, defaults["cache_disk_reading_max"])
	require.Equal(t, 16, defaults["cache_disk_writing_max"])

	resolved, err := Decode(`{"threads_auto":true,"read_timeout":0,"work_stealing":false}`)
	require.NoError(t, err)
	require.Equal(t, true, resolved["threads_auto"])
	require.Equal(t, 0, resolved["read_timeout"])
	require.Equal(t, false, resolved["work_stealing"])

	disk, err := Decode(`{"cache_capacity_mib":16,"cache_disk_capacity_mib":256,"cache_max_object_mib":32}`)
	require.NoError(t, err)
	require.Equal(t, 256, disk["cache_disk_capacity_mib"])
	require.Equal(t, 32, disk["cache_max_object_mib"])

	strictTLS, err := Decode(`{"verify_cert":true,"verify_hostname":true}`)
	require.NoError(t, err)
	require.Equal(t, true, strictTLS["verify_cert"])
	require.Equal(t, true, strictTLS["verify_hostname"])

	certDisabled, err := Decode(`{"verify_cert":false}`)
	require.NoError(t, err)
	require.Equal(t, false, certDisabled["verify_cert"])
	require.Equal(t, false, certDisabled["verify_hostname"])

	for _, body := range []string{
		`{"threads":0}`,
		`{"threads":null}`,
		`{"threads_auto":"true"}`,
		`{"verify_hostname":true}`,
		`{"verify_cert":false,"verify_hostname":true}`,
		`{"max_frame_size":16777216}`,
		`{"upstream_alpn":"h3"}`,
		`{"cache_capacity_mib":16,"cache_max_object_mib":17}`,
		`{"cache_disk_capacity_mib":4,"cache_max_object_mib":5}`,
		`{"cache_disk_inactive_seconds":59}`,
		`{"cache_disk_reading_max":0}`,
		`{"cache_disk_writing_max":257}`,
		`{"unknown":1}`,
	} {
		t.Run(body, func(t *testing.T) {
			_, err := Decode(body)
			require.Error(t, err)
		})
	}
}
