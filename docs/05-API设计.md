# 05 · API 设计

## 1. 通用约定

| 项 | 约定 |
| --- | --- |
| Base Path | `/api/v1` |
| 数据格式 | 请求/响应均为 JSON（UTF-8）；时间使用 RFC3339 |
| 分页 | `page`（默认 1）、`pageSize`（默认 20，上限 100）；响应 `{items, total, page, pageSize}` |
| 错误响应 | `{"code": "ERROR_CODE", "message": "...", "details": {...}}` |
| 鉴权 | 默认 `auth.provider=none`；预留 `X-User-Id` 请求头由中间件注入 `actor`（可插拔 AuthProvider，本期占位） |
| 幂等 | 创建类接口支持 `Idempotency-Key` 请求头（可选用） |
| 实时事件 | `GET /api/v1/events/stream`（SSE），按 `Last-Event-ID`（=事件 seq）续传 |

### 错误码

| 错误码 | HTTP | 说明 |
| --- | --- | --- |
| `VALIDATION_FAILED` | 400 | 定义/参数校验失败（details 含逐条错误） |
| `NOT_FOUND` | 404 | 定义/实例/任务不存在 |
| `CONFLICT` | 409 | 乐观锁冲突 / 状态机非法迁移 / 审批终态操作 |
| `INVALID_STATE` | 409 | 实例状态不允许该操作（如已完成后取消） |
| `FORBIDDEN` | 403 | 非审批参与人/无权限 |
| `UNAUTHORIZED` | 401 | 未认证 |
| `RATE_LIMITED` | 429 | 触发限流 |
| `INTERNAL` | 500 | 内部错误（响应不含堆栈） |

## 2. 工作流定义管理

### 2.1 定义 CRUD

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/workflows` | 创建定义（默认 draft，自动校验；校验失败返回 VALIDATION_FAILED） |
| GET | `/workflows` | 列表（过滤：name、status；分页） |
| GET | `/workflows/{id}` | 详情（含最新版本 graph） |
| GET | `/workflows/{id}/versions/{version}` | 指定版本详情 |
| PUT | `/workflows/{id}` | 更新（若存在已发布版本则升版本） |
| POST | `/workflows/{id}/publish` | 发布（冻结该版本，可被实例引用） |
| POST | `/workflows/{id}/archive` | 归档（软删除，已运行实例不受影响） |
| POST | `/workflows/validate` | **只校验不落库**，返回校验报告 |

请求体示例（创建/更新）：

```json
{
  "name": "leave-approval",
  "graph": { "nodes": [...], "edges": [...] },
  "recoveryConfig": {"instanceMaxRetry": 5, "deadLetter": true}
}
```

校验报告示例：

```json
{
  "valid": false,
  "errors": [
    {"path": "graph.nodes[2]", "code": "EDGE_DANGLING", "message": "节点 hr_approve 缺少出边"},
    {"path": "graph.edges[3].condition", "code": "CONDITION_SYNTAX", "message": "表达式解析失败: unexpected token"}
  ]
}
```

## 3. 实例管理

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| POST | `/workflows/{id}/instances` | 启动实例：`{"bizKey":"...","context":{...}}`（同 workflow+bizKey 幂等返回已有实例） |
| GET | `/instances` | 实例列表（过滤：workflowId、status、bizKey、时间范围；分页） |
| GET | `/instances/{id}` | 实例详情（状态、上下文、当前节点、节点实例列表、最近事件） |
| GET | `/instances/{id}/nodes` | 节点实例明细（状态/输入输出快照/重试次数） |
| GET | `/instances/{id}/events` | 事件列表（游标分页：`cursor`=seq） |
| POST | `/instances/{id}/suspend` | 暂停（WAITING_* 可暂停） |
| POST | `/instances/{id}/resume` | 恢复推进 |
| POST | `/instances/{id}/cancel` | 取消（触发已完成节点补偿，终态 TERMINATED） |
| POST | `/instances/{id}/retry` | 死信/失败实例重新入队（从失败节点重试） |
| POST | `/instances/{id}/restart` | 从起始节点重跑（需 cancel 或 DEAD 状态） |

## 4. 审批中心

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/approval-tasks?assignee={user}` | 我的待办（status=PENDING，含模式/截止时间/流程信息） |
| GET | `/approval-tasks/{taskId}` | 任务详情（历史记录、可执行操作） |
| POST | `/approval-tasks/{taskId}/approve` | 同意：`{"opinion":"同意","actor":"alice"}` |
| POST | `/approval-tasks/{taskId}/reject` | 驳回：`{"opinion":"资料不全","actor":"alice"}`（按节点驳回路由） |
| POST | `/approval-tasks/{taskId}/transfer` | 转办：`{"to":"bob","actor":"alice"}` |
| POST | `/approval-tasks/{taskId}/add-approver` | 加签：`{"add":"carol","actor":"alice"}` |
| POST | `/approval-tasks/{taskId}/delegate` | 委派：`{"to":"bob","actor":"alice"}` |
| POST | `/approval-tasks/{taskId}/revoke` | 撤销委派/加签 |
| GET | `/instances/{id}/approval-records` | 实例审批记录 |

所有审批操作返回最新任务状态与实例状态；操作人校验失败返回 FORBIDDEN，任务已终态返回 CONFLICT。

## 5. 运维与观测

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/healthz` | 存活探针：`{"status":"ok","checks":{"store":"ok","migrate":"ok"}}` |
| GET | `/readyz` | 就绪探针（store 可达、执行器运行中） |
| GET | `/metrics` | 指标（Prometheus 文本格式：见 docs/08 §4） |
| GET | `/events/stream` | SSE 事件流（`data: {"seq":..,"type":..,"instanceId":..,...}`，心跳注释行保活） |
| GET | `/dead-letters` | 死信实例列表（运维视图） |
| GET | `/dead-letters/{instanceId}/retry` | 快捷重试（等价 POST /instances/{id}/retry） |

## 6. DTO 设计（api/dto.go 摘要）

| DTO | 关键字段 |
| --- | --- |
| CreateWorkflowReq / WorkflowDTO | id、name、version、status、graph、recoveryConfig、createdAt |
| ValidateWorkflowReq / ValidationReport | valid、errors[]（path/code/message） |
| CreateInstanceReq | bizKey、context |
| InstanceDTO | id、workflowId、workflowVersion、bizKey、status、currentNodeIds、context、errorInfo、timestamps |
| NodeInstanceDTO | nodeId、type、status、attempts、input/output、lastError、timestamps |
| ApprovalTaskDTO | id、instanceId、nodeId、mode、status、assignees、passRatio、deadline、timeoutPolicy |
| ApprovalActionReq | actor、opinion、to/add |
| EventDTO | seq、type、instanceId、nodeId、actor、detail、occurredAt |
| ListResp[T] | items、total、page、pageSize |

## 7. 与前端/执行器的关系

- **前端**（docs/07）全部数据经本 API 获取，事件流经 SSE 实时刷新；
- **执行器**（docs/06）不经过 HTTP API，直接调用 engine/store（api 与 executor 互不依赖）；
- API 处理器为薄层：参数解析 → DTO 转换 → 调用 engine/store/validator → 统一错误映射，业务逻辑不放 handler。
