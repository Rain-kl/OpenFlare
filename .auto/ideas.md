# Ideas backlog (代码质量)

## 已尝试并收尾（2026-08-16 会话，14 个实验，108→8）

- 生产代码 golangci 扩展集 13 类 linter 全量清理（modernize/perfsprint/
  errorlint/canonicalheader/usestdlibvars/intrange/wastedassign/errname/
  forcetypeassert/prealloc/gosec/recvcheck/exhaustive），剩余 8 处全部为
  有据可查的刻意保留项（telegram %v、3 处嵌套 struct omitempty、
  3 处 not-found 惯例、1 处 encoding/json 接收者混合）。
- 测试代码质量维度（testifylint/usetesting/thelper）25→0。
- 前端 eslint/tsc 0。
- 修复中积累的工具经验：golangci-lint v2 `--fix` 的 import 管理不可靠，
  跑完必须 `goimports -w`；`--max-issues-per-linter=0` 才能拿到全量清单
  （默认 50 + max-same-issues=3 会掩盖重复模式）；cyclop 与 exhaustive
  有张力（显式 case 计入复杂度）。

## 未来可深化方向（均经评估）

- 测试可运行性修复：`go test ./internal/...` 目前在 main 上就有失败
  （无本地 redis、frpc 进程测试 flaky）。修复这些环境问题后，可以把
  `go test` 加入 checks.sh，解锁 paralleltest/tparallel 维度
  （t.Parallel 提速 + 正确性，目前因共享状态+不可运行而放弃）。
- frontend biome 格式漂移（76 文件）：一次性 `make format` 提交，
  与质量修复分开做，不进基准。
- fieldalignment：结构体内存布局优化，但会改变 JSON key 顺序且有
  位置字面量风险 —— 若做，需按文件人工核对，不进自动基准。
- Go 1.26 新特性扫描：`go vet` 新分析器、golangci-lint 新 linter
  （如 recvcheck 之后的 new receivers 检查）随版本跟进。
- 文档/示例代码（docs/、scripts/）质量：目前不在 golangci 范围（tests:false
  之外还有 scripts 目录），可用同一扩展集扫 scripts/ 下的 main.go。

## 会话收尾（2026-08-16，run #23 后）

- 已确认收敛：基准 5 维全下限、-race 全仓清零、双端测试全绿、发布构建可复现、
  config.example.yaml ↔ model.go 同步无漂移、无 flaky 测试。
- 明确评估为不值得做的方向：paralleltest/tparallel（共享全局状态风险）、
  fieldalignment（JSON key 顺序变化）、biome 格式漂移（纯噪声）、
  frpc/frps 慢测试注入 backoff（为省 ~40s 改生产时序逻辑，不值）。
- 未来如继续：可周期跑 `go test -race ./...` 全量（frpc/frps 慢套件）；
  或前端 a11y 用 axe 做浏览器级审计（超出 eslint 静态规则）。
