---
name: go-packages
description: Use when creating Go packages, organizing imports, managing dependencies, or deciding how to structure Go code into packages. Also use when starting a new Go project or splitting a growing codebase into packages, even if the user doesn't explicitly ask about package organization. Does not cover naming individual identifiers (see go-naming).
license: Apache-2.0
metadata:
  sources: "Google Style Guide, Uber Style Guide, Go Wiki CodeReviewComments"
---

# Go 包和 Import

> **本技能不适用的场景**：对于包内单个标识符的命名，参见 [go-naming](../go-naming/SKILL.md)。对于单文件中函数的组织，参见 [go-functions](../go-functions/SKILL.md)。对于强制执行 import 规则的 linter 配置，参见 [go-linting](../go-linting/SKILL.md)。

## 包组织

### 避免 Util 包

包名应描述包提供的内容。避免使用 `util`、`helper`、`common` 等泛化名称——它们会模糊含义并导致 import 冲突。

```go
// 好：有意义的包名
db := spannertest.NewDatabaseFromFile(...)
_, err := f.Seek(0, io.SeekStart)

// 不好：模糊的名称遮蔽含义
db := test.NewDatabaseFromFile(...)
_, err := f.Seek(0, common.SeekStart)
```

泛化名称可以作为名称的*一部分*（例如 `stringutil`），但不应成为整个包名。

### Package Size

| 问题 | 操作 |
|------|------|
| 你能用一句话描述它的用途吗？ | 不能 → 按职责拆分 |
| 文件中从未共享未导出的符号？ | 这些文件可以是独立的包 |
| 不同的用户群体使用不同部分？ | 按用户边界拆分 |
| Godoc 页面过于庞大？ | 拆分以提高可发现性 |

**不要拆分**的原因仅仅是文件很长、创建只有单一类型的包，或会产生循环依赖。

> 在决定是否拆分或合并包、组织包内文件或构建 CLI 程序时，阅读 [references/PACKAGE-SIZE.md](references/PACKAGE-SIZE.md)。

---

## Import

Import 按组组织，组之间用空行分隔。标准库包始终放在第一组。使用
[goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports) 自动管理。

```go
import (
    "fmt"
    "os"

    "github.com/foo/bar"
    "rsc.io/goversion/version"
)
```

**快速规则：**

| 规则 | 指导 |
|------|------|
| 分组 | 标准库优先，然后是外部包。扩展分组：标准库 → 其他 → proto → 副作用 |
| 重命名 | 除非冲突，否则避免重命名。重命名最本地的 import。Proto 包加 `pb` 后缀 |
| 空白 import（`import _`） | 仅在 `main` 包或测试中使用 |
| 点 import（`import .`） | 永不使用，除非用于循环依赖的测试文件 |

> 在组织扩展分组的 import、重命名 proto 包或决定使用空白/点 import 时，阅读 [references/IMPORTS.md](references/IMPORTS.md)。

---

## 避免 init()

尽可能避免 `init()`。当不可避免时，它必须是：

1. 完全确定性的
2. 不依赖于其他 `init()` 的执行顺序
3. 不依赖环境状态（环境变量、工作目录、参数）
4. 不进行 I/O（文件系统、网络、系统调用）

**可接受的使用场景**：无法用单个赋值完成的复杂表达式、可插拔钩子（例如 `database/sql` 方言）、确定性预计算。

> 在需要将 init() 重构为显式函数或理解可接受的 init() 使用场景时，阅读 [references/PACKAGE-SIZE.md](references/PACKAGE-SIZE.md)。

---

## Main 中的退出

仅在 `main()` 中调用 `os.Exit` 或 `log.Fatal*`。所有其他函数应返回 error。

**原因**：不明显的控制流、不可测试、`defer` 语句被跳过。

**最佳实践**：使用 `run()` 模式——将逻辑提取到
`func run() error` 中，在 `main()` 中调用并使用单一退出点：

```go
func main() {
    if err := run(); err != nil {
        log.Fatal(err)
    }
}
```

> 在实现 run() 模式、构建 CLI 子命令或选择 flag 命名约定时，阅读 [references/PACKAGE-SIZE.md](references/PACKAGE-SIZE.md)。

---

## 命令行 Flag

> **建议**：仅在 `package main` 中定义 flag。

- Flag 名称使用 `snake_case`：`--output_dir` 而非 `--outputDir`
- 库应通过参数接收配置，而非直接读取 flag——
  这使它们可测试且可复用
- 优先使用标准 `flag` 包；仅在需要 POSIX 约定
  （双破折号、单字符快捷方式）时使用 `pflag`

```go
// 好：Flag 在 main 中定义，作为参数传递给库
func main() {
    outputDir := flag.String("output_dir", ".", "directory for output files")
    flag.Parse()
    if err := mylib.Generate(*outputDir); err != nil {
        log.Fatal(err)
    }
}
```

---

## 相关技能

- **包命名**：在选择包名、避免名称重复或命名导出符号时，参见 [go-naming](../go-naming/SKILL.md)
- **跨包的错误处理**：在使用 `%w` vs `%v` 在包边界包装错误时，参见 [go-error-handling](../go-error-handling/SKILL.md)
- **Import linting**：在配置 goimports local-prefixes 或强制执行 import 分组时，参见 [go-linting](../go-linting/SKILL.md)
- **全局状态**：在用显式初始化替换 `init()` 或避免可变全局变量时，参见 [go-defensive](../go-defensive/SKILL.md)
