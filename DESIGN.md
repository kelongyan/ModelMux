# Claude Key Proxy — 开发方案书

> 版本：v1.0 | 日期：2026-05-23 | 语言：Go

---

## 一、项目概述

### 背景

使用第三方 Claude 兼容 API 提供商时，单个账号存在请求频率或额度限制。通过在本地运行一个透明反向代理，将多个账号的 API Key 统一管理，对上层应用（Claude Code、自定义客户端等）完全透明，实现额度的均匀消耗。

### 目标

- 对调用方零感知：只需修改 `ANTHROPIC_BASE_URL`，无需改动任何业务代码
- 支持流式响应（SSE）透传，不引入额外延迟
- 自动处理限速（429）和失效 Key（401），保证请求成功率
- 单二进制部署，无外部依赖

---

## 二、技术选型

| 维度 | 选择 | 理由 |
|------|------|------|
| 语言 | **Go 1.22+** | 原生并发、单二进制、标准库 HTTP 反向代理支持完善 |
| HTTP 框架 | **标准库 `net/http`** | 无需第三方依赖，流式代理更可控 |
| 配置格式 | **JSON** | 简单直观，便于手动编辑 |
| 日志 | **`log/slog`**（结构化日志） | Go 1.21+ 内置，无需引入第三方 |
| 构建工具 | **Go Modules** | 标准工具链 |

---

## 三、系统架构

```
┌─────────────────────────────────────────────────────┐
│                   调用方（Claude Code）               │
│  ANTHROPIC_BASE_URL=http://localhost:8080            │
│  ANTHROPIC_API_KEY=any-placeholder                  │
└───────────────────────┬─────────────────────────────┘
                        │ HTTP/HTTPS 请求
                        ▼
┌─────────────────────────────────────────────────────┐
│                  claude-key-proxy                    │
│                                                     │
│  ┌─────────────┐    ┌──────────────┐               │
│  │  HTTP Server │───▶│ ProxyHandler │               │
│  │  :8080       │    │              │               │
│  └─────────────┘    └──────┬───────┘               │
│                            │                        │
│                     ┌──────▼───────┐               │
│                     │   KeyPool    │               │
│                     │              │               │
│                     │ ┌──────────┐ │               │
│                     │ │ key[0] ✓ │ │               │
│                     │ │ key[1] ✓ │ │               │
│                     │ │ key[2] ⏸ │ │  (冷却中)     │
│                     │ │ key[3] ✗ │ │  (已失效)     │
│                     │ └──────────┘ │               │
│                     └──────┬───────┘               │
│                            │ 替换 Authorization     │
└────────────────────────────┼────────────────────────┘
                             │
                             ▼
              ┌──────────────────────────┐
              │   第三方 API 提供商       │
              │   https://provider.com   │
              └──────────────────────────┘
```

---

## 四、核心模块设计

### 4.1 KeyPool — Key 池管理

**数据结构：**

```go
type KeyState int

const (
    StateActive  KeyState = iota // 可用
    StateCooling                 // 冷却中（触发 429）
    StateInvalid                 // 失效（触发 401）
)

type Key struct {
    Value     string
    State     KeyState
    CoolUntil time.Time   // 冷却结束时间
    ReqCount  atomic.Int64 // 累计请求数（统计用）
    ErrCount  atomic.Int64 // 累计错误数
}

type KeyPool struct {
    keys    []*Key
    cursor  atomic.Int64  // 当前轮询位置
    mu      sync.RWMutex  // 保护 keys 切片
}
```

**轮询算法（Round-Robin + 跳过不可用）：**

```
NextKey():
  尝试次数 = len(keys)
  for i in range(尝试次数):
    idx = (cursor + i) % len(keys)
    key = keys[idx]
    if key.State == Active:
      cursor = idx + 1
      return key
    if key.State == Cooling && now > key.CoolUntil:
      key.State = Active  // 自动恢复
      cursor = idx + 1
      return key
  return error("所有 key 均不可用")
```

**状态转换：**

```
Active ──(429)──▶ Cooling ──(冷却期满)──▶ Active
Active ──(401)──▶ Invalid
Cooling ──(401)──▶ Invalid
```

### 4.2 ProxyHandler — 请求代理

**处理流程：**

```
1. 接收请求
2. 从 KeyPool 获取可用 Key
3. 克隆请求，替换：
   - Host → 目标提供商域名
   - Authorization: Bearer <selected-key>
   - X-Forwarded-* 头（可选）
4. 发送到上游
5. 判断响应状态码：
   - 429 → 标记 Key 进入冷却，重试（最多 N 次）
   - 401 → 标记 Key 失效，重试
   - 其他错误 → 直接返回给调用方
6. 透传响应（含流式 SSE）
```

**流式响应透传：**

```go
// 关键：不缓冲，直接 flush
func streamResponse(w http.ResponseWriter, resp *http.Response) {
    flusher := w.(http.Flusher)
    // 复制响应头
    // io.Copy 逐块写入 + 每块后调用 flusher.Flush()
}
```

### 4.3 Config — 配置管理

**配置文件 `config.json`：**

```json
{
  "listen": ":8080",
  "target_url": "https://your-provider.com",
  "keys": [
    "sk-key1-xxxx",
    "sk-key2-xxxx",
    "sk-key3-xxxx"
  ],
  "cooling_seconds": 60,
  "max_retries": 3,
  "request_timeout_seconds": 120,
  "log_level": "info"
}
```

| 字段 | 说明 | 默认值 |
|------|------|--------|
| `listen` | 监听地址 | `:8080` |
| `target_url` | 上游提供商地址 | 必填 |
| `keys` | Key 列表 | 必填，至少 1 个 |
| `cooling_seconds` | 429 后冷却时长（秒） | `60` |
| `max_retries` | 单次请求最大重试次数 | `3` |
| `request_timeout_seconds` | 上游请求超时 | `120` |
| `log_level` | 日志级别 debug/info/warn/error | `info` |

### 4.4 Admin API — 状态查询接口

提供只读 HTTP 接口，方便监控 Key 状态：

| 路径 | 方法 | 说明 |
|------|------|------|
| `/admin/status` | GET | 查看所有 Key 状态、请求数、错误数 |
| `/admin/health` | GET | 健康检查，返回可用 Key 数量 |
| `/admin/reload` | POST | 热重载配置文件（不重启进程） |

**`/admin/status` 响应示例：**

```json
{
  "total_keys": 3,
  "active_keys": 2,
  "cooling_keys": 1,
  "invalid_keys": 0,
  "keys": [
    { "index": 0, "state": "active",  "req_count": 142, "err_count": 0 },
    { "index": 1, "state": "cooling", "req_count": 98,  "err_count": 3, "cool_until": "2026-05-23T10:15:00Z" },
    { "index": 2, "state": "active",  "req_count": 115, "err_count": 1 }
  ]
}
```

---

## 五、项目结构

```
claude-key-proxy/
├── main.go              # 入口：加载配置、启动服务
├── config/
│   └── config.go        # 配置结构体、加载、热重载
├── proxy/
│   ├── handler.go       # HTTP 反向代理处理器
│   └── stream.go        # SSE 流式响应透传
├── pool/
│   ├── pool.go          # KeyPool 核心逻辑
│   └── key.go           # Key 数据结构与状态机
├── admin/
│   └── handler.go       # Admin API 路由与处理
├── config.json          # 配置文件（用户编辑）
├── config.example.json  # 配置示例
├── go.mod
├── go.sum
├── Makefile             # build / run / test 快捷命令
└── DESIGN.md            # 本文档
```

---

## 六、开发阶段规划

### Phase 1 — 核心功能（MVP）

- [ ] 项目初始化，`go mod init`
- [ ] `config` 模块：JSON 配置加载与校验
- [ ] `pool` 模块：KeyPool 数据结构 + Round-Robin 轮询
- [ ] `proxy` 模块：基础反向代理（非流式）
- [ ] `proxy/stream`：SSE 流式透传
- [ ] `main.go`：组装启动
- [ ] 基础日志输出（请求日志、Key 切换日志）

**验收标准：** Claude Code 配置 `ANTHROPIC_BASE_URL` 后可正常对话，多 Key 均匀轮换。

### Phase 2 — 错误感知与稳定性

- [ ] 429 处理：Key 冷却 + 自动重试
- [ ] 401 处理：Key 标记失效 + 告警日志
- [ ] 所有 Key 不可用时的降级响应（返回 503 + 明确错误信息）
- [ ] 请求超时控制
- [ ] 并发安全测试

**验收标准：** 手动触发 429/401 场景，代理自动切换 Key，调用方无感知。

### Phase 3 — 可观测性

- [ ] `admin` 模块：`/admin/status`、`/admin/health`
- [ ] `/admin/reload` 热重载配置
- [ ] 结构化日志（JSON 格式可选）
- [ ] 请求耗时统计

### Phase 4 — 工程化

- [ ] `Makefile`：`make build`、`make run`、`make test`
- [ ] 跨平台编译（Windows/Linux/macOS）
- [ ] 配置文件路径支持命令行参数 `-config`
- [ ] 优雅关闭（`SIGTERM` 处理）

---

## 七、关键技术细节

### 7.1 并发安全

KeyPool 的 `cursor` 使用 `atomic.Int64`，避免锁竞争。Key 状态变更（冷却/失效）使用 `sync.RWMutex` 保护，读多写少场景性能最优。

### 7.2 流式响应（SSE）

Claude API 使用 `Transfer-Encoding: chunked` + `Content-Type: text/event-stream`。代理必须：
1. 不设置 `Content-Length`（流式长度未知）
2. 每写入一个 chunk 后立即调用 `http.Flusher.Flush()`
3. 上游连接断开时，同步关闭下游连接

### 7.3 重试与幂等性

重试仅在以下情况触发：
- `429 Too Many Requests`（限速，切换 Key 重试）
- `401 Unauthorized`（Key 失效，切换 Key 重试）
- 网络连接错误（同一 Key 重试一次）

**不重试**：`4xx`（除 429/401）、`5xx`（上游服务错误，直接透传）。

### 7.4 冷却期设计

冷却期基于 `Retry-After` 响应头（如果提供商返回了该头），否则使用配置的 `cooling_seconds`。冷却期满后 Key 自动恢复为 Active，无需手动干预。

---

## 八、使用方式

### 启动代理

```bash
# 编辑配置
cp config.example.json config.json
# 填入 target_url 和 keys

# 启动
./claude-key-proxy -config config.json
# 输出: [INFO] proxy started on :8080, active keys: 3
```

### 配置 Claude Code

```bash
export ANTHROPIC_BASE_URL=http://localhost:8080
export ANTHROPIC_API_KEY=placeholder
claude  # 正常使用，代理透明处理 Key 轮换
```

### 查看 Key 状态

```bash
curl http://localhost:8080/admin/status | jq
```

---

## 九、非目标（明确不做）

- 不做 Key 的自动注册/购买
- 不做请求内容的修改或过滤
- 不做多提供商路由（只支持单一 `target_url`）
- 不做 Web UI（Admin API 已足够）
- 不做持久化存储（状态内存维护，重启后重置）
