// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package logstore

import (
	"os/exec"
	"strings"
	"testing"
)

// forbiddenImports 上层应用禁止直接触碰的底层日志实现。
var forbiddenImports = []string{
	"github.com/Rain-kl/Wavelet/internal/repository/analytics",
}

// allowedAnalyticsDelegation 允许直接依赖 analyticsrepo 的委托层：
//   - internal/repository：持久化门面，ListOpenFlareLatestMetricSnapshotsSince 的
//     CH 快速路径仍直连 analyticsrepo（LIMIT 1 BY node_id）；小时级聚合读已改走 logstore；
//   - internal/repository/logstore：CH 后端实现按设计委托 analyticsrepo。
//
// 除此之外，依赖闭包内任何包都禁止引入 analyticsrepo。
var allowedAnalyticsDelegation = map[string]bool{
	"github.com/Rain-kl/Wavelet/internal/repository":          true,
	"github.com/Rain-kl/Wavelet/internal/repository/logstore": true,
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
			for _, forbidden := range forbiddenImports {
				if imp == forbidden && !allowedAnalyticsDelegation[pkg] {
					t.Errorf("%s must not import forbidden log backend %s", pkg, forbidden)
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
