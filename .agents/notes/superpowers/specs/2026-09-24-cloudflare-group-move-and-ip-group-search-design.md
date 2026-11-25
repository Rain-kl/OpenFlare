# Cloudflare 分组域名移动/批量操作与 IP 组查看搜索设计文档

- **日期**：2026-09-24
- **状态**：已批准 (Approved)

---

## 1. 目标与背景 (Goals & Context)

1. **Cloudflare 分组域名管理增强**：
   - 在 `/cloudflare/groups/[id]` 页面，为单个成员域名增加「移动」操作，支持迁移至其他可用分组；
   - 增加表格行复选框多选机制与批量操作栏，支持「批量删除」与「批量移动」；
   - 确保域名移动后，如果目标分组处于启用状态，自动重新排队向 Cloudflare 同步目标节点的新 IP。
2. **WAF IP 组查看弹窗增强**：
   - 在 `/ip-groups` 的「查看 IP 组」弹窗（`IPGroupViewDialog`）内，新增即时 IP 搜索与过滤功能，方便用户在包含大量 IP 条目时快速查找与定位。

---

## 2. 后端接口设计 (Backend API)

### 2.1 数据模型与 DTO

文件：`internal/apps/openflare/cloudflare/types.go`

```go
type MemberMoveInput struct {
    TargetGroupID uint `json:"target_group_id"`
}

type MemberBatchMoveInput struct {
    MemberIDs     []uint `json:"member_ids"`
    TargetGroupID uint   `json:"target_group_id"`
}

type MemberBatchRemoveInput struct {
    MemberIDs []uint `json:"member_ids"`
}
```

### 2.2 路由端点

文件：`internal/apps/openflare/cloudflare/routers.go`
注册：`internal/router/v1/openflare/register_cloudflare.go`

1. **单条移动**：
   - 路径：`POST /api/v1/d/cloudflare/groups/:id/members/:memberId/move`
   - 请求体：`MemberMoveInput`
   - 响应：`200 OK` + `cloudflare.MemberItem`
2. **批量移动**：
   - 路径：`POST /api/v1/d/cloudflare/groups/:id/members/batch-move`
   - 请求体：`MemberBatchMoveInput`
   - 响应：`200 OK`
3. **批量移出/删除**：
   - 路径：`POST /api/v1/d/cloudflare/groups/:id/members/batch-remove`
   - 请求体：`MemberBatchRemoveInput`
   - 响应：`200 OK`

### 2.3 业务逻辑层

文件：`internal/apps/openflare/cloudflare/logics.go`

- `MoveMember(ctx context.Context, sourceGroupID, memberID, targetGroupID uint) (*MemberItem, error)`
  - 检查 `targetGroupID != sourceGroupID`；
  - 检查目标分组是否存在且非空；
  - 查询原成员，更新 `GroupID = targetGroupID`，将 `SyncStatus` 置为 `pending`，清除 `LastError`，保存到数据库；
  - 若目标分组 `Enabled == true`，调用 `DispatchMemberSync(ctx, member.ID, "cloudflare_member_move")` 排队执行远程 A 记录同步；
  - 返回更新后的 `MemberItem`。
- `BatchMoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchMoveInput) error`
  - 校验目标分组有效性；
  - 针对 `input.MemberIDs` 过滤去重，对每个成员执行分组归属更新并重置同步状态；
  - 若目标分组启用，分发同步任务。
- `BatchRemoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchRemoveInput) error`
  - 针对每个 `memberID`，校验属于 `sourceGroupID`，调用 `DeleteManagedRecord(ctx, member.ID)` 清除 Cloudflare 上的 A 记录，并调用 `repository.DeleteCFPointingMember(ctx, member)`。

---

## 3. 前端交互设计 (Frontend UI & Interaction)

### 3.1 Cloudflare 服务客户端扩展

文件：`frontend/lib/services/openflare/cloudflare.service.ts`

- `moveMember(groupId: number, memberId: number, targetGroupId: number): Promise<CloudflareMember>`
- `batchMoveMembers(groupId: number, memberIds: number[], targetGroupId: number): Promise<void>`
- `batchRemoveMembers(groupId: number, memberIds: number[]): Promise<void>`

### 3.2 域名成员列表交互 (`frontend/app/(main)/cloudflare/groups/[id]/page-client.tsx`)

1. **多选管理**：
   - `selectedMemberIDs: Set<number>` 状态；
   - 表头 Checkbox 支持全部选中/取消选中（含半选状态）；
   - 每行首列为 Checkbox，行内可单选。
2. **批量操作栏 (Batch Action Bar)**：
   - 当 `selectedMemberIDs.size > 0` 时，在成员卡片头部或列表上方显示批量操作工具条：
     - 已选条数提示（Badge）；
     - 「批量移动」按钮：唤起移动弹窗，包含选中的所有成员；
     - 「批量删除」按钮：唤起确认弹窗，明确提示删除的数量与域名，确认后执行批量移出；
     - 「取消选择」按钮：清空勾选。
3. **行级「移动」操作**：
   - 行右侧操作按钮区新增「移动」操作按钮；
   - 点击唤起 `MemberMoveDialog` 弹窗。
4. **移动域名弹窗 (`MemberMoveDialog`)**：
   - 提取独立组件或子组件，接收待移动的成员列表和全部 Cloudflare 分组列表；
   - 过滤排除当前分组，用户下拉选择目标分组；
   - 点击确认后执行 `moveMutation` 或 `batchMoveMutation`。

### 3.3 WAF IP 组查看弹窗搜索 (`frontend/app/(main)/waf/components/ip-group-view-dialog.tsx`)

1. **搜索框**：
   - 在 DialogHeader 和 IP 列表表格之间，增加搜索过滤条；
   - 带 `Search` 图标，清除按钮，响应输入并去除首尾空格；
2. **过滤行为**：
   - `searchKeyword` 过滤 `entries` 列表，匹配 `entry.ip`；
   - 更新汇总文案：如 `共 50 条 IP (匹配 3 条)`；
   - 无匹配时展示特定空状态「未找到匹配的 IP 地址」。

---

## 4. 国际化与文案 (i18n)

- `frontend/messages/fragments/cloudflare.zh-CN.json` & `cloudflare.en.json`：
  - 增加移动相关文案：`move`, `batchMove`, `batchRemove`, `moveTitle`, `targetGroup`, `selectTargetGroup`, `batchRemoveConfirm`, 等。
- `frontend/messages/fragments/security.zh-CN.json` & `security.en.json`：
  - 增加 `viewDialog` 搜索框文案：`searchPlaceholder`, `matchedCount`, `noMatchFilter`。

---

## 5. 验证标准 (Verification Criteria)

1. **后端单测**：
   - 针对 `MoveMember`、`BatchMoveMembers`、`BatchRemoveMembers` 编写完整单元测试，验证成功与异常分支（如移到同一分组报错、不存在的分组报错等）；
   - 执行 `go test ./internal/apps/openflare/cloudflare/...` 全部通过。
2. **前端测试与校验**：
   - 更新或新增前端测试用例覆盖移动与批量删除逻辑；
   - 执行 `make code-check` 保证 TypeScript、ESLint 与后端架构规则通过。
3. **API 文档与更新日志**：
   - 执行 `make swagger` 同步 OpenAPI 文档；
   - 更新 `docs/changelog/index.md`。
