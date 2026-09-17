#!/usr/bin/env bash
# Copyright 2026 Arctel.net
# SPDX-License-Identifier: Apache-2.0
#
# check_cordis_architecture.sh
# 验证代码库是否严格遵循 Cordis 插件化架构规约与设计规范。

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"

MODULE=$(cd "${BACKEND_DIR}" && go list -m 2>/dev/null || echo "Wavelet")

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
BOLD='\033[1m'
NC='\033[0m' # No Color

ERRORS=0

log_check() {
    echo -e "${BLUE}==>${NC} ${BOLD}$1${NC}"
}

log_pass() {
    echo -e "  ${GREEN}✓${NC} $1"
}

log_fail() {
    echo -e "  ${RED}✗ [FAIL]${NC} $1" >&2
    ERRORS=$((ERRORS + 1))
}

log_warn() {
    echo -e "  ${YELLOW}! [WARN]${NC} $1"
}

# 确保 ripgrep 可用
if ! command -v rg >/dev/null 2>&1; then
    echo -e "${RED}error: rg (ripgrep) is required to run architecture checks.${NC}" >&2
    exit 1
fi

echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${BOLD}     Cordis Architecture & Spatiotemporal Composability Linter   ${NC}"
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"

# ==============================================================================
# 1. 微内核绝对隔离 (Core Micro-Kernel Isolation)
# ==============================================================================
log_check "1. 检查微内核 (backend/core/) 纯洁度..."

# 1.1 禁止直接依赖重型 Web/ORM/Worker/Config 框架
CORE_FRAMEWORK_IMPORTS=$(rg -n '"github.com/gin-gonic/gin"|"gorm.io/gorm"|"github.com/hibiken/asynq"|"github.com/robfig/cron|"github.com/spf13/viper"|"github.com/mitchellh/mapstructure"' \
    "${BACKEND_DIR}/core/" --glob '*.go' -g '!*contracts*' -g '!*_test.go' || true)

if [ -n "${CORE_FRAMEWORK_IMPORTS}" ]; then
    log_fail "backend/core/ 严禁导入具体 Web/ORM/Worker/Config 运行时框架 (gin, gorm, asynq, cron, viper, mapstructure):"
    echo "${CORE_FRAMEWORK_IMPORTS}" >&2
else
    log_pass "backend/core/ 无重型框架依赖"
fi

# 1.2 core/ 禁止导入任何插件
CORE_PLUGIN_IMPORTS=$(rg -n "\"${MODULE}/plugins/|\"${MODULE}/downstream/|\"${MODULE}/openflare/" \
    "${BACKEND_DIR}/core/" --glob '*.go' -g '!*_test.go' || true)

if [ -n "${CORE_PLUGIN_IMPORTS}" ]; then
    log_fail "backend/core/ 严禁直接依赖具体插件 (plugins/, downstream/ 或 openflare/):"
    echo "${CORE_PLUGIN_IMPORTS}" >&2
else
    log_pass "backend/core/ 零插件反向依赖"
fi

# ==============================================================================
# 2. 服务契约纯洁度 (Contracts Cleanliness)
# ==============================================================================
log_check "2. 检查契约层 (backend/core/contracts/) 抽象纯洁度..."

CONTRACTS_PLUGIN_IMPORTS=$(rg -n "\"${MODULE}/plugins/|\"${MODULE}/downstream/|\"${MODULE}/openflare/|\"github.com/gin-gonic/gin\"|\"github.com/hibiken/asynq\"" \
    "${BACKEND_DIR}/core/contracts/" --glob '*.go' || true)

if [ -n "${CONTRACTS_PLUGIN_IMPORTS}" ]; then
    log_fail "backend/core/contracts/ 必须保持纯 Interface/DTO，严禁导入插件或 Web/Worker 框架依赖:"
    echo "${CONTRACTS_PLUGIN_IMPORTS}" >&2
else
    log_pass "backend/core/contracts/ 纯抽象无侵入"
fi

# 2.2 契约层禁止持久化耦合（表名映射 / ORM tag）
# DTO 一旦携带 TableName()，调用方就能用 Table("w_users") 直查他人表，
# 单表所有者原则与 contracts 抽象同时失效。
# 注意：不禁 gorm import 本身 —— DBService 契约合法返回 gorm 类型。
CONTRACTS_PERSISTENCE=$(rg -n 'func .*\)?\s*TableName\(\)|\bgorm:"' \
    "${BACKEND_DIR}/core/contracts/" --glob '*.go' -g '!*_test.go' || true)

if [ -n "${CONTRACTS_PERSISTENCE}" ]; then
    log_fail "backend/core/contracts/ 为跨插件纯 DTO，严禁携带表名映射或 ORM tag（持久化归属唯一所有者插件，否则调用方可绕过契约直读他人表）:"
    echo "${CONTRACTS_PERSISTENCE}" >&2
else
    log_pass "backend/core/contracts/ 无持久化耦合"
fi

# ==============================================================================
# 3. 基础包纯洁度 (backend/pkg/ Purity)
# ==============================================================================
log_check "3. 检查基础库 (backend/pkg/) 纯洁度..."

# 3.1 pkg/ 严禁导入 plugins/, downstream/, openflare/
PKG_PLUGIN_IMPORTS=$(rg -n "\"${MODULE}/plugins/|\"${MODULE}/downstream/|\"${MODULE}/openflare/" \
    "${BACKEND_DIR}/pkg/" --glob '*.go' -g '!*testhelper*' -g '!*_test.go' || true)

if [ -n "${PKG_PLUGIN_IMPORTS}" ]; then
    log_fail "backend/pkg/ 严禁导入任何上层 plugins/, downstream/ 或 openflare/:"
    echo "${PKG_PLUGIN_IMPORTS}" >&2
else
    log_pass "backend/pkg/ 零插件依赖"
fi

# 3.2 pkg/ 严禁导入 core/ (微内核与契约层)
PKG_CORE_IMPORTS=$(rg -n "\"${MODULE}/core(/|\")" \
    "${BACKEND_DIR}/pkg/" --glob '*.go' -g '!*testhelper*' -g '!*_test.go' || true)

if [ -n "${PKG_CORE_IMPORTS}" ]; then
    log_fail "backend/pkg/ 属于通用基础库，严禁依赖项目上层内核或契约 (${MODULE}/core/*):"
    echo "${PKG_CORE_IMPORTS}" >&2
else
    log_pass "backend/pkg/ 零 core/ 依赖"
fi

# 3.3 pkg/util/ 严禁导入 Gin / ORM / Session 框架
UTIL_FRAMEWORK_IMPORTS=$(rg -n '"gorm.io/gorm"|"github.com/gorilla/sessions"|"github.com/gin-gonic/gin"' \
    "${BACKEND_DIR}/pkg/util/" --glob '*.go' -g '!*_test.go' || true)

if [ -n "${UTIL_FRAMEWORK_IMPORTS}" ]; then
    log_fail "backend/pkg/util/ 必须保持纯粹，禁止导入 gin、gorm、sessions 等 Web/数据库/会话框架包:"
    echo "${UTIL_FRAMEWORK_IMPORTS}" >&2
else
    log_pass "backend/pkg/util/ 保持纯净无状态"
fi

# ==============================================================================
# 4. 全量跨插件直接调用拦截 (Universal Cross-Plugin Import Guard)
# ==============================================================================
log_check "4. 检查插件间隔离性 (严禁跨插件直接 import，必须面向 core/contracts 编程)..."

CROSS_PLUGIN_IMPORTS=""

# 遍历 plugins/ 下的所有类别 (domain, infra, drivers) 和子插件
for category_dir in "${BACKEND_DIR}"/plugins/*/; do
    [ -d "$category_dir" ] || continue
    category=$(basename "$category_dir")
    for plugin_dir in "$category_dir"*/; do
        [ -d "$plugin_dir" ] || continue
        plugin_name=$(basename "$plugin_dir")

        self_prefix="${MODULE}/plugins/${category}/${plugin_name}"

        # 查找该插件内所有的 "Wavelet/plugins/" 导入，排除自身前缀和测试文件
        cross_imports=$(rg -n "\"${MODULE}/plugins/" "${plugin_dir}" \
            -g '*.go' -g '!*_test.go' 2>/dev/null | rg -v "\"${self_prefix}(/|\")" || true)

        if [ -n "$cross_imports" ]; then
            CROSS_PLUGIN_IMPORTS="${CROSS_PLUGIN_IMPORTS}\n[${category}/${plugin_name} 违规引用其他插件]:\n${cross_imports}\n"
        fi
    done
done

# 检查 downstream/ 下的下游插件
if [ -d "${BACKEND_DIR}/downstream/plugins" ]; then
    for downstream_dir in "${BACKEND_DIR}"/downstream/plugins/*/; do
        [ -d "$downstream_dir" ] || continue
        downstream_name=$(basename "$downstream_dir")
        downstream_cross=$(rg -n "\"${MODULE}/plugins/" "${downstream_dir}" \
            -g '*.go' -g '!*_test.go' 2>/dev/null || true)
        if [ -n "$downstream_cross" ]; then
            CROSS_PLUGIN_IMPORTS="${CROSS_PLUGIN_IMPORTS}\n[downstream/${downstream_name} 违规直接引用内部插件实现]:\n${downstream_cross}\n"
        fi
    done
fi

# 检查 openflare/plugins/ 下的下游插件间隔离性
if [ -d "${BACKEND_DIR}/openflare/plugins" ]; then
    for openflare_dir in "${BACKEND_DIR}"/openflare/plugins/*/; do
        [ -d "$openflare_dir" ] || continue
        openflare_name=$(basename "$openflare_dir")
        openflare_self_prefix="${MODULE}/openflare/plugins/${openflare_name}"

        # 检查 openflare 插件之间是否违规跨插件直接 import
        openflare_cross=$(rg -n "\"${MODULE}/openflare/plugins/" "${openflare_dir}" \
            -g '*.go' -g '!*_test.go' 2>/dev/null | rg -v "\"${openflare_self_prefix}(/|\")" || true)

        if [ -n "$openflare_cross" ]; then
            CROSS_PLUGIN_IMPORTS="${CROSS_PLUGIN_IMPORTS}\n[openflare/plugins/${openflare_name} 违规引用其他 openflare 插件]:\n${openflare_cross}\n"
        fi

        # 检查 openflare 插件是否违规直接引用上游内部插件实现 (Wavelet/plugins/)
        openflare_upstream_cross=$(rg -n "\"${MODULE}/plugins/" "${openflare_dir}" \
            -g '*.go' -g '!*_test.go' 2>/dev/null || true)
        if [ -n "$openflare_upstream_cross" ]; then
            CROSS_PLUGIN_IMPORTS="${CROSS_PLUGIN_IMPORTS}\n[openflare/plugins/${openflare_name} 违规直接引用上游插件实现]:\n${openflare_upstream_cross}\n"
        fi
    done
fi

if [ -n "${CROSS_PLUGIN_IMPORTS}" ]; then
    log_fail "发现跨插件直接依赖违规（必须通过 core/contracts 契约接口或 EventBus 解耦，严禁跨插件直接 import 具体包）:"
    echo -e "${CROSS_PLUGIN_IMPORTS}" >&2
else
    log_pass "所有插件 100% 解耦，零跨插件直接 import"
fi

# ==============================================================================
# 5. 数据库规范与 GORM AutoMigrate 禁令 (Database Migration & ORM Rules)
# ==============================================================================
log_check "5. 检查数据库操作与 AutoMigrate 禁令..."

AUTOMIGRATE_CALLS=$(rg -n '\.AutoMigrate\(' "${BACKEND_DIR}" \
    --glob '*.go' -g '!*_test.go' -g '!*testhelper*' || true)

if [ -n "${AUTOMIGRATE_CALLS}" ]; then
    log_fail "严禁在生产代码中使用 GORM AutoMigrate（必须使用插件自包含 Goose SQL 迁移）:"
    echo "${AUTOMIGRATE_CALLS}" >&2
else
    log_pass "零 GORM AutoMigrate，100% Goose SQL 迁移管理"
fi

# ==============================================================================
# 6. 并发安全规范 (Goroutine Concurrency Safety)
# ==============================================================================
log_check "6. 检查并发安全规范 (禁止生产代码中使用裸 go 启动 goroutine)..."

# 同时覆盖 `go func() {...}()` 匿名形式与 `go worker.run()` / `go loop()` 命名调用形式：
# 两者都不具备 panic 恢复能力，被调方一旦 panic 会直接击穿整个进程。
# 例外：util.Go 自身的实现与事件总线 (它们内部已 recover)。
BARE_GO_ROUTINES=$(rg -n --pcre2 '^[[:space:]]*go\s+(func\s*[\w{]|\w+(\.\w+)*\s*[\({])' "${BACKEND_DIR}" \
    --glob '*.go' -g '!*_test.go' -g '!goroutine.go' -g '!events.go' || true)

if [ -n "${BARE_GO_ROUTINES}" ]; then
    log_fail "生产代码严禁裸 'go' 启动 goroutine（含 'go func()' 与 'go xxx()' 命名调用），必须使用 'util.Go' 确保 panic 恢复与调用栈追踪:"
    echo "${BARE_GO_ROUTINES}" >&2
else
    log_pass "并发调用统一使用 util.Go 具备 panic 恢复能力"
fi

# ==============================================================================
# 7. 插件驱动无关性 (Driver-Agnostic Plugins)
# ==============================================================================
log_check "7. 检查业务/基础设施插件的驱动无关性 (禁止绑定具体 Worker/调度器运行时类型)..."

# 业务插件只能通过 ctx.Task() / ctx.Schedule() 扩展点声明工作，不得 import Worker
# 驱动运行时类型：绑定 *asynq.Task 之类的签名只被 asynq 驱动满足，换用 in-process
# Worker 后 invokeHandler 会以 "unsupported handler type" 直接拒绝，任务永不执行。
RUNTIME_BOUND=$(rg -n '"github.com/hibiken/asynq"' \
    "${BACKEND_DIR}/plugins/domain/" "${BACKEND_DIR}/plugins/infra/" \
    --glob '*.go' -g '!*_test.go' 2>/dev/null || true)

if [ -n "${RUNTIME_BOUND}" ]; then
    log_fail "业务/基础设施插件严禁依赖具体 Worker/调度器运行时（必须面向 ctx.Task()/ctx.Schedule() 扩展点编程，否则更换驱动后任务无法执行）:"
    echo "${RUNTIME_BOUND}" >&2
else
    log_pass "业务/基础设施插件保持驱动无关，可自由切换 Worker 驱动"
fi

# ==============================================================================
# 总结与判定
# ==============================================================================
echo -e "${BOLD}═══════════════════════════════════════════════════════════════${NC}"
if [ ${ERRORS} -eq 0 ]; then
    echo -e "${GREEN}${BOLD}✓ 所有 Cordis 架构合规性检查全部通过 (0 Violations)!${NC}"
    exit 0
else
    echo -e "${RED}${BOLD}✗ 发现 ${ERRORS} 项 Cordis 架构规约违背，请根据上述提示修复！${NC}" >&2
    exit 1
fi
