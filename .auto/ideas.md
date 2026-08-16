# Ideas backlog (代码质量)

- 后端 204 个 *_test.go 被 repo golangci 配置 `tests: false` 跳过；可以用
  testifylint/usetesting/paralleltest 等对测试代码做一次质量扫描（当前基准不含测试，属于后续深化方向）。
- biome check 发现 76 处（基本全是格式漂移）；repo 只跑 format 不跑 check。
  可作为独立的一次性格式化提交处理，不进基准（避免纯格式噪声污染 metric）。
- repo `make code-check` 还有 `make format`（goimports + biome）契约：迭代中保持已改文件格式化干净。
- 观察 go vet 已启用但 `unusedresult`/`copylocks` 等子检查默认关闭；若需要可考虑仅作发现用，不进基准。
- errorlint 修复中有 4 处 `err != context.Canceled`，若 runner 实际从不 wrap，
  用 errors.Is 是更稳妥的最佳实践，且语义不变 —— 已验证三处 cmd 入口同构。