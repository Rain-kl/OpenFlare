# Web 服务器：Skill 的综合应用

本示例展示 Go skill 如何在真实的 HTTP 服务器中协同应用。每个部分
引用相关的 skill 以获取详细指导。

## 结构

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log/slog"
    "net/http"
    "os"
    "os/signal"
    "time"
)

// --- 接口（go-interfaces） ---

// Store 定义了数据访问边界。定义在消费方包中，
// 而非实现方包中。
type Store interface {
    GetUser(ctx context.Context, id string) (*User, error)
}

// --- 类型与构造函数（go-naming、go-declarations） ---

// Server 处理用户 API 的 HTTP 请求。
type Server struct {
    store  Store
    router *http.ServeMux
}

// NewServer 使用给定的依赖创建 Server。
// 调用者必须调用 Shutdown 来释放资源。
func NewServer(store Store) *Server {
    s := &Server{store: store}
    s.router = http.NewServeMux()
    s.router.HandleFunc("GET /users/{id}", s.handleGetUser)
    return s
}

// --- 错误处理（go-error-handling） ---

// 领域错误作为哨兵 —— 使用 errors.Is 进行检查。
var ErrNotFound = errors.New("not found")

// --- HTTP 处理器（go-control-flow、go-context、go-error-handling） ---

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()  // go-context：从 request 派生
    id := r.PathValue("id")

    user, err := s.store.GetUser(ctx, id)
    if err != nil {
        if errors.Is(err, ErrNotFound) {  // go-error-handling：errors.Is
            http.Error(w, "user not found", http.StatusNotFound)
            return  // go-control-flow：提前返回
        }
        // HTTP 处理器是"记录或返回"规则的例外：在服务端记录详细信息，向客户端返回脱敏错误。
        slog.Error("GetUser failed", "id", id, "err", err)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

// --- 优雅关闭（go-concurrency、go-defensive） ---

func main() {
    store := NewDBStore(os.Getenv("DATABASE_URL"))
    srv := NewServer(store)

    httpSrv := &http.Server{
        Addr:         ":8080",
        Handler:      srv.router,
        ReadTimeout:  5 * time.Second,   // go-defensive：使用 time.Duration
        WriteTimeout: 10 * time.Second,
    }

    // go-concurrency：goroutine 生命周期清晰
    go func() {
        sigCh := make(chan os.Signal, 1)  // go-concurrency：channel 大小为 1
        signal.Notify(sigCh, os.Interrupt)
        <-sigCh

        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()  // go-defensive：defer 清理
        httpSrv.Shutdown(ctx)
    }()

    slog.Info("starting server", "addr", httpSrv.Addr)
    if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
        slog.Error("server error", "err", err)
        os.Exit(1)  // go-packages：仅在 main 中退出
    }
}
```

## 应用的 Skill

| 领域 | Skill | 演示内容 |
|------|-------|----------|
| 接口在消费方 | [go-interfaces](../../go-interfaces/SKILL.md) | `Store` 在使用处定义 |
| 命名 | [go-naming](../../go-naming/SKILL.md) | MixedCaps、接收器缩写、清晰的函数名 |
| 错误处理 | [go-error-handling](../../go-error-handling/SKILL.md) | 哨兵错误、`errors.Is`、记录或返回 |
| Context | [go-context](../../go-context/SKILL.md) | 从 request 派生，逐层传递 |
| 控制流 | [go-control-flow](../../go-control-flow/SKILL.md) | 错误情况的提前返回 |
| 并发 | [go-concurrency](../../go-concurrency/SKILL.md) | 清晰的 goroutine 生命周期、channel 大小 |
| 防御性 | [go-defensive](../../go-defensive/SKILL.md) | `defer cancel()`、`time.Duration`、优雅关闭 |
| 包管理 | [go-packages](../../go-packages/SKILL.md) | 仅在 `main()` 中退出 |
| 日志 | [go-error-handling](../../go-error-handling/SKILL.md) | 结构化 slog，错误只处理一次 |
