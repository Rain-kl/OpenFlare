// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"os/exec"
	"strings"
	"testing"
)

// forbiddenImportPrefixes 上层应用禁止直接引入的日志后端/驱动包前缀。
var forbiddenImportPrefixes = []string{
	"github.com/ClickHouse/clickhouse-go/",
	"gorm.io/driver/clickhouse",
}

// allowedInfraPersistence 允许 apps 引入的 infra/persistence 子包。
var allowedInfraPersistence = []string{
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/batchwriter", // batchwriter 统计类型
	"github.com/Rain-kl/Wavelet/internal/infra/persistence/idgen",       // 雪花 ID 生成（无日志依赖）
}

func TestAppsMustNotImportLogBackendDirectly(t *testing.T) {
	t.Chdir("../../..") // module root，保证 ./internal/apps/... 可解析
	out, err := exec.Command("go", "list", "-test", "-f", `{{.ImportPath}} {{join .Imports " "}}`, "./internal/apps/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		if !strings.HasPrefix(pkg, "github.com/Rain-kl/Wavelet/internal/apps") {
			continue
		}
		for _, imp := range fields[1:] {
			for _, prefix := range forbiddenImportPrefixes {
				if imp == prefix || strings.HasPrefix(imp, prefix) {
					t.Errorf("%s must not import forbidden log backend %s", pkg, imp)
				}
			}
			if strings.HasPrefix(imp, "github.com/Rain-kl/Wavelet/internal/infra/persistence/") {
				allowed := false
				for _, a := range allowedInfraPersistence {
					if imp == a || strings.HasPrefix(imp, a+"/") {
						allowed = true
						break
					}
				}
				if !allowed {
					t.Errorf("%s must not import infra/persistence subpackage directly: %s", pkg, imp)
				}
			}
		}
	}
}
