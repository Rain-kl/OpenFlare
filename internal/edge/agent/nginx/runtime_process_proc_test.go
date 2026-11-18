// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package nginx

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestInspectProcRuntime(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "openresty")
	if err := os.WriteFile(binary, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "nginx.conf")
	writeProcFixture(t, root, 42, 1, 1000, "nginx: master process "+binary+" -c "+config, "S", binary)
	// Root masters may run workers as a different user. Track them during shutdown.
	writeProcFixture(t, root, 43, 42, 1001, "nginx: worker process", "S", binary)
	writeProcFixture(t, root, 44, 42, 1000, "nginx: worker process", "Z", binary)
	writeProcFixture(t, root, 45, 1, 1000, "unrelated command", "S", binary)
	// Another instance using the same executable must not block this instance.
	writeProcFixture(t, root, 48, 1, 1000, "nginx: master process "+binary+" -c /other/nginx.conf", "S", binary)
	writeProcFixture(t, root, 49, 48, 1000, "nginx: worker process", "S", binary)
	otherBinary := filepath.Join(root, "other")
	if err := os.WriteFile(otherBinary, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	writeProcFixture(t, root, 46, 1, 1000, "nginx: worker process", "S", otherBinary)
	processes, err := inspectProcRuntime(root, executable, binary, config, 1000)
	if err != nil {
		t.Fatal(err)
	}
	processes = runtimeDescendants(processes, runtimeMasterIdentities(processes))
	if len(processes) != 2 {
		t.Fatalf("inspected processes = %v, want live master and worker", processes)
	}
	if processes[0] != (runtimeProcess{PID: 42, ParentPID: 1, Master: true, StartTime: "142"}) {
		t.Errorf("master identity = %v, want PID 42 and start time 142", processes[0])
	}
	if processes[1].PID != 43 || processes[1].Master {
		t.Errorf("worker identity = %v, want worker 43", processes[1])
	}
	if err := os.RemoveAll(filepath.Join(root, "42")); err != nil {
		t.Fatal(err)
	}
	remaining, err := inspectProcRuntime(root, executable, binary, config, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if master, err := findRuntimeMaster(remaining); err != nil || master.PID != 0 {
		t.Errorf("unattributed worker blocks cold start: master=%v err=%v", master, err)
	}
	writeProcFixture(t, root, 47, 1, 1000, "nginx: master process "+binary+" -c "+config, "S", otherBinary)
	if _, err := inspectProcRuntime(root, executable, binary, config, 1000); err == nil {
		t.Error("inspectProcRuntime(replaced executable) = nil, want refusal")
	}
}

func TestInspectProcRuntimeIgnoresDifferentConfigOrOwner(t *testing.T) {
	for _, tc := range []struct {
		name   string
		uid    int
		config string
	}{
		{name: "different config", uid: 1000, config: "/other/nginx.conf"},
		{name: "different owner", uid: 1001, config: "/data/nginx.conf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			binary := filepath.Join(root, "openresty")
			if err := os.WriteFile(binary, nil, 0o755); err != nil {
				t.Fatal(err)
			}
			executable, err := os.Stat(binary)
			if err != nil {
				t.Fatal(err)
			}
			writeProcFixture(t, root, 42, 1, tc.uid, "nginx: master process "+binary+" -c "+tc.config, "S", binary)
			processes, err := inspectProcRuntime(root, executable, binary, "/data/nginx.conf", 1000)
			if err != nil {
				t.Fatal(err)
			}
			if master, err := findRuntimeMaster(processes); err != nil || master.PID != 0 {
				t.Errorf("unrelated master blocks cold start: master=%v err=%v", master, err)
			}
		})
	}
}

func writeProcFixture(t *testing.T, root string, pid, parentPID, uid int, title, state, binary string) {
	t.Helper()
	dir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fields := make([]string, 20)
	for i := range fields {
		fields[i] = "0"
	}
	fields[0] = state
	fields[procParentPIDIndex] = strconv.Itoa(parentPID)
	fields[19] = strconv.Itoa(pid + 100)
	for name, content := range map[string]string{
		"status":  fmt.Sprintf("Uid:\t%d\t%d\t%d\t%d\n", uid, uid, uid, uid),
		"cmdline": title + "\x00",
		"stat":    fmt.Sprintf("%d (nginx worker) %s\n", pid, strings.Join(fields, " ")),
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(binary, filepath.Join(dir, "exe")); err != nil {
		t.Fatal(err)
	}
}

func TestInspectProcRuntimeScopesUnreadableExecutables(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read the permission-denied fixture")
	}
	root := t.TempDir()
	binary := filepath.Join(root, "openresty")
	if err := os.WriteFile(binary, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Stat(binary)
	if err != nil {
		t.Fatal(err)
	}
	denied := filepath.Join(root, "denied")
	if err := os.Mkdir(denied, 0o700); err != nil {
		t.Fatal(err)
	}
	inaccessible := filepath.Join(denied, "exe")
	if err := os.Symlink(binary, inaccessible); err != nil {
		t.Fatal(err)
	}
	config := "/data/nginx.conf"
	writeProcFixture(t, root, 42, 1, 1000, "nginx: master process "+binary+" -c "+config, "S", inaccessible)
	writeProcFixture(t, root, 43, 42, 1001, "nginx: worker process", "S", inaccessible)
	writeProcFixture(t, root, 50, 1, 1000, "nginx: master process "+binary+" -c /other/nginx.conf", "S", inaccessible)
	writeProcFixture(t, root, 51, 50, 1001, "nginx: worker process", "S", inaccessible)
	if err := os.Chmod(denied, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(denied, 0o700); err != nil {
			t.Error(err)
		}
	})
	if _, err := os.Stat(filepath.Join(root, "51", "exe")); !os.IsPermission(err) {
		t.Fatalf("fixture exe read = %v, want permission denied", err)
	}
	processes, err := inspectProcRuntime(root, executable, binary, config, 1000)
	if err != nil {
		t.Fatal(err)
	}
	processes = runtimeDescendants(processes, runtimeMasterIdentities(processes))
	if len(processes) != 2 || processes[0].PID != 42 || processes[1].PID != 43 {
		t.Fatalf("instance processes = %v, want own master and worker", processes)
	}
}
