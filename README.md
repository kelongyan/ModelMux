# claude-key-proxy

面向 Claude / Anthropic-compatible API 的本地反向代理。它把多个 provider 的上游地址和 API Key 池收口成一个本地入口，运行时只使用当前选中的 `active_provider`，并在该 provider 内部做 key 轮换、限速冷却、失效摘除、配置热重载和日志轮转。

适合个人在本机给 Claude Code 或其他兼容客户端使用：客户端只连 `http://127.0.0.1:8080`，真正的上游地址和 key 池由代理维护。

## 特性

- 支持在配置文件中保存多个 provider，并用 `active_provider` 选择当前使用的 provider
- 每个 provider 拥有独立 key 池，默认 round-robin；不会在多个 provider 之间自动轮询或 fallback
- 上游返回 `429` 时自动冷却当前 key，并切换下一个 key 重试
- 上游返回 `401` 时自动标记当前 key 失效，避免继续使用
- 支持 `Retry-After`，优先使用上游建议冷却时间
- SSE / 流式响应透传，每个响应块写出后立即 flush
- 自动监听配置文件变化并热重载运行时配置
- 管理接口查看健康状态、key 状态和手动 reload
- key 池运行状态持久化，按 provider 分组保存，状态文件不包含完整 API Key
- 结构化日志，支持按 category / event 分类
- 支持控制台、文件、控制台+文件双写
- 支持日志文件自动轮转、压缩和保留策略
- 单二进制部署，默认本机使用

## 快速开始

### 1. 准备配置

复制示例配置：

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
  "request_timeout_seconds": 120,
  "max_body_bytes": 33554432,
  "log_level": "info",
  "log_format": "text",
  "log_output": "both",
  "log_file": "logs/claude-key-proxy.log",
  "log_max_size_mb": 20,
  "log_max_backups": 5,
  "log_max_age_days": 30,
  "log_compress": true
}
```

### 2. 构建

```powershell
make build
```

没有 `make` 时可以直接运行：

```powershell
go build -trimpath -ldflags="-s -w" -o claude-key-proxy.exe .
```

### 3. 启动

```powershell
.\claude-key-proxy.exe -config config.json
```

默认代理地址是：

```text
http://127.0.0.1:8080
```

默认管理地址是：

```text
http://127.0.0.1:8081
```

## 配置 Claude Code

PowerShell：

```powershell
$env:ANTHROPIC_BASE_URL='http://127.0.0.1:8080'
$env:ANTHROPIC_API_KEY='dummy'
claude
```

`ANTHROPIC_API_KEY` 这里只需要是任意非空值，真正发给上游的 key 会由代理从当前 `active_provider` 的 `keys` 列表中选择。

## 配置项

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `listen` | `:8080` | 代理服务监听地址 |
| `admin_listen` | `127.0.0.1:8081` | 管理服务监听地址，默认只监听本机 |
| `active_provider` | 第一个 provider | 当前使用的 provider ID |
| `providers` | 无 | provider 列表，至少一个 |
| `providers[].id` | 无 | provider 稳定 ID，不能重复 |
| `providers[].target_url` | 无 | 当前 provider 的上游 Claude 兼容 API 地址 |
| `providers[].keys` | 无 | 当前 provider 的 API Key 列表，至少一个 |
| `cooling_seconds` | `60` | 429 后默认冷却秒数 |
| `max_retries` | `3` | 401/429 后最大重试次数 |
| `request_timeout_seconds` | `120` | 上游请求超时 |
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

旧版 `target_url` + `keys` 单 provider 配置仍然兼容，会被自动视为 ID 为 `default` 的 provider。如果设置了 `log_file` 但没有设置 `log_output`，程序会默认使用 `both`，也就是控制台和文件双写。

## 热重载

程序会自动监听配置文件所在目录。保存 `config.json` 后，命中的配置会自动热重载。

会热生效：

- `active_provider`
- `providers`
- `providers[].keys`
- `providers[].target_url`
- `cooling_seconds`
- `max_retries`
- `request_timeout_seconds`
- `max_body_bytes`

需要重启才生效：

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

也可以手动触发 reload：

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8081/admin/reload
```

热重载失败时，旧的运行时配置会继续保留，不会把代理切到半坏状态。

## Admin API

### `GET /admin/health`

查看代理是否还有可用 key。

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

查看所有 key 的状态和统计信息。

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/status
```

响应字段：

- `active_keys`: 当前可用 key 数量
- `cooling_keys`: 正在冷却的 key 数量
- `invalid_keys`: 已失效 key 数量
- `active_provider`: 当前选中的 provider ID
- `providers`: 所有 provider 的 key 状态汇总
- `keys`: 当前 active provider 内每个 key 的状态、请求数、错误数、平均延迟、冷却结束时间

### `POST /admin/reload`

手动重读配置文件。

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8081/admin/reload
```

## Key 状态机

```text
active --429--> cooling --冷却到期--> active
active --401--> invalid
```

状态说明：

- `active`: 可用
- `cooling`: 因 429 暂时冷却
- `invalid`: 因 401 被摘除

`cooling` 到期后会自动恢复。`invalid` 在进程运行期间不会自动恢复；启用状态持久化后，超过 `invalid_ttl_hours` 的 invalid key 会在下次启动时恢复为 active，也可以通过更新配置中的 key 列表重新启用。

启用状态持久化后，`cooling`、`invalid` 和请求统计会写入 `state_file`。状态按 provider ID 分组保存，并只保存 key 的 SHA-256 标识，不保存完整 API Key。启动时会恢复仍在 TTL 内的 `invalid` 状态；超过 `invalid_ttl_hours` 的 invalid key 会恢复为 active。

## 请求处理策略

代理处理一次请求时：

1. 读取请求体，并检查 `max_body_bytes`
2. 从 key 池选择一个可用 key
3. 覆盖请求中的 `Authorization` 和 `X-Api-Key`
4. 转发到当前 `active_provider` 的 `target_url`
5. 上游返回 `429` 时冷却当前 key，换 key 重试
6. 上游返回 `401` 时标记当前 key invalid，换 key 重试
7. 不会因为某个 provider 的 key 耗尽而自动切换到其他 provider；需要修改 `active_provider` 并热重载
8. 其他状态码直接透传给客户端
9. 响应体按 4KB 块流式转发，并在每次写入后 flush

为了支持重试，请求体会在代理内完整读入内存。因此建议保持 `max_body_bytes` 为合理值。

## 日志

日志使用 `log/slog` 输出，并统一带有：

- `category`: 日志分类
- `event`: 具体事件

常见分类：

- `lifecycle`
- `config`
- `admin`
- `proxy`
- `retry`
- `upstream`
- `stream`

文件轮转由 `lumberjack` 处理。示例配置：

```json
{
  "log_output": "both",
  "log_file": "logs/claude-key-proxy.log",
  "log_max_size_mb": 20,
  "log_max_backups": 5,
  "log_max_age_days": 30,
  "log_compress": true
}
```

如果想让日志更适合机器解析，可以设置：

```json
{
  "log_format": "json"
}
```

## 构建与测试

常用命令：

```powershell
make build
make run
make test
```

直接使用 Go：

```powershell
go test ./...
go vet ./...
go test -race ./...
go build -trimpath -ldflags="-s -w" -o claude-key-proxy.exe .
```

跨平台构建：

```powershell
make build-linux
make build-windows
make build-mac
```

## 目录结构

```text
claude-key-proxy/
├── admin/              # 管理 API
├── config/             # 配置加载、校验、文件监听
├── logx/               # 日志分类和脱敏工具
├── pool/               # key 池、状态机、轮询
├── proxy/              # 反向代理、重试、流式响应
├── config.example.json # 示例配置
├── go.mod
├── Makefile
└── README.md
```

## 依赖

项目主要逻辑仍基于 Go 标准库实现，当前引入的第三方依赖用于稳定性和易用性：

- `github.com/fsnotify/fsnotify`: 监听配置文件变化并自动热重载
- `gopkg.in/natefinch/lumberjack.v2`: 日志文件轮转

## 故障排查

### Claude Code 连接不上

先检查代理是否启动：

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/health
```

再确认客户端环境变量：

```powershell
$env:ANTHROPIC_BASE_URL
$env:ANTHROPIC_API_KEY
```

### 一直返回 503

通常表示没有可用 key。查看状态：

```powershell
Invoke-RestMethod http://127.0.0.1:8081/admin/status
```

可能原因：

- 所有 key 都被 429 冷却
- 所有 key 都因 401 被标记 invalid
- 配置文件里 key 列表为空或错误

### 修改配置后没生效

确认修改的是启动时传入的配置文件：

```powershell
.\claude-key-proxy.exe -config config.json
```

可以手动 reload：

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:8081/admin/reload
```

端口和日志输出配置需要重启后生效。

### 日志文件没有生成

检查：

- `log_output` 是否为 `file` 或 `both`
- `log_file` 是否非空
- 当前进程是否有权限创建日志目录
- 如果日志级别是 `warn` 或 `error`，普通请求成功日志不会输出

## 安全建议

- `admin_listen` 默认保持 `127.0.0.1:8081`
- 不要把 admin 端口暴露到公网
- 不要把完整 API Key 写入日志或截图
- `config.json` 建议加入 `.gitignore`
- 如果必须远程访问，请放在可信网络或额外加鉴权层
