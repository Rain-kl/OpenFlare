---
name: go-code-review
description: Use when reviewing Go code or checking code against community style standards. Also use proactively before submitting a Go PR or when reviewing any Go code changes, even if the user doesn't explicitly request a style review. Does not cover language-specific syntax — delegates to specialized skills.
license: Apache-2.0
compatibility: Web server example in references uses slog (Go 1.21+)
metadata:
  sources: "Go Wiki CodeReviewComments, Uber Style Guide"
allowed-tools: Bash(bash:*)
---

# Go 代码审查清单

## 审查流程

> 使用 `assets/review-template.md` 格式化代码审查输出，确保结构与"必须修复 / 建议修复 / 吹毛求疵"的严重程度分组保持一致。

1. 运行 `gofmt -d .` 和 `go vet ./...` 先捕获机械性问题
2. 逐文件阅读 diff；对于每个文件，按以下类别顺序检查
3. 标记问题时需要包含具体行号引用和规则名称
4. 审查完所有文件后，重新阅读标记项以确认它们是真实的问题
5. 按严重程度分组汇总发现（必须修复、建议修复、吹毛求疵）

> **验证**：完成审查后，再次阅读 diff 以验证每个标记的问题都是真实的。删除任何无法用具体行号引用的发现。

---

## 格式化

- [ ] **gofmt**：代码已使用 `gofmt` 或 `goimports` 格式化 → [go-linting](../go-linting/SKILL.md)

---

## 文档

- [ ] **注释句子**：注释是完整的句子，以被描述的名称开头，以句号结尾 → [go-documentation](../go-documentation/SKILL.md)
- [ ] **文档注释**：所有导出名称都有文档注释；非平凡的未导出声明也应有 → [go-documentation](../go-documentation/SKILL.md)
- [ ] **包注释**：包注释出现在 package 子句附近，无空行 → [go-documentation](../go-documentation/SKILL.md)
- [ ] **命名结果参数**：仅当它们能澄清含义时使用（例如，多个相同类型返回值），而不仅仅是为了启用裸返回 → [go-documentation](../go-documentation/SKILL.md)

---

## 错误处理

- [ ] **处理错误**：不使用 `_` 丢弃错误；处理、返回或（在特殊情况下）panic → [go-error-handling](../go-error-handling/SKILL.md)
- [ ] **错误字符串**：小写开头，无标点（除非以专有名词/首字母缩略词开头） → [go-error-handling](../go-error-handling/SKILL.md)
- [ ] **带内错误**：不使用魔术值（-1、""、nil）；使用带 error 或 ok bool 的多返回值 → [go-error-handling](../go-error-handling/SKILL.md)
- [ ] **错误流缩进**：先处理错误并返回；保持正常路径的缩进最小化 → [go-error-handling](../go-error-handling/SKILL.md)

---

## 命名

- [ ] **MixedCaps**：使用 `MixedCaps` 或 `mixedCaps`，不使用下划线；未导出使用 `maxLength` 而非 `MAX_LENGTH` → [go-naming](../go-naming/SKILL.md)
- [ ] **首字母缩略词**：保持一致的大小写：`URL`/`url`、`ID`/`id`、`HTTP`/`http`（例如 `ServeHTTP`、`xmlHTTPRequest`） → [go-naming](../go-naming/SKILL.md)
- [ ] **变量名**：有限作用域用短名称（`i`、`r`、`c`）；更广作用域用较长名称 → [go-naming](../go-naming/SKILL.md)
- [ ] **接收器名称**：类型的一两个字母缩写（`c` 代表 `Client`）；不使用 `this`、`self`、`me`；各方法之间保持一致 → [go-naming](../go-naming/SKILL.md)
- [ ] **包名**：不重复（使用 `chubby.File` 而非 `chubby.ChubbyFile`）；避免 `util`、`common`、`misc` → [go-packages](../go-packages/SKILL.md)
- [ ] **避免内置名称**：不遮蔽 `error`、`string`、`len`、`cap`、`append`、`copy`、`new`、`make` → [go-declarations](../go-declarations/SKILL.md)

---

## 并发

- [ ] **Goroutine 生命周期**：明确 goroutine 何时/是否退出；如不明显则添加文档 → [go-concurrency](../go-concurrency/SKILL.md)
- [ ] **同步函数**：优先同步而非异步；让调用者在需要时添加并发 → [go-concurrency](../go-concurrency/SKILL.md)
- [ ] **Context**：作为第一个参数；不放在 struct 中；不自定义 Context 类型；即使认为不需要也应传递 → [go-context](../go-context/SKILL.md)

---

## 接口

- [ ] **接口位置**：在消费方包中定义，而非实现方；生产者返回具体类型 → [go-interfaces](../go-interfaces/SKILL.md)
- [ ] **不提前定义接口**：不在使用前定义；不在实现方"为了 mock"而定义 → [go-interfaces](../go-interfaces/SKILL.md)
- [ ] **接收器类型**：如果会修改状态、有 sync 字段或体积大，使用指针；小的不可变类型使用值；不要混用 → [go-interfaces](../go-interfaces/SKILL.md)

---

## 数据结构

- [ ] **空切片**：优先使用 `var t []string`（nil）而非 `t := []string{}`（非 nil 零长度） → [go-data-structures](../go-data-structures/SKILL.md)
- [ ] **复制**：小心复制含指针/切片字段的结构体；不按值复制 `*T` 方法的接收器 → [go-data-structures](../go-data-structures/SKILL.md)

---

## 安全性

- [ ] **加密随机数**：密钥使用 `crypto/rand`，不使用 `math/rand` → [go-defensive](../go-defensive/SKILL.md)
- [ ] **不 panic**：常规错误处理使用 error 返回；仅在真正特殊的情况下 panic → [go-defensive](../go-defensive/SKILL.md)

---

## 声明与初始化

- [ ] **分组相似的**：相关的 `var`/`const`/`type` 放在括号块中；不相关的分开 → [go-declarations](../go-declarations/SKILL.md)
- [ ] **var vs :=**：有意使用零值时用 `var`；显式赋值时用 `:=` → [go-declarations](../go-declarations/SKILL.md)
- [ ] **缩小作用域**：将声明移到使用位置附近；使用 if-init 限制变量作用域 → [go-declarations](../go-declarations/SKILL.md)
- [ ] **Struct 初始化**：始终使用字段名；省略零值字段；零值 struct 使用 `var` → [go-declarations](../go-declarations/SKILL.md)
- [ ] **使用 `any`**：新代码中优先使用 `any` 而非 `interface{}` → [go-declarations](../go-declarations/SKILL.md)

---

## 函数

- [ ] **文件排序**：类型 → 构造函数 → 导出方法 → 未导出方法 → 工具函数 → [go-functions](../go-functions/SKILL.md)
- [ ] **签名格式化**：换行时所有参数各占一行并带尾逗号 → [go-functions](../go-functions/SKILL.md)
- [ ] **裸参数**：为含义不明确的 bool/int 参数添加 `/* name */` 注释，或使用自定义类型 → [go-functions](../go-functions/SKILL.md)
- [ ] **Printf 命名**：接受格式字符串的函数以 `f` 结尾，以便 `go vet` 检查 → [go-functions](../go-functions/SKILL.md)

---

## 风格

- [ ] **行长度**：无硬性限制，但避免令人不适的长行；按语义断行，而非任意长度 → [go-style-core](../go-style-core/SKILL.md)
- [ ] **裸返回**：仅在短函数中使用；中/大函数使用显式返回 → [go-style-core](../go-style-core/SKILL.md)
- [ ] **传值**：不要仅为节省字节而使用指针；小的固定大小类型传 `string` 而非 `*string` → [go-performance](../go-performance/SKILL.md)
- [ ] **字符串拼接**：简单拼接用 `+`；格式化用 `fmt.Sprintf`；循环中用 `strings.Builder` → [go-performance](../go-performance/SKILL.md)

---

## 日志

- [ ] **使用 slog**：新代码使用 `log/slog`，不使用 `log` 或 `fmt.Println` 进行运维日志记录 → [go-logging](../go-logging/SKILL.md)
- [ ] **结构化字段**：日志消息使用静态字符串加键值属性，不使用 fmt.Sprintf → [go-logging](../go-logging/SKILL.md)
- [ ] **适当的级别**：Debug 用于开发者追踪，Info 用于重要事件，Warn 用于可恢复的问题，Error 用于故障 → [go-logging](../go-logging/SKILL.md)
- [ ] **日志中无敏感信息**：PII、凭证和令牌永远不记录在日志中 → [go-logging](../go-logging/SKILL.md)

---

## 导入

- [ ] **导入分组**：标准库优先，然后空行，再外部包 → [go-packages](../go-packages/SKILL.md)
- [ ] **导入重命名**：除非冲突否则避免重命名；冲突时重命名本地/项目特定的导入 → [go-packages](../go-packages/SKILL.md)
- [ ] **空白导入**：`import _ "pkg"` 仅在 main 包或测试中使用 → [go-packages](../go-packages/SKILL.md)
- [ ] **点导入**：仅在测试中用于解决循环依赖 → [go-packages](../go-packages/SKILL.md)

---

## 泛型

- [ ] **何时使用**：仅当多个类型共享相同逻辑且接口不足时 → [go-generics](../go-generics/SKILL.md)
- [ ] **类型别名**：使用定义创建新类型；别名仅用于包迁移 → [go-generics](../go-generics/SKILL.md)

---

## 测试

- [ ] **示例**：包含可运行的 `Example` 函数或演示用法的测试 → [go-documentation](../go-documentation/SKILL.md)
- [ ] **有用的测试失败信息**：消息包含出了什么错、输入、实际值和期望值；顺序为 `got != want` → [go-testing](../go-testing/SKILL.md)
- [ ] **TestMain**：仅当所有测试都需要带清理的公共设置时使用；优先使用作用域化的 helper → [go-testing](../go-testing/SKILL.md)
- [ ] **真实传输**：优先使用 `httptest.NewServer` + 真实客户端而非 mock HTTP → [go-testing](../go-testing/SKILL.md)

---

## 自动化检查

运行自动化预审查检查：

```bash
bash scripts/pre-review.sh ./...         # 文本输出
bash scripts/pre-review.sh --json ./...  # 结构化 JSON 输出
```

或手动：`gofmt -l <path> && go vet ./... && golangci-lint run ./...`

在进入上述清单之前修复所有问题。有关 linter 设置和配置，请参阅 [go-linting](../go-linting/SKILL.md)。

---

## 综合示例

> 在构建生产级 HTTP 服务器并希望验证代码是否正确应用了并发、错误处理、context、文档和命名规范时，阅读 [references/WEB-SERVER.md](references/WEB-SERVER.md)。

---

## 相关 Skill

- **风格基础**：在解决格式化争议或应用"清晰 > 简单 > 简洁"优先级时，请参阅 [go-style-core](../go-style-core/SKILL.md)
- **Linting 设置**：在配置 golangci-lint 或将自动化检查添加到 CI 时，请参阅 [go-linting](../go-linting/SKILL.md)
- **错误策略**：在审查错误包装、哨兵错误或 handle-once 模式时，请参阅 [go-error-handling](../go-error-handling/SKILL.md)
- **命名规范**：在评估标识符名称、接收器名称或包-符号重复时，请参阅 [go-naming](../go-naming/SKILL.md)
- **测试模式**：在审查表驱动结构、失败消息或 helper 使用的测试代码时，请参阅 [go-testing](../go-testing/SKILL.md)
- **并发安全**：在审查 goroutine 生命周期、channel 使用或互斥锁放置时，请参阅 [go-concurrency](../go-concurrency/SKILL.md)
- **日志实践**：在审查日志使用、结构化日志或 slog 配置时，请参阅 [go-logging](../go-logging/SKILL.md)
