# Cloudflare 分组域名移动/批量操作与 IP 组查看搜索实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 `/cloudflare/groups/[id]` 页面增加域名单条移动、批量移动与批量移出操作及后端支持，并为 `/ip-groups` 的查看 IP 组弹窗增加 IP 即时搜索过滤功能。

**Architecture:** 后端在 `internal/apps/openflare/cloudflare` 扩展单条移动及批量移动/删除接口并排队更新 DNS 同步；前端在服务层封装调用，在详情页增加多选框、批量工具栏与移动弹窗；IP 组查看弹窗增加输入框实时过滤 entries。

**Tech Stack:** Go 1.25+, Gin, GORM, Next.js App Router, TypeScript, React Query, Tailwind CSS, shadcn/ui, Vitest.

## Global Constraints

- 禁止删除 `frontend/node_modules`。
- 遵循 `AGENTS.md`：API 错误使用 `response.Abort*`，禁止使用 200 返回错误体。
- 业务路由仅在 `internal/router/v1/openflare/register_cloudflare.go` 挂载。
- i18n 变更修改 `frontend/messages/fragments`，通过 `node scripts/merge-i18n-fragments.mjs` 合并，禁止直接修改 `zh-CN.json` / `en.json`。
- 完成后必须执行 `make swagger` 与 `make code-check`。

---

### Task 1: 后端 DTO、业务逻辑及单测 (Backend Logic & Unit Tests)

**Files:**
- Modify: `internal/apps/openflare/cloudflare/types.go`
- Modify: `internal/apps/openflare/cloudflare/errs.go`
- Modify: `internal/apps/openflare/cloudflare/logics.go`
- Modify: `internal/apps/openflare/cloudflare/logics_test.go`

**Interfaces:**
- Produces:
  - `MemberMoveInput { TargetGroupID uint `json:"target_group_id"` }`
  - `MemberBatchMoveInput { MemberIDs []uint `json:"member_ids"`, TargetGroupID uint `json:"target_group_id"` }`
  - `MemberBatchRemoveInput { MemberIDs []uint `json:"member_ids"` }`
  - `MoveMember(ctx context.Context, sourceGroupID, memberID, targetGroupID uint) (*MemberItem, error)`
  - `BatchMoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchMoveInput) error`
  - `BatchRemoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchRemoveInput) error`

- [ ] **Step 1: 在 types.go 和 errs.go 中新增结构体与错误常量**

在 `internal/apps/openflare/cloudflare/types.go` 增加：
```go
// MemberMoveInput contains the target group ID for moving a member.
type MemberMoveInput struct {
	TargetGroupID uint `json:"target_group_id"`
}

// MemberBatchMoveInput contains the member IDs and target group ID for batch moving.
type MemberBatchMoveInput struct {
	MemberIDs     []uint `json:"member_ids"`
	TargetGroupID uint   `json:"target_group_id"`
}

// MemberBatchRemoveInput contains the member IDs for batch deletion.
type MemberBatchRemoveInput struct {
	MemberIDs []uint `json:"member_ids"`
}
```

在 `internal/apps/openflare/cloudflare/errs.go` 增加：
```go
	errTargetGroupSame    = "目标分组不能为当前分组"
	errTargetGroupInvalid = "目标分组不存在"
	errNoMembersSelected  = "未选择任何成员"
```

- [ ] **Step 2: 编写失败的单元测试**

在 `internal/apps/openflare/cloudflare/logics_test.go` 中新增测试函数 `TestMoveMemberAndBatchOperations(t *testing.T)`，测试：
1. 目标分组等于源分组时报错 `errTargetGroupSame`；
2. 目标分组不存在时报错；
3. 成功移动成员到目标分组，更新 `GroupID` 并将 `SyncStatus` 置为 `pending`；
4. 批量移动多个成员到目标分组；
5. 批量删除成员。

- [ ] **Step 3: 运行测试验证失败**

运行：`go test -v ./internal/apps/openflare/cloudflare -run TestMoveMemberAndBatchOperations`
预期：编译错误或未实现失败。

- [ ] **Step 4: 实现 MoveMember, BatchMoveMembers, BatchRemoveMembers**

在 `internal/apps/openflare/cloudflare/logics.go` 实现：
```go
// MoveMember transfers a member from sourceGroupID to targetGroupID.
func MoveMember(ctx context.Context, sourceGroupID, memberID, targetGroupID uint) (*MemberItem, error) {
	if targetGroupID == 0 || targetGroupID == sourceGroupID {
		return nil, errors.New(errTargetGroupSame)
	}
	targetGroup, err := repository.GetCFPointingGroup(ctx, targetGroupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errTargetGroupInvalid)
		}
		return nil, err
	}
	member, err := repository.GetCFPointingMember(ctx, sourceGroupID, memberID)
	if err != nil {
		return nil, err
	}
	member.GroupID = targetGroupID
	member.SyncStatus = model.CFMemberSyncPending
	member.LastError = ""
	if err = repository.SaveCFPointingMember(ctx, member); err != nil {
		return nil, err
	}
	if targetGroup.Enabled {
		if _, err = DispatchMemberSync(ctx, member.ID, "cloudflare_member_move"); err != nil {
			logger.WarnF(ctx, "[Cloudflare] dispatch move sync failed: member_id=%d error=%v", member.ID, err)
		}
	}
	domain, err := repository.GetZoneDomainByID(ctx, member.ZoneDomainID)
	if err != nil {
		return nil, err
	}
	return memberItem(member, domain), nil
}

// BatchMoveMembers transfers multiple members from sourceGroupID to targetGroupID.
func BatchMoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchMoveInput) error {
	if len(input.MemberIDs) == 0 {
		return errors.New(errNoMembersSelected)
	}
	if input.TargetGroupID == 0 || input.TargetGroupID == sourceGroupID {
		return errors.New(errTargetGroupSame)
	}
	targetGroup, err := repository.GetCFPointingGroup(ctx, input.TargetGroupID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New(errTargetGroupInvalid)
		}
		return nil, err
	}
	for _, memberID := range input.MemberIDs {
		member, getErr := repository.GetCFPointingMember(ctx, sourceGroupID, memberID)
		if getErr != nil {
			continue
		}
		member.GroupID = input.TargetGroupID
		member.SyncStatus = model.CFMemberSyncPending
		member.LastError = ""
		if saveErr := repository.SaveCFPointingMember(ctx, member); saveErr != nil {
			logger.ErrorF(ctx, "[Cloudflare] batch move save member failed: member_id=%d error=%v", memberID, saveErr)
			continue
		}
		if targetGroup.Enabled {
			if _, syncErr := DispatchMemberSync(ctx, member.ID, "cloudflare_member_move"); syncErr != nil {
				logger.WarnF(ctx, "[Cloudflare] dispatch batch move sync failed: member_id=%d error=%v", member.ID, syncErr)
			}
		}
	}
	return nil
}

// BatchRemoveMembers deletes multiple members and their remote A records.
func BatchRemoveMembers(ctx context.Context, sourceGroupID uint, input MemberBatchRemoveInput) error {
	if len(input.MemberIDs) == 0 {
		return errors.New(errNoMembersSelected)
	}
	for _, memberID := range input.MemberIDs {
		member, err := repository.GetCFPointingMember(ctx, sourceGroupID, memberID)
		if err != nil {
			continue
		}
		if delErr := DeleteManagedRecord(ctx, member.ID); delErr != nil {
			logger.WarnF(ctx, "[Cloudflare] delete remote record failed during batch remove: member_id=%d error=%v", member.ID, delErr)
		}
		if err = repository.DeleteCFPointingMember(ctx, member); err != nil {
			logger.ErrorF(ctx, "[Cloudflare] delete member failed during batch remove: member_id=%d error=%v", member.ID, err)
		}
	}
	return nil
}
```

- [ ] **Step 5: 运行测试验证通过**

运行：`go test -v ./internal/apps/openflare/cloudflare -run TestMoveMemberAndBatchOperations`
预期：PASS。

- [ ] **Step 6: Commit**

```bash
git add internal/apps/openflare/cloudflare/
git commit -m "feat(cloudflare): add move and batch operation logics with tests"
```

---

### Task 2: 后端 Handler、路由注册与 Swagger (Backend Handlers & Routing)

**Files:**
- Modify: `internal/apps/openflare/cloudflare/routers.go`
- Modify: `internal/apps/openflare/cloudflare/routers_test.go`
- Modify: `internal/router/v1/openflare/register_cloudflare.go`

**Interfaces:**
- Produces:
  - `MoveMemberHandler(c *gin.Context)`
  - `BatchMoveMembersHandler(c *gin.Context)`
  - `BatchRemoveMembersHandler(c *gin.Context)`
  - 路由：`POST /groups/:id/members/:memberId/move`
  - 路由：`POST /groups/:id/members/batch-move`
  - 路由：`POST /groups/:id/members/batch-remove`

- [ ] **Step 1: 在 routers.go 中实现 Handler 并补充 Swagger 注解**

```go
// MoveMemberHandler moves a member to a target group.
// @Summary 移动 Cloudflare 指向成员到其他分组
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "原分组 ID"
// @Param memberId path int true "成员 ID"
// @Param body body cloudflare.MemberMoveInput true "目标分组参数"
// @Success 200 {object} response.Any{data=cloudflare.MemberItem}
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/members/{memberId}/move [post]
func MoveMemberHandler(c *gin.Context) {
	groupID, memberID, ok := memberParams(c)
	if !ok {
		return
	}
	var input MemberMoveInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	item, err := MoveMember(c.Request.Context(), groupID, memberID, input.TargetGroupID)
	if abortLogic(c, err) {
		return
	}
	c.JSON(http.StatusOK, response.OK(item))
}

// BatchMoveMembersHandler moves multiple members to a target group.
// @Summary 批量移动 Cloudflare 指向成员
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "原分组 ID"
// @Param body body cloudflare.MemberBatchMoveInput true "批量移动参数"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Failure 404 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/members/batch-move [post]
func BatchMoveMembersHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input MemberBatchMoveInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	if abortLogic(c, BatchMoveMembers(c.Request.Context(), id, input)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}

// BatchRemoveMembersHandler removes multiple members.
// @Summary 批量移出 Cloudflare 指向成员
// @Tags openflare-cloudflare
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param id path int true "分组 ID"
// @Param body body cloudflare.MemberBatchRemoveInput true "批量移出参数"
// @Success 200 {object} response.Any
// @Failure 400 {object} response.Any
// @Router /api/v1/d/cloudflare/groups/{id}/members/batch-remove [post]
func BatchRemoveMembersHandler(c *gin.Context) {
	id, ok := apiutil.IDParam(c)
	if !ok {
		return
	}
	var input MemberBatchRemoveInput
	if !apiutil.BindJSON(c, &input) {
		return
	}
	if abortLogic(c, BatchRemoveMembers(c.Request.Context(), id, input)) {
		return
	}
	c.JSON(http.StatusOK, response.OKNil())
}
```

- [ ] **Step 2: 在 register_cloudflare.go 中挂载路由**

在 `internal/router/v1/openflare/register_cloudflare.go` 添加：
```go
	route.POST("/groups/:id/members/:memberId/move", cf.MoveMemberHandler)
	route.POST("/groups/:id/members/batch-move", cf.BatchMoveMembersHandler)
	route.POST("/groups/:id/members/batch-remove", cf.BatchRemoveMembersHandler)
```

- [ ] **Step 3: 在 routers_test.go 中补充接口级单测**

为新 Handler 添加单元测试，验证请求解析与响应状态码。
运行：`go test -v ./internal/apps/openflare/cloudflare -run TestRouters`
预期：PASS。

- [ ] **Step 4: Commit**

```bash
git add internal/apps/openflare/cloudflare/routers.go internal/apps/openflare/cloudflare/routers_test.go internal/router/v1/openflare/register_cloudflare.go
git commit -m "feat(cloudflare): register move and batch member API routes"
```

---

### Task 3: WAF IP 组「查看 IP 组」弹窗搜索功能 (IP Group View Dialog Search)

**Files:**
- Modify: `frontend/messages/fragments/security.zh-CN.json`
- Modify: `frontend/messages/fragments/security.en.json`
- Modify: `frontend/app/(main)/waf/components/ip-group-view-dialog.tsx`

**Interfaces:**
- Produces:
  - 搜索框 Input，支持清空按钮
  - `filteredEntries` 动态过滤
  - 数量摘要显示，如 `共 10 条 IP（匹配 3 条）`
  - 过滤无匹配时的独立空状态提示

- [ ] **Step 1: 在 security.zh-CN.json 与 security.en.json 中补充文案**

在 `frontend/messages/fragments/security.zh-CN.json` 中的 `ipGroups.viewDialog` 增加：
```json
      "searchPlaceholder": "搜索 IP 地址...",
      "summaryFiltered": "{type} · 共 {count} 条 IP（匹配 {matched} 条）",
      "noSearchResult": "未找到匹配的 IP 地址。"
```
并在 `security.en.json` 对应增加英文文案。
执行 `node scripts/merge-i18n-fragments.mjs` 同步生成全量文件。

- [ ] **Step 2: 修改 ip-group-view-dialog.tsx**

增加 `searchKeyword` state：
- 渲染包含 `Search` 图标和清空按钮的 Input；
- `filteredEntries`：使用 `entries.filter(e => e.ip.toLowerCase().includes(keyword.trim().toLowerCase()))`；
- 弹窗关闭时自动清空 `searchKeyword`；
- 当 `entries.length > 0` 且 `filteredEntries.length === 0` 时展示 `EmptyStateWithBorder`（使用 `noSearchResult`）；
- DialogDescription 中根据是否有过滤展示 `summaryFiltered` 或 `summary`。

- [ ] **Step 3: 运行 TypeScript 检查**

运行：`cd frontend && pnpm tsc --noEmit`
预期：无类型错误。

- [ ] **Step 4: Commit**

```bash
git add frontend/messages/ frontend/app/\(main\)/waf/components/ip-group-view-dialog.tsx
git commit -m "feat(waf): add search and filter in ip group view dialog"
```

---

### Task 4: 前端 Cloudflare 分组单条移动与批量操作 (Frontend Move & Batch Operations)

**Files:**
- Modify: `frontend/messages/fragments/cloudflare.zh-CN.json`
- Modify: `frontend/messages/fragments/cloudflare.en.json`
- Modify: `frontend/lib/services/openflare/cloudflare.service.ts`
- Create: `frontend/app/(main)/cloudflare/components/member-move-dialog.tsx`
- Modify: `frontend/app/(main)/cloudflare/groups/[id]/page-client.tsx`
- Modify: `frontend/tests/cloudflare/cloudflare-group-detail.test.tsx`

**Interfaces:**
- Produces:
  - `CloudflareService.moveMember(groupId, memberId, targetGroupId)`
  - `CloudflareService.batchMoveMembers(groupId, memberIds, targetGroupId)`
  - `CloudflareService.batchRemoveMembers(groupId, memberIds)`
  - `<MemberMoveDialog />` 组件
  - 多选框 Checkbox、全选/半选/清空、批量操作工具条

- [ ] **Step 1: 在 cloudflare.zh-CN.json 与 cloudflare.en.json 增加国际化文案**

增加：
- `move`: "移动"
- `batchMove`: "批量移动"
- `batchRemove`: "批量删除"
- `selectedCount`: "已选择 {count} 项"
- `clearSelection`: "取消选择"
- `moveDialog`:
  - `titleSingle`: "移动域名「{domain}」"
  - `titleBatch`: "批量移动域名（共 {count} 项）"
  - `targetGroupLabel`: "目标指向分组"
  - `targetGroupPlaceholder`: "请选择目标分组"
  - `noOtherGroups`: "没有其他可用的指向分组，请先创建分组"
  - `submit`: "确认移动"
  - `moving`: "移动中..."
- `batchRemoveDialog`:
  - `title`: "确认批量删除域名"
  - `desc`: "确认从当前分组移出已选中的 {count} 个域名吗？这将同时清理 Cloudflare 上的解析记录。"
  - `confirm`: "确认删除"
  - `removing`: "删除中..."
- `memberMoved`: "域名移动成功"
- `batchMoved`: "成功移动 {count} 个域名"
- `batchRemoved`: "成功删除 {count} 个域名"

执行 `node scripts/merge-i18n-fragments.mjs` 同步。

- [ ] **Step 2: 在 cloudflare.service.ts 扩展 API 方法**

```typescript
  static async moveMember(
    groupId: number,
    memberId: number,
    targetGroupId: number,
  ): Promise<CloudflareMember> {
    return this.post<CloudflareMember>(
      `/cloudflare/groups/${groupId}/members/${memberId}/move`,
      { target_group_id: targetGroupId },
    );
  }

  static async batchMoveMembers(
    groupId: number,
    memberIds: number[],
    targetGroupId: number,
  ): Promise<void> {
    return this.post<void>(
      `/cloudflare/groups/${groupId}/members/batch-move`,
      { member_ids: memberIds, target_group_id: targetGroupId },
    );
  }

  static async batchRemoveMembers(
    groupId: number,
    memberIds: number[],
  ): Promise<void> {
    return this.post<void>(
      `/cloudflare/groups/${groupId}/members/batch-remove`,
      { member_ids: memberIds },
    );
  }
```

- [ ] **Step 3: 创建 MemberMoveDialog 组件**

在 `frontend/app/(main)/cloudflare/components/member-move-dialog.tsx`：
- Props: `open`, `onOpenChange`, `members: CloudflareMember[]`, `groups: CloudflareGroup[]`, `currentGroupId: number`, `pending: boolean`, `onSubmit: (targetGroupId: number) => void`
- 过滤 `availableGroups = groups.filter(g => g.id !== currentGroupId)`
- 下拉选择 Select 组件展示分组名称与主节点 IP
- 提交按钮

- [ ] **Step 4: 修改 CloudflareGroupDetailPageClient 实现多选与批量/单条操作**

在 `page-client.tsx`：
- 使用 React Query 获取全部分组列表 `CloudflareService.listGroups()`；
- 状态管理：
  - `selectedMemberIDs: Set<number>`
  - `moveDialogOpen: boolean`
  - `movingMembers: CloudflareMember[]`
  - `batchRemoveOpen: boolean`
- 多选逻辑：
  - 表头 Checkbox 绑定当前页成员选中状态（`allSelected`, `indeterminate`）；
  - 每行首列增加 Checkbox；
- 批量工具栏：
  - 在卡片内部当 `selectedMemberIDs.size > 0` 时展示浮动/嵌入的操作栏：包含数量、`批量移动`、`批量删除`、`取消选择`；
- 行操作区：
  - 在原有 `同步`、`删除` 旁新增 `移动` 按钮（FolderInput 或 ArrowRightLeft 图标），点击打开 `MemberMoveDialog`（传入单条成员）；
- Mutations：
  - `moveMutation`：支持单条和批量移动，完成后清空 `selectedMemberIDs` 并刷新 query；
  - `batchRemoveMutation`：批量移出，完成后清空并刷新。

- [ ] **Step 5: 更新前端测试**

在 `frontend/tests/cloudflare/cloudflare-group-detail.test.tsx` 增加测试用例覆盖移动与批量删除触发逻辑。

- [ ] **Step 6: Commit**

```bash
git add frontend/
git commit -m "feat(cloudflare): add domain move and batch operations in group detail page"
```

---

### Task 5: 质量门禁、Swagger 与更新日志 (Quality Gates & Verification)

**Files:**
- Modify: `docs/changelog/index.md`
- Generated: `internal/router/root/swagger/` (via make swagger)

- [ ] **Step 1: 生成 Swagger API 文档**

运行：`make swagger`
检查生成的 swagger 规范包含新端点。

- [ ] **Step 2: 执行全量代码检查**

运行：`make code-check`
确保 `golangci-lint`、`tsc` 与 `eslint` 均零警告零错误通过。

- [ ] **Step 3: 更新 docs/changelog/index.md**

在 `[Unreleased]` 下添加新增功能说明：
```markdown
### 新增
- Cloudflare 指向分组：支持将已添加的域名在不同分组之间移动，自动排队更新远程 DNS 指向。
- Cloudflare 指向分组：详情页支持批量勾选域名进行批量移动与批量移出操作。
- WAF IP 组：在查看 IP 组弹窗中新增即时搜索功能，支持快速过滤和定位 IP 地址。
```

- [ ] **Step 4: 提交变更**

```bash
git add docs/changelog/index.md internal/router/root/swagger/
git commit -m "docs(changelog): document cloudflare member move and ip group search"
```
