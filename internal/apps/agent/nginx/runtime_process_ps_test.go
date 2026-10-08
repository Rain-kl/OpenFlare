// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import "testing"

func TestParsePSRuntimeInstanceMembership(t *testing.T) {
	output := `42 1 1000 Wed Oct 7 16:00:00 2026 nginx: master process openresty -c /data/nginx.conf
43 42 1001 Wed Oct 7 16:00:01 2026 nginx: worker process
50 1 1000 Wed Oct 7 16:00:00 2026 nginx: master process openresty -c /other/nginx.conf
51 50 1001 Wed Oct 7 16:00:01 2026 nginx: worker process`
	processes, err := parsePSRuntime(output, "openresty", "/data/nginx.conf", 1000)
	if err != nil {
		t.Fatal(err)
	}
	processes = runtimeDescendants(processes, runtimeMasterIdentities(processes))
	if len(processes) != 2 || processes[0].PID != 42 || processes[1].PID != 43 || processes[1].ParentPID != 42 {
		t.Fatalf("instance processes = %v, want master 42 and worker 43", processes)
	}
	if processes[0].StartTime != "Wed Oct 7 16:00:00 2026" {
		t.Errorf("master start time = %q", processes[0].StartTime)
	}
}
