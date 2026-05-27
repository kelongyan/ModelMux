# ModelMux

ModelMux 是一个本地运行的模型 API 反向代理。客户端只连一个本地地址，ModelMux 根据 `active_provider` 选择当前 provider，再从该 provider 自己的 key 池里挑选可用 key 转发请求。

适合个人或小团队把多家模型服务、多组上游地址、多把 API key 收到一个本地入口里管理。

## 核心行为

- 只使用当前 `active_provider`，不会自动跨 provider 故障转移。
- 每个 provider 有独立 key 池，key 状态互不影响。
- 转发前会覆盖 `Authorization` 和 `X-Api-Key` 为选中的真实上游 key。
- `429` 会让当前 key 进入 cooling，并优先尊重上游 `Retry-After`。
- `401` 和余额/额度不足类 `403` 会把当前 key 标记为 invalid。
- DNS、连接拒绝、TLS 和上游 `502/503/504` 属于 provider 临时故障，不会污染 key 状态。
- 流式响应按小块透传并立即 flush，适合 SSE / streaming 场景。
- 配置支持热重载；管理台保存配置会原子写盘，reload 失败会回滚。
- 状态持久化只保存 key 的 SHA-256 标识，不保存完整 API key。

## 快速开始

准备配置：

```powershell
Copy-Item config.example.json config.json
```

编辑 `config.json`，至少改掉 `target_url` 和 `keys`：

```json
{
  "listen": "127.0.0.1:18080",
  "admin_listen": "127.0.0.1:18081",
  "active_provider": "primary",
  "providers": [
    {
      "id": "primary",
      "target_url": "https://your-provider.example.com",
      "keys": ["sk-your-key-1", "sk-your-key-2"]
    },
    {
      "id": "backup",
      "target_url": "https://backup-provider.example.com",
      "keys": ["sk-backup-key"]
    }
  ]
}
```

一键启动：

```powershell
.\start.ps1
```

`start.ps1` 会检查 `config.json`，缺少 `modelmux.exe` 时自动构建，后台启动服务，并打开管理台。

手动构建和启动：

```powershell
go build -trimpath -ldflags="-s -w" -o modelmux.exe .
.\modelmux.exe -config config.json
```

默认入口：

```text
代理入口: http://127.0.0.1:18080
管理入口: http://127.0.0.1:18081
管理台:   http://127.0.0.1:18081/console/
```

客户端配置方式：

- Base URL 填 `http://127.0.0.1:18080/v1`。
- 客户端 API key 填任意非空值即可。
- ModelMux 转发到上游时会替换成当前 provider key 池里选中的真实 key。

## 配置说明

推荐使用多 provider 配置：

```json
{
  "active_provider": "primary",
  "providers": [
    {
      "id": "primary",
      "target_url": "https://provider-a.example.com",
      "keys": ["sk-a1", "sk-a2"]
    },
    {
      "id": "backup",
      "target_url": "https://provider-b.example.com",
      "keys": ["sk-b1"]
    }
  ]
}
```

兼容旧版单 provider 配置：

```json
{
  "target_url": "https://provider.example.com",
  "keys": ["sk-1", "sk-2"]
}
```

旧版配置会被视为 provider `default`。

常用字段：

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `listen` | `127.0.0.1:18080` | 代理服务监听地址 |
| `admin_listen` | `127.0.0.1:18081` | 管理服务监听地址 |
| `active_provider` | 第一个 provider | 当前使用的 provider ID |
| `providers[].id` | 无 | provider 稳定 ID，不能重复 |
| `providers[].target_url` | 无 | 上游服务根地址，必须是绝对 URL |
| `providers[].keys` | 无 | 当前 provider 的 key 列表，至少一个 |
| `cooling_seconds` | `60` | 429 未返回 `Retry-After` 时的冷却秒数 |
| `max_retries` | `3` | key 级错误的换 key 重试预算 |
| `max_transient_retries` | `1` | 网络/provider 临时故障重试预算 |
| `request_timeout_seconds` | `120` | 单次上游请求总超时 |
| `connect_timeout_seconds` | `5` | 建连和 TLS 握手超时 |
| `response_header_timeout_seconds` | `30` | 等待上游响应头超时 |
| `transient_cooling_seconds` | `15` | 连接级临时故障后的短冷却 |
| `wait_for_key_timeout_ms` | `1000` | 所有 key 仅 cooling 时最多短等多久 |
| `max_body_bytes` | `33554432` | 请求体读取上限，默认 32 MiB |
| `persist_state` | `true` | 是否持久化 key 状态 |
| `state_file` | `state.json` | key 状态文件 |
| `invalid_ttl_hours` | `24` | invalid 状态下次启动自动恢复前的保留小时数 |

日志字段：

| 字段 | 默认值 | 说明 |
|---|---:|---|
| `log_level` | `info` | `debug` / `info` / `warn` / `error` |
| `log_format` | `text` | `text` / `json` |
| `log_output` | `stdout` | `stdout` / `file` / `both` |
| `log_file` | 空 | 文件日志路径 |
| `log_max_size_mb` | `20` | 单日志文件最大 MB |
| `log_max_backups` | `5` | 保留旧日志数量 |
| `log_max_age_days` | `30` | 保留旧日志天数 |
| `log_compress` | `false` | 是否压缩旧日志 |

## 热重载

保存配置文件后，ModelMux 会通过 `fsnotify` 自动 reload。也可以手动触发：

```powershell
Invoke-RestMethod -Method Post http://127.0.0.1:18081/admin/reload
```

这些字段热生效：

- `active_provider`
- `providers`
- `cooling_seconds`
- `max_retries`
- `max_transient_retries`
- `request_timeout_seconds`
- `connect_timeout_seconds`
- `response_header_timeout_seconds`
- `transient_cooling_seconds`
- `wait_for_key_timeout_ms`
- `max_body_bytes`

这些字段需要重启才完全生效：

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
- `persist_state`
- `state_file`
- `invalid_ttl_hours`

## 管理台和 API

管理台地址：

```text
http://127.0.0.1:18081/console/
```

常用接口：

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/admin/health` | 当前 active provider 是否还有可用 key |
| `GET` | `/admin/status` | provider 和 key 池状态 |
| `POST` | `/admin/reload` | 手动重读配置 |
| `GET` | `/admin/api/v1/dashboard` | 管理台首页数据 |
| `GET` | `/admin/api/v1/providers` | provider 列表 |
| `POST` | `/admin/api/v1/providers` | 新增 provider |
| `GET` | `/admin/api/v1/providers/{id}` | provider 详情 |
| `PUT` | `/admin/api/v1/providers/{id}` | 修改 provider 上游地址 |
| `DELETE` | `/admin/api/v1/providers/{id}` | 删除非 active provider |
| `POST` | `/admin/api/v1/providers/{id}/activate` | 切换 active provider |
| `POST` | `/admin/api/v1/providers/{id}/keys:append` | 追加 keys |
| `POST` | `/admin/api/v1/providers/{id}/keys:replace` | 替换 keys |
| `POST` | `/admin/api/v1/providers/{id}/keys:delete` | 删除 keys |
| `POST` | `/admin/api/v1/providers/{id}/keys/{key_id}/reset` | 重置单个 key 为 active |
| `GET` | `/admin/api/v1/settings` | 当前配置和热重载边界 |
| `PUT` | `/admin/api/v1/settings` | 保存设置 |
| `GET` | `/admin/api/v1/events` | 最近运行事件 |
| `GET` | `/admin/api/v1/about` | 运行环境和版本信息 |
| `POST` | `/admin/api/v1/config/backup` | 导出配置 |
| `POST` | `/admin/api/v1/state/backup` | 导出 key 状态 |

健康检查示例：

```powershell
Invoke-RestMethod http://127.0.0.1:18081/admin/health
```

## Key 状态和重试

Key 状态：

```text
active --429--> cooling --到期--> active
active --401--> invalid
active --余额/额度不足 403--> invalid
```

重试规则：

| 类型 | 触发 | key 状态变化 | 预算 |
|---|---|---|---|
| key 级失败 | `401`、`429`、余额/额度不足类 `403` | cooling 或 invalid | `max_retries` |
| 连接级临时失败 | EOF、连接重置、响应头超时等 | 当前 key 短 cooling | `max_transient_retries` |
| provider 级临时失败 | DNS、连接拒绝、TLS、上游 `502/503/504` | 不改变 key 状态 | `max_transient_retries` |

`invalid` 在进程运行期间不会自动恢复。可以通过管理台重置、API 重置、更新 key 列表，或等下次启动时超过 `invalid_ttl_hours` 后恢复。

## 开发和验证

Go 版本以 `go.mod` 为准。

常用命令：

```powershell
go test ./...
go vet ./...
go test -race ./...
go build -trimpath -ldflags="-s -w" -o modelmux.exe .
```

运行单个测试：

```powershell
go test ./pool/ -run TestRoundRobin -v
```

Makefile 快捷命令：

```powershell
make build
make test
make run
```

前端管理台在 `web/`，构建产物会输出到 `web/dist` 并被 Go 二进制嵌入：

```powershell
Set-Location web
npm ci
npm run build
Set-Location ..
```

前端开发服务器会把 `/admin` 代理到本机 Go 管理服务 `127.0.0.1:18081`：

```powershell
Set-Location web
npm run dev
```

如果只改 Go 代码，通常不需要重建前端。若改了 `web/`，发布或提交前要重新生成 `web/dist`。

## 目录速览

```text
admin/   管理 API、事件缓冲区、嵌入式管理台挂载
config/  JSON 配置读取、校验、默认值、热重载、原子写盘
logx/    slog 字段、事件分类、密钥脱敏
pool/    provider 池和每个 provider 内部的 key 池状态机
proxy/   反向代理、认证头改写、重试分类、流式透传
state/   key 状态持久化，只保存 key 哈希标识
web/     React + Vite 管理台，dist 由 Go embed 打进二进制
```

## 排障

查看当前可用性：

```powershell
Invoke-RestMethod http://127.0.0.1:18081/admin/health
```

查看 key 池状态：

```powershell
Invoke-RestMethod http://127.0.0.1:18081/admin/status
```

请求返回 `503` 的常见原因：

- 当前 `active_provider` 下没有 active key。
- 所有 key 都在 cooling，且 `wait_for_key_timeout_ms` 内没有 key 恢复。
- 所有 key 都被标记 invalid。
- 上游地址、DNS、TLS 或网络不可用。
- 当前 provider 配置错了，但还没成功 reload。

配置修改没生效时：

- 确认启动参数 `-config` 指向的就是你修改的文件。
- 调用 `POST /admin/reload` 看是否有错误。
- `listen`、`admin_listen`、日志和状态持久化相关字段需要重启。

余额不足类错误反复出现时：

- ModelMux 会把命中的 key 标为 invalid 并换 key。
- 如果仍失败，通常是当前 provider 下所有可用 key 都额度不足，或它们共享同一个账户余额。

## 安全提醒

- `admin_listen` 默认是 `127.0.0.1:18081`，不要轻易改成公网监听。
- 不要提交 `config.json`、`state.json`、日志文件或任何真实 API key。
- 不要在日志、截图、issue、PR 里暴露完整 API key。
- 如果必须远程访问管理台，请额外加网络隔离、认证或反向代理鉴权。
