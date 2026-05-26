# ModelMux

<p align="center">
  <img src="ModelMux.png" alt="ModelMux" width="640" />
</p>

ModelMux 是一个本地模型提供商管理与请求路由工具。它把多个模型服务提供商、上游地址和 API Key 池集中到一个本地入口，通过 `active_provider` 选择当前使用的提供商，并在该提供商内部完成 key 轮询、错误摘除、限速冷却、热重载、状态持久化、结构化日志和嵌入式可视化管理台。

它适合个人或小型本地环境统一管理多组模型服务凭据：客户端只需要连接本地代理地址，真实上游地址和 key 由 ModelMux 维护。

## 功能特性

- 多提供商集中配置，通过 `active_provider` 控制当前路由目标
- 每个提供商拥有独立 key 池，key 状态和统计互不影响
- 当前提供商内部按带 in-flight 感知的 round-robin 选择可用 key
- `429` 自动进入 cooling 状态，并支持 `Retry-After`
- `401` 自动标记 key 为 invalid
- 余额或额度不足类 `403` 自动标记 key 为 invalid，并换 key 重试
- 连接抖动、响应头超时和 `502/503/504` 支持独立的 transient 重试预算
- 所有 key 仅因 cooling 暂不可用时，可短等最近恢复的 key，尽量避免立刻向客户端暴露 `503`
- 请求体大小限制，避免异常请求占用过多内存
- 流式响应按块透传，降低代理侧缓冲影响
- 配置文件热重载，支持手动 reload
- 嵌入式可视化管理台，支持 provider、key、设置、事件与导出操作
- 管理接口查看健康状态、提供商状态、key 状态和运行概览
- key 池状态持久化，状态文件只保存 key 哈希标识
- 结构化日志，支持控制台、文件、控制台加文件双写
- 日志文件支持轮转、压缩和保留策略
- 单二进制部署，默认只监听本机管理端口

## 快速开始

### 准备配置

```powershell
Copy-Item config.example.json config.json
```

编辑 `config.json`：

```json
{
  "listen": ":8080",
  "admin_listen": "127.0.0.1:8081",
  "active_provider": "primary",
  "providers": [
    {
      "id": "primary",
      "target_url": "https://your-provider.com",
      "keys": [
        "sk-your-primary-key-1",
        "sk-your-primary-key-2"
      ]
    },
    {
      "id": "backup",
      "target_url": "https://backup-provider.com",
      "keys": [
        "sk-your-backup-key-1"
      ]
    }
  ],
  "cooling_seconds": 60,
  "max_retries": 3,
  "max_transient_retries": 1,
  "request_timeout_seconds": 120,
  "connect_timeout_seconds": 5,
  "response_header_timeout_seconds": 30,
  "transient_cooling_seconds": 15,
  "wait_for_key_timeout_ms": 1000,
  "max_body_bytes": 33554432,
  "log_level": "info",
  "log_format": "text",
  "log_output": "both",
  "log_file": "logs/modelmux.log",
  "log_max_size_mb": 20,
  "log_max_backups": 5,
  "log_max_age_days": 30,
  "log_compress": true,
  "persist_state": true,
  "state_file": "state.json",
  "invalid_ttl_hours": 24
}
```

### 一键启动（推荐）

```powershell
.\start.ps1
```

脚本会自动检测并构建 `modelmux.exe`，启动后会自动打开管理控制台。

### 构建

```powershell
go build -trimpath -ldflags="-s -w" -o modelmux.exe .
```

### 启动

```powershell
.\modelmux.exe -config config.json
```

默认地址：

```text
代理入口: http://127.0.0.1:8080
管理入口: http://127.0.0.1:8081
控制台入口: http://127.0.0.1:8081/console/
```

客户端应把本地代理入口作为模型服务 base URL，并提供任意非空客户端侧 API Key。转发到上游时，ModelMux 会使用当前提供商 key 池中选出的真实 key 覆盖认证头。

## 管理台

管理台已嵌入到 Go 二进制中，访问 `http://127.0.0.1:8081/console/` 即可使用。

### 页面

- `总览`：当前 provider、key 状态、最近事件和快捷重载
- `提供商`：新增、编辑、删除、切换 active provider，key 池追加/替换/删除，以及单 key 手动恢复为 active
- `设置`：运行参数、日志参数和状态持久化配置
- `事件`：最近运行事件、级别过滤和关键词搜索
- `关于`：运行信息、配置导出和状态导出

### 常用 API

只读：

- `GET /admin/api/v1/dashboard`：首页聚合视图（active provider + key 计数 + 最近事件）
- `GET /admin/api/v1/providers`：provider 列表与状态汇总
- `GET /admin/api/v1/providers/{id}`：单个 provider 详情与每个 key 的状态
- `GET /admin/api/v1/settings`：当前配置 + 字段热生效分类
- `GET /admin/api/v1/events`：最近事件，支持 `?limit=N`
- `GET /admin/api/v1/about`：运行环境与可用 API 列表

写操作（均经过原子写盘 + 热重载，失败时自动回滚到上一版本）：

- `POST   /admin/api/v1/providers`：新增 provider
- `PUT    /admin/api/v1/providers/{id}`：更新 provider 的 `target_url`
- `DELETE /admin/api/v1/providers/{id}`：删除非 active provider
- `POST   /admin/api/v1/providers/{id}/activate`：切换 active provider
- `POST   /admin/api/v1/providers/{id}/keys:append`：追加 keys
- `POST   /admin/api/v1/providers/{id}/keys:replace`：全量替换 keys
- `POST   /admin/api/v1/providers/{id}/keys:delete`：按 `key_id` 删除 keys
- `POST   /admin/api/v1/providers/{id}/keys/{key_id}/reset`：把某个 key 手动恢复为 active
- `PUT    /admin/api/v1/settings`：保存设置
- `POST   /admin/api/v1/reload`：手动触发配置重载

备份：

- `POST /admin/api/v1/config/backup`：导出当前配置 JSON
- `POST /admin/api/v1/state/backup`：导出当前 key 池状态快照

### 快捷键

- `Ctrl` / `⌘ + R`：触发后端 reload
- 先按 `g` 再按下列任一键切换页面：
  - `d` → 总览
  - `p` → 提供商
  - `s` → 设置
  - `e` → 事件
  - `a` → 关于

## 配置说明

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `listen` | `:8080` | 本地代理服务监听地址 |
| `admin_listen` | `127.0.0.1:8081` | 管理服务监听地址 |
| `active_provider` | 第一个 provider | 当前使用的提供商 ID |
| `providers` | 无 | 提供商列表，至少一个 |
| `providers[].id` | 无 | 提供商稳定 ID，不能重复 |
| `providers[].target_url` | 无 | 提供商上游服务地址 |
| `providers[].keys` | 无 | 提供商 API Key 列表，至少一个 |
| `cooling_seconds` | `60` | 未提供 `Retry-After` 时的 429 冷却秒数 |
| `max_retries` | `3` | 可换 key 错误的最大重试次数 |
| `max_transient_retries` | `1` | 网络/provider 临时故障的最大重试次数 |
| `request_timeout_seconds` | `120` | 上游请求超时时间 |
| `connect_timeout_seconds` | `5` | 上游连接和 TLS 握手超时 |
| `response_header_timeout_seconds` | `30` | 等待上游返回响应头的超时 |
| `transient_cooling_seconds` | `15` | 连接级临时故障命中后，当前 key 的短冷却秒数 |
| `wait_for_key_timeout_ms` | `1000` | 所有 key 仅因 cooling 不可用时允许短等的毫秒数 |
| `max_body_bytes` | `33554432` | 请求体最大字节数，默认 32MB |
| `log_level` | `info` | `debug` / `info` / `warn` / `error` |
| `log_format` | `text` | `text` / `json` |
| `log_output` | `stdout` | `stdout` / `file` / `both` |
| `log_file` | 空 | 日志文件路径 |
| `log_max_size_mb` | `20` | 单个日志文件最大 MB |
| `log_max_backups` | `5` | 保留旧日志文件数量 |
| `log_max_age_days` | `30` | 保留旧日志天数 |
| `log_compress` | `false` | 是否压缩旧日志 |
| `persist_state` | `true` | 是否保存 key 池运行状态 |
| `state_file` | `state.json` | key 池状态文件路径 |
| `invalid_ttl_hours` | `24` | invalid 状态保留小时数，超时后下次启动恢复 active |

旧版 `target_url` 加 `keys` 的单提供商配置仍可使用，会被自动转换为 ID 为 `default` 的提供商。

## 热重载

ModelMux 会监听配置文件所在目录。保存配置文件后，可热更新字段会自动生效。

热生效字段：

- `active_provider`
- `providers`
- `providers[].keys`
- `providers[].target_url`
- `cooling_seconds`
- `max_retries`
- `max_transient_retries`
- `request_timeout_seconds`
- `connect_timeout_seconds`
- `response_header_timeout_seconds`
- `transient_cooling_seconds`
- `wait_for_key_timeout_ms`
- `max_body_bytes`

需要重启才生效的字段：

- `listen`
- `admin_listen`
- `log_level`
- `log_format`
- `log_output`
- `log_file`
- `log_max_size_mb`
- `log_max_backups`
- `log_max_age_days`
- `log_compress`

手动触发 reload：

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8081/admin/reload
```

热重载失败时，旧运行时配置会继续保留。通过管理台或 REST API 修改配置时，磁盘文件采用 `.tmp` + rename 原子写盘；若新配置触发的 reload 失败，配置文件会被自动回滚到上一版本并恢复原运行时，避免管理台保存让服务停留在不一致状态。

## 管理接口

### `GET /admin/health`

查看当前提供商是否还有可用 key。

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/health
```

响应示例：

```json
{
  "active_provider": "primary",
  "active_keys": 2,
  "total_keys": 3,
  "ok": true
}
```

### `GET /admin/status`

查看所有提供商和 key 的运行状态。

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/status
```

主要字段：

- `active_provider`: 当前提供商 ID
- `active_keys`: 当前提供商可用 key 数量
- `cooling_keys`: 当前提供商 cooling key 数量
- `invalid_keys`: 当前提供商 invalid key 数量
- `providers`: 所有提供商的状态列表
- `keys`: 当前提供商 key 状态、请求数、错误数、平均延迟和冷却结束时间

### `POST /admin/reload`

手动重读配置文件。

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8081/admin/reload
```

## Key 状态

```text
active --429--> cooling --冷却到期--> active
active --401--> invalid
active --余额或额度不足类 403--> invalid
```

- `active`: 可用 key
- `cooling`: 因限速暂时冷却
- `invalid`: 因认证失败或余额不足被摘除

`cooling` 到期后会自动恢复。`invalid` 在进程运行期间不会自动恢复；启用状态持久化后，距上次 401 / 额度错误超过 `invalid_ttl_hours` 的 invalid key 会在下次启动时恢复为 active。也可以通过控制台「重置」按钮、`POST /admin/api/v1/providers/{id}/keys/{key_id}/reset`，或更新配置中的 key 列表来手动恢复。

状态持久化文件按 provider ID 分组保存，只保存 key 的 SHA-256 标识，不保存完整 API Key。

连接级临时故障（例如 EOF、连接重置、响应头超时）会把当前 key 短暂置为 `cooling`。provider 级临时故障（例如 DNS、连接拒绝、TLS 或上游统一返回 `502/503/504`）不会污染 key 状态，而是消耗独立的 transient 重试预算。

## 请求处理策略

一次请求的处理流程：

1. 读取请求体，并检查 `max_body_bytes`
2. 获取当前 `active_provider` 的 key 池
3. 从 key 池按带 in-flight 感知的 round-robin 选择一个可用 key
4. 覆盖上游请求的 `Authorization` 和 `X-Api-Key`
5. 转发到当前提供商的 `target_url`
6. `429`：当前 key 进入 cooling，换 key 重试
7. `401`：当前 key 标记 invalid，换 key 重试
8. 余额或额度不足类 `403`：当前 key 标记 invalid，换 key 重试
9. 连接级临时故障：当前 key 进入短 cooling，并按 `max_transient_retries` 尝试下一个 key
10. provider 级临时故障（如 `502/503/504`、DNS、连接拒绝）：不污染 key 状态，但会消耗 `max_transient_retries`
11. 如果所有 key 仅因 cooling 暂时不可用，且最近一个恢复时间不超过 `wait_for_key_timeout_ms`，代理会先短等再试
12. 其他状态码：原样透传给客户端
13. 响应体按 4KB 块转发，并在每次写入后 flush

ModelMux 不会在一个提供商耗尽时自动切换到另一个提供商。提供商切换由 `active_provider` 控制，修改后热重载即可生效。

## 日志

日志使用 `log/slog`，统一包含：

- `category`: 日志分类
- `event`: 事件名称

常见分类：

- `lifecycle`
- `config`
- `admin`
- `proxy`
- `retry`
- `upstream`
- `stream`
- `state`

示例配置：

```json
{
  "log_output": "both",
  "log_file": "logs/modelmux.log",
  "log_max_size_mb": 20,
  "log_max_backups": 5,
  "log_max_age_days": 30,
  "log_compress": true
}
```

如果需要机器解析日志，可以设置：

```json
{
  "log_format": "json"
}
```

## 构建与测试

```powershell
cd web
npm install
npm run build
cd ..
go test ./...
go vet ./...
go test -race ./...
go build -trimpath -ldflags="-s -w" -o modelmux.exe .
```

如果你主要修改前端管理台，先在 `web/` 下运行 `npm run dev` 可以获得本地热更新预览。

## 目录结构

```text
ModelMux/
├── admin/              # 管理接口
├── config/             # 配置加载、校验和文件监听
├── logx/               # 日志字段和脱敏工具
├── pool/               # provider 池、key 池和状态机
├── proxy/              # 请求转发、重试和流式响应
├── state/              # key 池状态持久化
├── config.example.json # 示例配置
├── go.mod
├── Makefile
└── README.md
```

## 故障排查

### 服务是否可用

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/health
```

如果 `ok` 为 `false`，说明当前提供商没有可用 key。

### 请求返回 503

查看 key 状态：

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/status
```

常见原因：

- 所有 key 都在 cooling
- 所有 key 都被标记 invalid
- provider 级临时故障在 `max_transient_retries` 预算内仍未恢复
- 当前 `active_provider` 配置错误
- key 列表为空或上游地址不可用

如果所有 key 都只是短暂 cooling，代理会在 `wait_for_key_timeout_ms` 预算内短等最近恢复的 key，再决定是否返回 `503`。

### 配置修改未生效

确认修改的是启动时传入的配置文件，并手动 reload：

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8081/admin/reload
```

监听地址和日志输出配置需要重启后生效。

### 余额不足类错误反复出现

如果上游返回余额或额度不足类 `403`，ModelMux 会将命中的 key 标记为 invalid 并换 key 重试。若仍持续失败，通常表示当前提供商下所有可用 key 都不足以完成请求，或者这些 key 共享同一个账户余额。

### 网络抖动或上游偶发 502/503/504

ModelMux 会先按 `max_transient_retries` 做短路重试。连接级抖动会让当前 key 暂时 cooling；provider 级故障不会污染 key 状态。如果你感觉失败暴露过快，可以适当提高：

- `max_transient_retries`
- `transient_cooling_seconds`
- `wait_for_key_timeout_ms`

如果你感觉切换太慢，可以适当降低：

- `request_timeout_seconds`
- `connect_timeout_seconds`
- `response_header_timeout_seconds`

## 安全建议

- 管理端口默认保持 `127.0.0.1:8081`
- 不要把管理端口暴露到公网
- 不要提交 `config.json`、`state.json` 或日志文件
- 不要在日志、截图或 issue 中暴露完整 API Key
- 远程访问时应额外增加网络隔离或鉴权层
