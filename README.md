# claude-key-proxy

一个面向 Claude 兼容 API 的本地反向代理。它把多个上游 API Key 统一收口成一个本地入口，对客户端透明地做 key 轮换、限速冷却和失效摘除。

项目目标很简单：

- 让 Claude Code 或其他 Anthropic-compatible 客户端只连接一个本地地址
- 自动在多个 key 之间轮询
- 遇到 `429` 自动切换 key 并冷却当前 key
- 遇到 `401` 自动标记 key 失效，避免继续使用
- 保持纯标准库实现，无第三方依赖

## 特性

- 纯 Go 标准库实现，无外部依赖
- 单二进制运行
- 双 HTTP 服务
  - 代理服务，默认 `:8080`
  - 管理服务，默认 `:8081`
- Round-robin key 轮换
- key 状态机
  - `active`
  - `cooling`
  - `invalid`
- 支持 `Retry-After`
- SSE 流式响应透传，不做缓冲聚合
- JSON 配置文件
- 支持热重载 key 列表
- 结构化日志，支持 `text` 和 `json`

## 适用场景

- 你有多个 Claude 兼容提供商 key，希望均匀消耗
- 你只想在本地改一个 `ANTHROPIC_BASE_URL`
- 你需要让 Claude Code 继续使用兼容接口，但不希望自己管理 key 故障切换

## 非目标

- 不做多上游路由
- 不做请求内容改写
- 不做持久化状态存储
- 不做 Web UI
- 不做外部依赖接入

## 工作原理

客户端始终请求本地代理。

代理对每个请求执行以下流程：

1. 从内存 key 池中选出下一个可用 key
2. 构造发往上游的请求
3. 重写认证头
   - `Authorization: Bearer <key>`
   - `X-Api-Key: <key>`
4. 转发到上游 `target_url`
5. 根据上游响应做处理
   - `200-399`：直接回传
   - `429`：当前 key 进入冷却并换下一个 key 重试
   - `401`：当前 key 标记失效并换下一个 key 重试
   - 其他错误：直接返回给客户端

请求体会在进入重试流程前读入内存，以保证 `401/429` 后重试仍然携带完整 body。

## 目录结构

```text
claude-key-proxy/
├── admin/
│   └── handler.go
├── config/
│   └── config.go
├── pool/
│   ├── key.go
│   ├── pool.go
│   └── pool_test.go
├── proxy/
│   ├── handler.go
│   └── handler_test.go
├── main.go
├── config.example.json
├── config.json
├── DESIGN.md
├── Makefile
└── README.md
```

## 环境要求

- Go 1.26.3 或兼容版本
- 一个可用的 Claude 兼容上游地址
- 至少一个可用 key

## 快速开始

### 1. 准备配置

复制示例配置：

```bash
cp config.example.json config.json
```

编辑 `config.json`：

```json
{
  "listen": ":8080",
  "admin_listen": ":8081",
  "target_url": "https://your-provider.com",
  "keys": [
    "sk-your-key-1",
    "sk-your-key-2",
    "sk-your-key-3"
  ],
  "cooling_seconds": 60,
  "max_retries": 3,
  "request_timeout_seconds": 120,
  "log_level": "info",
  "log_format": "text"
}
```

### 2. 编译

```bash
make build
```

或者直接：

```bash
go build -trimpath -ldflags="-s -w" -o claude-key-proxy .
```

### 3. 运行

```bash
make run
```

或者：

```bash
./claude-key-proxy -config config.json
```

启动后你会看到两类日志：

- 代理服务监听地址
- 管理服务监听地址

## 配置 Claude Code

如果你使用 Claude Code，只需要把请求地址改到本地代理。

示例：

```bash
export ANTHROPIC_BASE_URL=http://127.0.0.1:8080
export ANTHROPIC_API_KEY=dummy
claude
```

说明：

- 本地代理会忽略客户端传入的占位 key，并替换成池中的真实 key
- `ANTHROPIC_API_KEY` 这里只需要是任意非空值，便于客户端完成自身校验

Windows PowerShell 示例：

```powershell
$env:ANTHROPIC_BASE_URL='http://127.0.0.1:8080'
$env:ANTHROPIC_API_KEY='dummy'
claude
```

## 配置项说明

| 字段 | 是否必填 | 默认值 | 说明 |
|------|----------|--------|------|
| `listen` | 否 | `:8080` | 代理服务监听地址 |
| `admin_listen` | 否 | `:8081` | 管理服务监听地址 |
| `target_url` | 是 | 无 | 上游 Claude 兼容服务地址 |
| `keys` | 是 | 无 | key 列表，至少 1 个 |
| `cooling_seconds` | 否 | `60` | 上游返回 `429` 且未提供 `Retry-After` 时的冷却秒数 |
| `max_retries` | 否 | `3` | 单次请求最大重试次数 |
| `request_timeout_seconds` | 否 | `120` | 上游请求超时秒数 |
| `log_level` | 否 | `info` | `debug` / `info` / `warn` / `error` |
| `log_format` | 否 | `text` | `text` 或 `json` |

### 默认值行为

配置加载时会自动补齐以下默认值：

- `listen = :8080`
- `admin_listen = :8081`
- `cooling_seconds = 60`
- `max_retries = 3`
- `request_timeout_seconds = 120`
- `log_level = info`
- `log_format = text`

## Admin API

管理接口运行在单独端口，默认 `:8081`。

### `GET /admin/health`

返回当前 key 池健康情况。

可用 key 数量大于 0 时返回 `200`，否则返回 `503`。

示例：

```bash
curl http://127.0.0.1:8081/admin/health
```

响应示例：

```json
{
  "active_keys": 3,
  "total_keys": 5,
  "ok": true
}
```

### `GET /admin/status`

返回所有 key 的状态和统计信息。

示例：

```bash
curl http://127.0.0.1:8081/admin/status
```

响应示例：

```json
{
  "total_keys": 3,
  "active_keys": 2,
  "cooling_keys": 1,
  "invalid_keys": 0,
  "keys": [
    {
      "index": 0,
      "state": "active",
      "req_count": 142,
      "err_count": 0,
      "avg_latency_ms": 318.2,
      "cool_until": "0001-01-01T00:00:00Z"
    },
    {
      "index": 1,
      "state": "cooling",
      "req_count": 98,
      "err_count": 3,
      "avg_latency_ms": 401.7,
      "cool_until": "2026-05-23T10:15:00Z"
    }
  ]
}
```

字段说明：

- `state`
  - `active`：可正常使用
  - `cooling`：因 `429` 暂时冷却
  - `invalid`：因 `401` 被永久摘除，直到进程重启或 key 列表更新
- `req_count`：该 key 被选中请求的次数
- `err_count`：该 key 被标记冷却或失效的次数
- `avg_latency_ms`：平均上游耗时，按已成功记录的请求统计
- `cool_until`：冷却结束时间

### `POST /admin/reload`

重新读取配置文件并更新 key 列表。

示例：

```bash
curl -X POST http://127.0.0.1:8081/admin/reload
```

成功响应：

```json
{
  "ok": true
}
```

重要说明：

- 当前实现里，热重载只会更新 key 池内容
- 已存在且仍在新配置中的 key 会保留原有状态和统计信息
- 被删除的 key 会被移出池
- 新增 key 会以 `active` 状态加入
- 当前实现不会在热重载时重建代理 handler

这意味着以下配置项修改后不会立即生效，仍需要重启进程：

- `target_url`
- `request_timeout_seconds`
- `max_retries`
- `log_level`
- `log_format`
- `listen`
- `admin_listen`

## 构建与测试

### 常用命令

```bash
make build
make run
make test
```

### 单独运行 pool 测试

```bash
go test ./pool/ -run TestRoundRobin -v
```

### 跨平台编译

```bash
make build-linux
make build-windows
make build-mac
```

## 日志

日志通过 `log/slog` 输出到标准输出。

支持两种格式：

- `text`
- `json`

常见日志事件：

- 启动监听
- 请求代理成功
- key 进入冷却
- key 被标记失效
- 请求被客户端取消
- 所有 key 不可用

示例：

```text
time=2026-05-23T15:12:20.253+08:00 level=INFO msg="proxy listening" addr=:8080 target=https://iaigc.fun
time=2026-05-23T15:12:43.240+08:00 level=INFO msg=proxied path=/v1/messages status=200 attempt=1 latency_ms=2323
time=2026-05-23T15:12:43.782+08:00 level=INFO msg="request canceled" path=/v1/messages attempt=1
```

## 状态机说明

每个 key 有三种状态：

- `active`
- `cooling`
- `invalid`

状态流转：

```text
active --429--> cooling --cooldown expires--> active
active --401--> invalid
cooling --401--> invalid
```

行为说明：

- `cooling` key 不会被选中
- 冷却时间到达后，key 会在下次被检查时自动恢复为 `active`
- `invalid` key 不会自动恢复

## 错误处理策略

当前实现的重试策略如下：

- `429 Too Many Requests`
  - 标记当前 key 为 `cooling`
  - 优先使用上游 `Retry-After`
  - 然后切换下一个 key 重试
- `401 Unauthorized`
  - 标记当前 key 为 `invalid`
  - 切换下一个 key 重试
- 其他上游状态码
  - 直接透传给客户端
- 上游不可达
  - 返回 `502`
- 无可用 key
  - 返回 `503`

注意：

- 当前实现不会对一般网络错误做同 key 重试
- 这是当前代码的真实行为，不是设计文档中的预期行为

## SSE 和流式响应

代理对上游响应体按 4KB chunk 读取，并在每次写入后调用 `Flush()`。

这意味着：

- 不会把整个响应缓存完再返回
- 适合 Claude 流式输出
- 不应在这个代理层再引入额外缓冲逻辑

## 已知限制

### 1. 热重载范围有限

`/admin/reload` 目前只更新 key 池，不会热更新代理客户端和监听配置。

### 2. 请求体会完整读入内存

为了支持重试，代理会在处理请求前把 body 读入内存。对 Claude 常见请求通常可接受，但这意味着超大请求体会增加内存占用。

### 3. 仅支持单一上游

当前只支持一个 `target_url`。

### 4. 无持久化

key 状态和统计都在内存中，进程重启后重置。

### 5. 测试覆盖仍偏薄

当前已有：

- `pool` 的并发和状态测试
- `proxy` 的认证头和重试 body 测试

当前还没有：

- 端到端集成测试
- admin handler 测试
- 真正的 SSE 行为测试

## 故障排查

### Claude Code 连接不上代理

检查：

```bash
curl http://127.0.0.1:8081/admin/health
```

如果管理接口不通，先确认进程是否启动、监听端口是否正确。

### 请求一直返回 `503`

查看：

```bash
curl http://127.0.0.1:8081/admin/status
```

如果所有 key 都是 `invalid`：

- 上游大概率返回了 `401`
- 检查 key 是否真实可用
- 检查 `target_url` 是否是正确的 Claude 兼容接口

如果所有 key 都是 `cooling`：

- 上游可能在持续限流
- 等待冷却结束或增加 key 数量

### 修改配置后没生效

如果你改的是 key 列表，可以：

```bash
curl -X POST http://127.0.0.1:8081/admin/reload
```

如果你改的是以下内容，需要直接重启进程：

- `target_url`
- 超时
- 重试次数
- 日志级别
- 监听地址

### 上游流式响应中断

先看代理日志是否出现：

- `request canceled`
- `upstream unreachable`
- `key rate-limited, cooling`

如果客户端主动取消，代理会记录 `request canceled`，这不是服务端故障。

## 设计文档

项目设计说明见 [DESIGN.md](./DESIGN.md)。

`DESIGN.md` 是设计意图说明；实际行为请以代码和本 README 为准。

## 安全建议

- 不要把真实 key 提交到版本库
- 不要把带明文 key 的 `config.json` 发到公共环境
- 如果需要共享配置，使用 `config.example.json` 作为模板

## License

仓库当前未声明 License。如需开源分发，请补充明确的许可证文件。
