# OpenFlare 引用替换为 GitHub 路径方案评估计划

## 1. 目标与背景 (Goal & Context)
* **需求背景**：当前 OpenFlare 内部组件（Server、Agent、Relay、Flared）之间采用本地包名引用（例如 `openflare`、`openflare-agent`），并使用 Go `replace` 相对路径指向本地目录。这导致代码无法直接以标准的 GitHub 路径（如 `github.com/rain-kl/openflare`）进行分发、远程安装或被外部引用（例如 `go install` 远程二进制会因为 replace 指令失效而报错）。
* **评估目标**：评估将本地引用替换为 `github.com/rain-kl/openflare` 格式的两种可行方案（单模块 Monorepo 方案 vs 多模块 Multi-Module 方案），分析各自的优缺点、工作量及对现有 CI/CD、Docker 镜像构建的影响，给出推荐方案。

## 2. 设计与决策 (Design & Decisions)

### 方案 A：标准 Go 多模块方案 (Multi-Module with Sub-paths)
保留当前 4 个独立的 Go 模块结构，在各自的 `go.mod` 中将模块名改写为符合 GitHub 结构的子路径：
- `openflare-server/go.mod` -> `module github.com/rain-kl/openflare/openflare-server`
- `openflare-relay/go.mod` -> `module github.com/rain-kl/openflare/openflare-relay`
- `openflare-agent/go.mod` -> `module github.com/rain-kl/openflare/openflare-agent`
- `openflared/go.mod` -> `module github.com/rain-kl/openflare/openflared`

同时，其他模块（Relay, Agent, Flared）的 `go.mod` 中的 `replace` 修改为：
`replace github.com/rain-kl/openflare/openflare-server => ../openflare-server`

#### 优缺点分析：
* **优点**：
  - **模块边界清晰**：各二进制模块依赖独立。例如 `openflare-agent` 不会引入 Server 依赖的 GORM、Gin、Swagger 等库，保持各自模块的 `go.sum` 纯净。
  - **改动小**：对 Dockerfile 和 GitHub Workflows 影响极小，构建上下文仍可保持原样。
* **缺点**：
  - **远程安装不可用**：仍然需要在 `go.mod` 中保留 `replace` 指令。由于 Go 不允许在远程 `go install` 或 `go get` 时解析本地相对路径的 `replace` 指令，用户依然无法直接通过 `go install github.com/rain-kl/openflare/openflared/cmd/flared@latest` 安装，必须先克隆整个仓库到本地再构建。

---

### 方案 B：统一单模块方案 (Unified Single Module Monorepo - 推荐)
将整个仓库合并为一个 Go 模块。在仓库根目录下创建 `go.mod`，模块名为 `github.com/rain-kl/openflare`，并删除子目录中的所有 `go.mod` 和 `go.sum`。

所有内部包导入路径统一改写为：
- `"github.com/rain-kl/openflare/openflare-server/..."`
- `"github.com/rain-kl/openflare/openflare-relay/..."`
- `"github.com/rain-kl/openflare/openflare-agent/..."`
- `"github.com/rain-kl/openflare/openflared/..."`

#### 优缺点分析：
* **优点**：
  - **彻底摆脱 replace**：完全不需要在 `go.mod` 中写 `replace` 指令，代码清爽、易于维持。
  - **支持远程 Go 工具链**：用户和开发者可以直接使用 `go install github.com/rain-kl/openflare/openflared/cmd/flared@latest` 或 `go install github.com/rain-kl/openflare/openflare-agent/cmd/agent@latest` 远程下载并安装最新二进制。
  - **版本依赖统一**：所有组件共享相同的依赖版本，避免了组件间因第三方库版本不一致导致潜在的运行时兼容问题。
* **缺点**：
  - **依赖库大一统**：根目录的 `go.mod` 会包含 Server、Agent、Relay 等所有组件的依赖，但这只影响开发时的依赖下载，对最终编译出的二进制大小和运行效率**没有任何影响**（Go 编译器会自动进行死代码消除/树摇）。
  - **构建配置变动**：Dockerfile 以及 GitHub Actions 需要修改构建上下文，从原本 COPY 子目录改为从根目录统一进行 COPY 和 `go build`。

---

## 3. 具体修改文件清单 (Proposed Changes)
如果采用**方案 B（推荐）**，需要修改的文件清单和逻辑如下：

### 根目录与配置文件
* #### [NEW] [go.mod](file:///Users/ryan/DEV/Go/OpenFlare/go.mod)
  - 职责：全局单一 Go 模块定义，模块名：`github.com/rain-kl/openflare`。
* #### [DELETE] `openflare-server/go.mod` / `go.sum`
* #### [DELETE] `openflare-relay/go.mod` / `go.sum`
* #### [DELETE] `openflare-agent/go.mod` / `go.sum`
* #### [DELETE] `openflared/go.mod` / `go.sum`

### 源代码文件 (约 252 个 Go 文件)
* #### [MODIFY] `openflare-server/**/*.go`
  - 职责：将 `import "openflare/..."` 替换为 `import "github.com/rain-kl/openflare/openflare-server/..."`。
* #### [MODIFY] `openflare-relay/**/*.go`
  - 职责：将 `import "openflare-relay/..."` 替换为 `import "github.com/rain-kl/openflare/openflare-relay/..."`，将 `import "openflare/..."` 替换为 `import "github.com/rain-kl/openflare/openflare-server/..."`。
* #### [MODIFY] `openflare-agent/**/*.go`
  - 职责：将 `import "openflare-agent/..."` 替换为 `import "github.com/rain-kl/openflare/openflare-agent/..."`，将 `import "openflare/..."` 替换为 `import "github.com/rain-kl/openflare/openflare-server/..."`。
* #### [MODIFY] `openflared/**/*.go`
  - 职责：将 `import "openflare-flared/..."` 替换为 `import "github.com/rain-kl/openflare/openflared/..."`，将 `import "openflare/..."` 替换为 `import "github.com/rain-kl/openflare/openflare-server/..."`。

### Dockerfile & Workflows

如果采用**方案 B（推荐）**，我们将继续保持每个组件（Server、Agent、Relay、Flared）编译并产生自己独立的 Docker 镜像（共 4 个镜像），但其 Dockerfile 的构建上下文（Build Context）统一提升至仓库根目录。具体调整细节如下：

* #### [MODIFY] [openflare-server/Dockerfile](file:///Users/ryan/DEV/Go/OpenFlare/openflare-server/Dockerfile)
  - 职责：由于 `openflare-server` 中没有独立的 `go.mod`，构建上下文必须在**仓库根目录**执行。
  - 修改内容：
    ```dockerfile
    # 更改 go-builder 阶段的 COPY 方式：
    COPY go.mod go.sum ./
    RUN go mod download
    COPY openflare-server/ ./openflare-server/
    # go build 指定编译子包：
    RUN go build -trimpath -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflare-server/common.Version=$VERSION'" -o openflare ./openflare-server
    ```

* #### [MODIFY] [openflare-relay/Dockerfile](file:///Users/ryan/DEV/Go/OpenFlare/openflare-relay/Dockerfile)
  - 职责：适配单 go.mod 构建上下文。
  - 修改内容：
    ```dockerfile
    # 更改 builder 阶段的 COPY 方式：
    COPY go.mod go.sum ./
    RUN go mod download
    COPY openflare-server/ ./openflare-server/
    COPY openflare-relay/ ./openflare-relay/
    # go build 指定编译子包：
    RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflare-relay/internal/config.Version=$VERSION'" -o openflare-relay ./openflare-relay/cmd/relay
    ```

* #### [MODIFY] [openflare-agent/Dockerfile](file:///Users/ryan/DEV/Go/OpenFlare/openflare-agent/Dockerfile)
  - 职责：适配单 go.mod 构建上下文。
  - 修改内容：
    ```dockerfile
    # 更改 builder 阶段的 COPY 方式：
    COPY go.mod go.sum ./
    RUN go mod download
    COPY openflare-server/ ./openflare-server/
    COPY openflare-agent/ ./openflare-agent/
    # go build 指定编译子包：
    RUN go build -trimpath -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflare-agent/internal/config.Version=$VERSION'" -o /build/openflare-agent ./openflare-agent/cmd/agent
    ```

* #### [MODIFY] [openflared/Dockerfile](file:///Users/ryan/DEV/Go/OpenFlare/openflared/Dockerfile)
  - 职责：适配单 go.mod 构建上下文。
  - 修改内容：
    ```dockerfile
    # 更改 builder 阶段的 COPY 方式：
    COPY go.mod go.sum ./
    RUN go mod download
    COPY openflare-server/ ./openflare-server/
    COPY openflared/ ./openflared/
    # go build 指定编译子包：
    RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-s -w -X 'github.com/rain-kl/openflare/openflared/internal/config.Version=$VERSION'" -o flared ./openflared/cmd/flared
    ```

* #### [MODIFY] [.github/workflows/release.yml](file:///Users/ryan/DEV/Go/OpenFlare/.github/workflows/release.yml)
  - 职责：更新 go build 构建命令及 ldflags 版本注入参数（例如将 `-ldflags "-X 'openflare/common.Version=$VERSION'"` 替换为 `-ldflags "-X 'github.com/rain-kl/openflare/openflare-server/common.Version=$VERSION'"`，同时编译命令需要指向正确的子包目录，如 `./openflare-server`）。

---

## 4. 验证计划 (Verification Plan)

### 编译与运行测试
* 运行单测以确保各包逻辑正常：
  `go test ./...`（在根目录执行）
* 本地编译各个二进制：
  `go build -o bin/openflare-server ./openflare-server`
  `go build -o bin/openflare-agent ./openflare-agent/cmd/agent`
  `go build -o bin/openflare-relay ./openflare-relay/cmd/relay`
  `go build -o bin/openflared ./openflared/cmd/flared`
* 启动服务并检查版本输出：
  `./bin/openflare-server --version`

### Docker 构建验证
* 验证镜像构建命令：
  `docker build -t openflare-server -f openflare-server/Dockerfile .`
  `docker build -t openflare-agent -f openflare-agent/Dockerfile .`
  `docker build -t openflare-relay -f openflare-relay/Dockerfile .`
  `docker build -t openflared -f openflared/Dockerfile .`
