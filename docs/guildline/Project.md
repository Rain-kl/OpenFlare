# OpenFlare 特定项目开发准则 (Project Guidelines)

本文档定义了针对 **OpenFlare** 项目特定的后端开发约束、架构设计模式、GORM 数据库交互规范以及关键的 JSON 序列化避坑指南。所有参与项目后端开发的代码必须严格遵守。

---

## 1. 统一接口输入与响应处理（Controller 约束）

为了保证 API 的一致性，并消除控制器层中大量的样板代码，所有 Gin Controller 必须遵守以下规范：

### 1.1 参数解析与绑定
- **URL ID 参数解析**：必须调用统一的 `parseIDParam(c)` 辅助函数。严禁手写 `strconv.ParseUint(c.Param("id"), ...)`。
- **JSON 请求体绑定**：必须调用统一的 `bindJSON(c, &input)` 辅助函数。严禁手动调用 `c.ShouldBindJSON` 或 `json.NewDecoder` 并重复编写错误返回逻辑。

### 1.2 标准 API 响应
- 所有控制器方法的返回必须统一使用 `respondSuccess`、`respondFailure`、`respondBadRequest` 等标准方法。
- **严禁手写** `c.JSON(http.StatusOK, gin.H{...})`，以确保全局 API 响应字段结构（`success`/`message`/`data`）的百分之百一致。

> [!IMPORTANT]
> 接口的入参解析与响应统一规范定义在 [openflare_server/controller/response.go](file:///Users/ryan/DEV/Go/OpenFlare/openflare_server/controller/response.go) 中。

---

## 2. 纯净工具类与数据库逻辑完全隔离（Utils 约束）

为了确保代码的可测试性、高内聚和低耦合，`utils/` 目录下的工具包必须保持纯净性：

### 2.1 无副作用与解耦原则
- 所有底层客户端与外部服务对接包（如 `utils/acme` 证书操作、邮件发送、DNS 供应商对接等）**必须完全剥离数据库或 GORM 依赖**。
- 工具包中严禁导入 `openflare/model` 包或直接访问数据库连接。它们应当只接受基础数据类型（如 `string`、`[]byte` 等）或本地无依赖结构体作为输入，并返回纯粹的计算或请求结果。

### 2.2 业务服务层（Service）职责
- 业务服务层 `service/` 负责数据库实体的加载、组装、事务持久化，并将底层的具体网络或加密操作委托给 `utils/` 工具包。
- 这样不仅保证了底层工具类的百分之百可单元测试性，也维护了清晰的系统分层。

---

## 3. Go 泛型切片去重与 JSON 序列化陷阱（Slice 约束）

在进行切片操作和去重时，必须使用泛型辅助函数，并注意 Go Slice 的空/零值在 JSON 序列化中的表现。

### 3.1 避免重复编写 map-seen 逻辑
- 禁止在 `service/` 或 `model/` 中手写临时的 map-seen 去重样板代码。
- 必须统一调用基于 Go 泛型实现的 [openflare_server/utils/slice.go](file:///Users/ryan/DEV/Go/OpenFlare/openflare_server/utils/slice.go) 中的 `utils.Unique()` 辅助函数。

### 3.2 关键的 JSON 序列化规则（Nil vs. Empty Slice）
在 Go 中，未初始化的 `nil` 切片和已初始化的空切片 `[]T{}` 在内存中不同，它们在序列化为 JSON 时也有着决定性的区别：
- **`nil` 切片**：序列化为 JSON `null`。
- **空切片 (`make([]T, 0)`)**：序列化为 JSON `[]`。

> [!CAUTION]
> **开发避坑准则**：
> 1. GORM 数据库的很多 JSON/Array 字段（例如 `domain_cert_ids`、`upstreams` 等）或配置版本变更检测机制（如 `checksum` 计算和 `diff` 检测），要求空数组在 JSON 中必须表示为 `[]` 而非 `null`，否则会触发重复发布或解析失败的 bug。
> 2. `utils.Unique` 必须具备 **Nil-Preservation（空值保留）** 特性：
>    - 如果传入的 Slice 是 `nil`，它必须返回 `nil`，以支持 `omitempty` 或在需要表示“缺失”的场景中输出 `null`。
>    - 如果传入的 Slice 不是 `nil`（即使长度为 0 或去重后长度为 0），它必须返回非 nil 的空切片 `make([]T, 0)`，以确保序列化为 `[]`。
> 3. 所有类似的切片加工辅助函数都必须遵循此行为。
