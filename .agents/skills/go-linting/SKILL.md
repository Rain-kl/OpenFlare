---
name: go-linting
description: Use when setting up linting for a Go project, configuring golangci-lint, or adding Go checks to a CI/CD pipeline. Also use when starting a new Go project and deciding which linters to enable, even if the user only asks about "code quality" or "static analysis" without mentioning specific linter names. Does not cover code review process (see go-code-review).
license: Apache-2.0
metadata:
  sources: "Uber Style Guide"
allowed-tools: Bash(bash:*)
---

# Go Lint

## 核心原则

比任何"推荐"的 linter 集合更重要的是：**在整个代码库中一致地进行 lint**。

一致的 lint 有助于捕获常见问题，并在不过度限制的情况下建立高标准的代码质量。

---

## 设置步骤

1. 使用下面的配置创建 `.golangci.yml`
2. 运行 `golangci-lint run ./...`
3. 如果出现错误，按类别逐一修复（先格式化，再 vet，再风格）
4. 重新运行直到通过

---

## 最低推荐 Linter

这些 linter 能捕获最常见的问题，同时保持高质量标准：

| Linter | 用途 |
|--------|------|
| [errcheck](https://github.com/kisielk/errcheck) | 确保错误被处理 |
| [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) | 格式化代码和管理导入 |
| [revive](https://github.com/mgechev/revive) | 常见风格错误（golint 的现代替代品） |
| [govet](https://pkg.go.dev/cmd/vet) | 分析代码中的常见错误 |
| [staticcheck](https://staticcheck.dev) | 各种静态分析检查 |

> **注意**：`revive` 是现已弃用的 `golint` 的现代、更快的替代品。

---

## Lint 运行器：golangci-lint

使用 [golangci-lint](https://github.com/golangci/golangci-lint) 作为你的 lint 运行器。参见 uber-go/guide 的 [示例 .golangci.yml](https://github.com/uber-go/guide/blob/master/.golangci.yml)。

---

## 示例配置

> 在创建新的 `.golangci.yml` 或将现有配置与推荐基线进行比较时，参见 `assets/golangci.yml`。

在项目根目录创建 `.golangci.yml`：

```yaml
linters:
  enable:
    - errcheck
    - goimports
    - revive
    - govet
    - staticcheck

linters-settings:
  goimports:
    local-prefixes: github.com/your-org/your-repo
  revive:
    rules:
      - name: blank-imports
      - name: context-as-argument
      - name: error-return
      - name: error-strings
      - name: exported

run:
  timeout: 5m
```

### 运行

```bash
# 安装
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# 运行所有 linter
golangci-lint run

# 对特定路径运行
golangci-lint run ./pkg/...
```

---

## 额外推荐的 Linter

除了最低集合之外，在生产项目中可以考虑以下 linter：

| Linter | 用途 | 何时启用 |
|--------|------|----------|
| [gosec](https://github.com/securego/gosec) | 安全漏洞检测 | 处理用户输入的服务始终启用 |
| [ineffassign](https://github.com/gordonklaus/ineffassign) | 检测无效赋值 | 始终——捕获死代码 |
| [misspell](https://github.com/client9/misspell) | 纠正注释/字符串中的常见拼写错误 | 始终 |
| [gocyclo](https://github.com/fzipp/gocyclo) | 圈复杂度阈值 | 当函数超过约 15 的复杂度时 |
| [exhaustive](https://github.com/nishanths/exhaustive) | 确保 switch 覆盖所有枚举值 | 使用 iota 枚举时 |
| [bodyclose](https://github.com/timakin/bodyclose) | 检测未关闭的 HTTP 响应体 | HTTP 客户端代码始终启用 |

---

## Nolint 指令

在抑制 lint 发现时，始终说明原因：

```go
//nolint:errcheck // 即发即忘的日志；错误不可操作
_ = logger.Sync()
```

规则：
- 使用 `//nolint:lintername`——永远不要使用裸 `//nolint`
- 将注释放在与发现相同的行
- 在 `//` 之后包含理由说明

---

## CI/CD 集成

### GitHub Actions

```yaml
# .github/workflows/lint.yml
name: Lint
on: [push, pull_request]
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - uses: golangci/golangci-lint-action@v6
        with:
          version: latest
```

### Pre-commit Hook

```bash
#!/bin/sh
# .git/hooks/pre-commit
golangci-lint run --new-from-rev=HEAD~1
```

使用 `--new-from-rev` 只对更改的代码进行 lint，保持快速反馈循环。

---

## 可用脚本

- **`scripts/setup-lint.sh`**——生成 `.golangci.yml` 并运行初始 lint

```bash
bash scripts/setup-lint.sh github.com/your-org/your-repo
bash scripts/setup-lint.sh --force github.com/your-org/your-repo  # 覆盖现有配置
bash scripts/setup-lint.sh --dry-run                               # 预览配置
bash scripts/setup-lint.sh --json                                  # 结构化输出
```

> **验证**：在生成 `.golangci.yml` 后，运行 `golangci-lint run ./...` 验证配置有效并产生预期输出。如果因配置错误而失败，修复后重试。

> `scripts/setup-lint.sh` 生成**最低**配置（5 个核心 linter）。
> 对于已有项目，使用 `assets/golangci.yml` 作为起点——
> 它增加了 gosec、ineffassign、misspell、gocyclo 和 bodyclose。

---

## 快速参考

| 任务 | 命令/操作 |
|------|-----------|
| 安装 golangci-lint | `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest` |
| 运行 linter | `golangci-lint run` |
| 对路径运行 | `golangci-lint run ./pkg/...` |
| 配置文件 | 项目根目录的 `.golangci.yml` |
| CI 集成 | 在管道中运行 `golangci-lint run` |
| Nolint 指令 | `//nolint:name // 原因`——永远不要使用裸 `//nolint` |
| CI 集成 | 使用 `golangci/golangci-lint-action` 用于 GitHub Actions |
| Pre-commit | `golangci-lint run --new-from-rev=HEAD~1` |

### Linter 选择指南

| 当你需要... | 使用 |
|-------------|------|
| 错误处理覆盖率 | errcheck |
| 导入格式化 | goimports |
| 风格一致性 | revive |
| Bug 检测 | govet、staticcheck |
| 以上全部 | golangci-lint 配合配置 |

---

## 相关技能

- **风格基础**：在解决 linter 执行的风格问题（格式化、嵌套、命名）时，参见 [go-style-core](../go-style-core/SKILL.md)
- **代码审查**：在将 linter 输出与手动审查清单结合使用时，参见 [go-code-review](../go-code-review/SKILL.md)
- **错误处理**：在 errcheck 标记未处理的错误并需要决定如何处理时，参见 [go-error-handling](../go-error-handling/SKILL.md)
- **测试**：在 CI 管道中将 linter 与测试一起运行时，参见 [go-testing](../go-testing/SKILL.md)
