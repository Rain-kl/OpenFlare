# Wavelet 异步任务代码示例

这些示例用于新增或修改 Wavelet Asynq 任务时快速套用。复制前先对照当前代码，因为任务框架可能随项目演进。

## 任务元数据与常量定义

在对应的业务包 `internal/apps/<module>/tasks.go` 中定义 Asynq task type、Admin task type 和 `TaskMeta`。

```go
package upload

import (
    "github.com/Rain-kl/Wavelet/internal/infra/task"
)

// 异步任务类型标识。格式建议为 "{module}:{action}"。
const CleanupUnusedUploadsTask = "upload:cleanup_unused"

// 管理员可下发的任务类型标识。用于 Admin API 的 task_type。
const TaskTypeCleanupUploads = "cleanup_unused_uploads"

// CleanupUnusedUploadsMeta 任务元数据
var CleanupUnusedUploadsMeta = task.TaskMeta{
    Type:         TaskTypeCleanupUploads,
    AsynqTask:    CleanupUnusedUploadsTask,
    Name:         "清理未使用上传",
    Description:  "清理超过1小时未使用的上传文件",
    SupportsTime: false,
    MaxRetry:     task.DefaultMaxRetry,
    Queue:        task.QueueDefault,
    Retryable:    true,
}
```

带参数任务把前端表单元数据放在 `Params`。`Name` 必须和 payload JSON tag 对齐。

```go
{
    Type:         TaskTypeSendEmail,
    AsynqTask:    SendEmailTask,
    Name:         "发送邮件",
    Description:  "异步发送系统邮件",
    SupportsTime: false,
    MaxRetry:     defaultMaxRetry,
    Queue:        QueueDefault,
    Retryable:    true,
    Params: []TaskParam{
        {
            Name:        "to",
            Label:       "接收邮箱 (To)",
            Type:        "string",
            Required:    true,
            Placeholder: "receiver@example.com",
            Description: "接收邮件的目标邮箱地址",
        },
        {
            Name:        "subject",
            Label:       "邮件主题 (Subject)",
            Type:        "string",
            Required:    true,
            Placeholder: "请输入邮件主题",
            Description: "发送邮件的主题标题",
        },
        {
            Name:        "body",
            Label:       "邮件内容 (Body)",
            Type:        "text",
            Required:    true,
            Placeholder: "请输入邮件内容",
            Description: "发送邮件的内容主体",
        },
    },
}
```

## 无参数 Handler

放在对应业务模块，例如 `internal/apps/upload/tasks.go`。

```go
package upload

import (
    "context"

    "github.com/Rain-kl/Wavelet/internal/infra/task"
)

type CleanupUnusedUploadsHandler struct{}

func (h *CleanupUnusedUploadsHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
    task.AppendLog(ctx, "开始扫描未使用上传")

    // 调用 model/service 完成业务逻辑。
    // 批量处理时按批次记录日志，不要每条记录都 AppendLog。

    msg := "清理完成"
    task.AppendLog(ctx, "%s", msg)
    return &task.TaskResult{Message: msg}, nil
}
```

## 带参数 Handler

实现 `PayloadValidator` 做 Admin 下发时的服务端校验和标准化。`Execute` 仍然解析 payload，因为 Scheduler 和 Retry 不一定经过 Admin 校验路径。

```go
package user

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "strings"

    "github.com/Rain-kl/Wavelet/internal/infra/task"
)

type SendEmailPayload struct {
    To      string `json:"to"`
    Subject string `json:"subject"`
    Body    string `json:"body"`
}

type SendEmailHandler struct{}

func (h *SendEmailHandler) ValidatePayload(payload []byte) ([]byte, error) {
    if len(payload) == 0 {
        return nil, errors.New("任务参数不能为空")
    }

    var req SendEmailPayload
    if err := json.Unmarshal(payload, &req); err != nil {
        return nil, fmt.Errorf("无效的 JSON 格式: %w", err)
    }

    req.To = strings.TrimSpace(req.To)
    req.Subject = strings.TrimSpace(req.Subject)
    req.Body = strings.TrimSpace(req.Body)
    if req.To == "" || req.Subject == "" || req.Body == "" {
        return nil, errors.New("to、subject、body 不能为空")
    }

    return json.Marshal(req)
}

func (h *SendEmailHandler) Execute(ctx context.Context, payload []byte) (*task.TaskResult, error) {
    var req SendEmailPayload
    if err := json.Unmarshal(payload, &req); err != nil {
        return nil, fmt.Errorf("解析任务参数: %w", err)
    }

    task.AppendLog(ctx, "开始发送邮件到: %s", req.To)

    // 调用业务服务发送邮件。

    msg := fmt.Sprintf("邮件成功发送至: %s", req.To)
    task.AppendLog(ctx, "%s", msg)
    return &task.TaskResult{Message: msg}, nil
}
```

## 统一注册

在 `internal/infra/task/handlers/register.go` 注册。Admin dispatch 的 `ValidateAndNormalizePayload` 和 Worker 执行都依赖这里。

```go
package handlers

import (
    "github.com/Rain-kl/Wavelet/internal/apps/upload"
    "github.com/Rain-kl/Wavelet/internal/apps/user"
    "github.com/Rain-kl/Wavelet/internal/infra/task"
)

func Register() {
    task.RegisterHandler(task.CleanupUnusedUploadsTask, &upload.CleanupUnusedUploadsHandler{})
    task.RegisterHandler(task.SendEmailTask, &user.SendEmailHandler{})
}
```

## Cron 调度和配置

系统默认的定时任务必须通过 Goose SQL 迁移初始化插入到 `schedules` 表。

在 `internal/infra/persistence/migrator/goose/postgres` 下的示例：

```sql
-- +goose Up
INSERT INTO schedules (id, name, task_type, cron, payload, is_active, created_at, updated_at)
VALUES (1, '清理未使用上传', 'cleanup_unused_uploads', '0 */2 * * *', '{}', TRUE, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
ON CONFLICT (id) DO NOTHING;

-- +goose Down
-- 根据业务需求决定是否需要在此删除
```

对于 `sqlite` 也可以使用类似的 `INSERT INTO ... ON CONFLICT(id) DO NOTHING` 语法。数据库更新后，后端会自动热重载调度器。

## Handler 测试

带参数任务至少覆盖合法 payload、空 payload、非法 JSON、缺失必填和标准化。

```go
func TestSendEmailHandlerValidatePayload(t *testing.T) {
    tests := []struct {
        name    string
        payload []byte
        want    SendEmailPayload
        wantErr bool
    }{
        {
            name:    "valid payload is normalized",
            payload: []byte(`{"to":" user@example.com ","subject":" hi ","body":" body "}`),
            want: SendEmailPayload{
                To:      "user@example.com",
                Subject: "hi",
                Body:    "body",
            },
        },
        {
            name:    "empty payload",
            payload: nil,
            wantErr: true,
        },
        {
            name:    "invalid json",
            payload: []byte(`{`),
            wantErr: true,
        },
        {
            name:    "missing required field",
            payload: []byte(`{"to":"user@example.com","subject":"","body":"body"}`),
            wantErr: true,
        },
    }

    h := &SendEmailHandler{}
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            gotPayload, err := h.ValidatePayload(tt.payload)
            if gotErr := err != nil; gotErr != tt.wantErr {
                t.Fatalf("ValidatePayload(%s) error = %v, want error presence = %t", tt.payload, err, tt.wantErr)
            }
            if tt.wantErr {
                return
            }

            var got SendEmailPayload
            if err := json.Unmarshal(gotPayload, &got); err != nil {
                t.Fatalf("json.Unmarshal(%s) error = %v", gotPayload, err)
            }
            if diff := cmp.Diff(tt.want, got); diff != "" {
                t.Errorf("ValidatePayload(%s) mismatch (-want +got):\n%s", tt.payload, diff)
            }
        })
    }
}
```

`Execute` 测试优先验证业务服务调用、错误返回和结果摘要；日志可只验证关键路径，避免把精确日志文本写成脆弱断言。

## Admin Dispatch 测试形状

Admin dispatch 测试关注通用链路是否调用了 `PayloadValidator`，不要为每种任务在 handler 里写 if 分支。

```go
func TestDispatchTaskValidatesPayload(t *testing.T) {
    // 1. 初始化测试 DB 和 task.AsynqClient。
    // 2. 注册测试 handler: task.RegisterHandler(task.SendEmailTask, &user.SendEmailHandler{})
    // 3. POST /api/v1/admin/tasks/dispatch，传入非法 payload。
    // 4. 断言响应为 400，错误信息清晰，且没有创建可执行任务。
}
```

需要 Redis/Asynq 时优先复用项目现有测试模式；没有现成依赖时可用 `miniredis` 初始化 `task.AsynqClient`。不要把 `internal/infra/task` 依赖塞进通用 testhelper 造成 import cycle。
