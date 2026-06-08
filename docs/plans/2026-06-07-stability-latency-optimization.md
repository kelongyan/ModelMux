# ModelMux Stability And Latency Optimization Plan

**Goal:** 在不改变 ModelMux 核心语义的前提下，提高代理稳定性，降低普通请求和故障场景下的尾延迟，并减少磁盘 I/O 对请求热路径的影响。

**Recommended Strategy:** 采用“保守热路径优化 + provider 级熔断”的分阶段方案。先处理确定收益高、行为变化小的问题，再引入可控的 provider 故障快速失败机制。

**Scope:** 后端代理、key/provider 状态、调用统计写入、故障恢复与验证体系。前端只在需要展示新增健康状态或统计指标时做最小改动。

---

## 1. 背景

当前版本已经具备比较清晰的运行边界：

- `proxy/` 负责请求转发、重试、流式透传和调用统计采集。
- `pool/` 负责 provider 内 key 池隔离、轮询、状态机和快照。
- `config/` 负责配置读取、热重载、原子写盘和失败回滚。
- `stats/` 负责调用明细 JSONL 持久化和查询聚合。
- `admin/` 负责管理 API、控制台静态资源和事件缓冲区。

现有设计的稳定性基础是好的：provider key 池隔离、runtime 原子快照、state hash-only 持久化、streaming 按 chunk flush。优化方案必须保留这些不变量。

## 2. 核心不变量

这些原则在所有阶段都不能破坏：

- 请求只使用 `active_provider`，不新增跨 provider 自动 failover。
- `proxy.Handler.runtime` 继续作为不可变快照放入 `atomic.Value`，热重载不能影响旧请求。
- SSE / streaming 继续边读边写边 flush，不增加响应缓冲。
- 401、429、余额不足类 403 只影响 key 状态。
- DNS、TLS、dial、502、503、504 等 provider 级故障不能毒化 key。
- `state.json` 继续只保存 key hash，不写入原始 key。
- 管理端配置写入继续走 `config.Manager.Update`，保留原子写盘和失败回滚。

## 3. 当前主要问题

### 3.1 调用统计同步写盘位于请求结束路径

`proxy.Handler.ServeHTTP` 在请求结束时调用 `recordCallStats`，当前 `stats.Store.Append` 每条记录都会：

1. 规范化记录。
2. `json.Marshal`。
3. `os.OpenFile`。
4. `Write`。
5. `Close`。
6. 更新内存 recent records。

这会让磁盘 I/O、文件句柄打开关闭和偶发杀毒/文件系统延迟进入请求生命周期。对流式请求而言是在流结束后影响收尾，对普通请求而言会直接扩大完整响应耗时。

### 3.2 上游连接复用参数偏保守

当前 `http.Transport` 设置了 `MaxIdleConns: 100`，但没有显式设置 `MaxIdleConnsPerHost`。Go 默认 per-host idle 连接数偏低，ModelMux 又通常只访问当前 active provider 一个 host，突发并发时容易重复建 TCP/TLS 连接。

### 3.3 重试响应没有显式 bounded drain

429、401、quota 403、502、503、504 等响应在进入重试前会关闭 body。如果不读取小体积响应体，连接复用机会可能下降。不能无界读取，否则错误响应体过大时会引入新的延迟风险。

### 3.4 provider 大故障时缺少快速失败

DNS、dial、TLS 或上游持续 502/503/504 时，每个请求都可能重新触发连接超时或响应头等待。即使 retry scope 已经区分 provider/key，当前仍缺少 provider 级健康状态来临时阻断明显不可用的上游。

### 3.5 stats 查询可能重复扫描 JSONL

统计页面定时查询 summary、models、logs。当前 `recordsSince` 从文件读取，数据量上来后可能出现查询开销上升。这个问题优先级低于代理热路径，但需要放入后续阶段。

## 4. 优先级总览

| 阶段 | 状态 | 优先级 | 主题 | 主要收益 | 行为风险 |
|---|---|---:|---|---|---|
| P0 | 已完成 | 最高 | 建立基线与专项测试 | 后续优化可量化，不靠感觉 | 低 |
| P1 | 已完成 | 最高 | 上游连接复用优化 | 降低建连/TLS 抖动和突发延迟 | 低 |
| P2 | 核心已完成 | 高 | stats 异步写入 | 移除请求热路径磁盘 I/O | 中 |
| P3 | 已完成 | 高 | 重试前 bounded drain | 提高连接复用，减少重试建连 | 低 |
| P4 | 已完成 | 高 | provider 级熔断 | 上游故障时快速失败，降低尾延迟 | 中 |
| P5 | 已完成 | 中 | stats 查询缓存与管理台展示 | 降低控制台统计查询成本 | 低 |
| P6 | 已完成 | 中 | 长期压测与运维可观测 | 防止回归，便于排障 | 低 |

## 5. 分阶段详细计划

## P0. 建立基线与专项测试

**目标:** 在改动前明确当前延迟、错误恢复和 I/O 行为，避免后续“优化了但不知道优化在哪”。

**改动范围:** 不改运行逻辑。新增测试、benchmark 或本地压测脚本。

**建议文件:**

- `proxy/handler_test.go`
- `stats/store_test.go`
- 可选新增 `proxy/benchmark_test.go`
- 可选新增 `stats/benchmark_test.go`
- 可选新增 `docs/plans/2026-06-07-stability-latency-optimization.md` 后续记录基线结果

**具体任务:**

1. 增加或整理代理请求 benchmark：
   - 单 key 成功请求。
   - 多 key 轮询请求。
   - 401 后换 key。
   - 429 后 cooling 并换 key。
   - 502/503/504 provider scope 重试。
   - streaming 响应持续 flush。
2. 增加 stats 写入 benchmark：
   - 单条 append。
   - 1000 条连续 append。
   - 并发 append。
3. 增加故障场景测试：
   - DNS/dial/TLS/provider transient 不能标记 key invalid。
   - provider transient retry 次数受 `max_transient_retries` 限制。
   - streaming 请求不受固定 request timeout 误杀。
4. 记录基线：
   - p50 / p95 / p99 完整请求耗时。
   - 首字节时间。
   - stats append 平均耗时。
   - provider 故障时单请求最坏耗时。

**验收标准:**

- `go test ./proxy ./stats ./pool ./config ./admin` 通过。
- benchmark 能稳定复现 stats 同步写盘和 provider 故障尾延迟。
- 记录优化前数据，作为后续阶段对照。

**风险与控制:**

- 不把 benchmark 结果写死为测试断言，避免不同机器误报。
- 功能测试只断言行为，不断言精确耗时。

### P0 本机短基线记录

记录时间：2026-06-07。

运行命令：

```powershell
go test ./proxy ./stats -run '^$' -bench 'BenchmarkServeHTTP|BenchmarkStore' -benchtime=100ms -count=1
```

运行环境：

- OS：Windows
- Arch：amd64
- CPU：12th Gen Intel(R) Core(TM) i5-12400

proxy 基线：

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkServeHTTPSuccess-12` | 116090 | 27551 | 226 |
| `BenchmarkServeHTTPSuccessWithStatsStore-12` | 349539 | 30432 | 239 |
| `BenchmarkServeHTTPUnauthorizedRetry-12` | 242829 | 32982 | 298 |
| `BenchmarkServeHTTPRateLimitRetry-12` | 221905 | 33861 | 299 |
| `BenchmarkServeHTTPProviderUnavailable-12` | 175884 | 27147 | 257 |
| `BenchmarkServeHTTPStreamingResponse-12` | 125711 | 28359 | 234 |

stats 基线：

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkStoreAppend-12` | 109399 | 1942 | 12 |
| `BenchmarkStoreAppendParallel-12` | 106998 | 1984 | 14 |
| `BenchmarkStoreSummarySince-12` | 52247000 | 18673200 | 170047 |
| `BenchmarkStoreModelsSince-12` | 53685500 | 18675760 | 170066 |
| `BenchmarkStoreQueryLogs-12` | 55211500 | 20173580 | 170053 |

观察：

- 当前 stats append 是同步 open/write/close JSONL，单条约 0.1ms；请求路径挂真实 `stats.Store` 后，成功请求基线从约 0.12ms 上升到约 0.35ms，P2 异步写入有明确优化空间。
- stats 查询 10k 记录时约 50ms、17 万次 allocation，P5 查询缓存或内存聚合有明确收益。
- provider unavailable 基线约 0.17ms 是本地 `httptest` 直接返回 503 的场景，不代表真实 DNS/TLS/dial 超时；P4 熔断仍需要针对真实 provider transport failure 降低尾延迟。

## P1. 上游连接复用优化

**目标:** 提高到 active provider 的连接复用能力，减少突发并发下 TCP/TLS 重连带来的延迟。

**建议改动:**

在 `proxy.newRuntimeConfig` 创建 `http.Transport` 时显式设置：

- `MaxIdleConns`
- `MaxIdleConnsPerHost`
- `MaxConnsPerHost` 可选，默认不限制或保守配置
- `ForceAttemptHTTP2: true`

建议先采用内置保守默认值，不急着暴露到 `config.json`：

- `MaxIdleConns: 256`
- `MaxIdleConnsPerHost: 64`
- `IdleConnTimeout: 90s` 保持现状
- `ForceAttemptHTTP2: true`

**建议文件:**

- `proxy/handler.go`
- `proxy/handler_test.go`

**测试重点:**

- `newRuntimeConfig` 能生成期望 transport 参数。
- 热重载后旧 transport 仍会 `CloseIdleConnections`。
- streaming 行为不变。

**验收标准:**

- `go test ./proxy -run TestRuntimeConfig -v` 通过。
- `go test ./proxy -run TestServeHTTPStreamingRequestIgnoresFixedClientTimeout -v` 通过。
- 手动压测突发请求时建连次数下降，p95/p99 更稳定。

**风险与控制:**

- HTTP/2 对部分非标准上游可能有兼容风险。若压测发现问题，保留配置开关 `upstream_http2_enabled`，默认开启或按 provider 配置覆盖。
- 不设置过低 `MaxConnsPerHost`，避免把并发限制误变成延迟来源。

### P1 实施记录

实施时间：2026-06-07。

实际落地参数：

- `MaxIdleConns: 256`
- `MaxIdleConnsPerHost: 64`
- `ForceAttemptHTTP2: true`
- `MaxConnsPerHost: 0`，不人为限制 active provider 上游并发
- 保持 `IdleConnTimeout: 90s`
- 保持 `TLSHandshakeTimeout` 使用 `connect_timeout_seconds`
- 保持 `ResponseHeaderTimeout` 使用 `response_header_timeout_seconds`

新增验证：

- `TestRuntimeConfigUsesOptimizedUpstreamConnectionPool`

运行命令：

```powershell
go test ./proxy -run "TestRuntimeConfigUsesOptimizedUpstreamConnectionPool|TestRuntimeConfigUsesSplitTimeouts|TestServeHTTPStreamingRequestIgnoresFixedClientTimeout|TestUpdateConfigChangesTargetForNewRequests" -v
go test ./proxy -run '^$' -bench 'BenchmarkServeHTTPSuccess|BenchmarkServeHTTPStreamingResponse' -benchtime=100ms -count=1
```

短 benchmark 结果：

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkServeHTTPSuccess-12` | 104882 | 27160 | 226 |
| `BenchmarkServeHTTPSuccessWithStatsStore-12` | 389368 | 28974 | 238 |
| `BenchmarkServeHTTPStreamingResponse-12` | 128069 | 28121 | 235 |

观察：

- 这次主要提升真实上游场景的连接复用能力；本地 `httptest` benchmark 对 TCP/TLS 复用收益不敏感，只作为无回归 smoke。
- 后续若发现个别上游 HTTP/2 兼容异常，再加 provider 级 `upstream_http2_enabled` 开关。

## P2. stats 异步写入

**目标:** 把调用统计持久化从请求关键路径移到后台，减少磁盘 I/O 对代理延迟的影响。

**推荐设计:**

保持 `proxy` 对 `statsRecorder.Append(record)` 的调用方式不变，但把 `stats.Store` 内部改造成有界队列 + 后台 writer。

### 设计细节

1. `Append(record)` 只做轻量规范化和入队：
   - 补齐 `At`、`ID`、`Attempts`、`UsageSource`。
   - 使用 non-blocking 或短超时 enqueue。
   - 队列满时丢弃 stats 记录，不阻塞代理请求。
2. 后台 writer goroutine：
   - 持有当前日期 JSONL 文件句柄。
   - 批量写入记录。
   - 定时 flush。
   - 日期变化时关闭旧文件并打开新文件。
   - 触发过期文件清理。
3. recent records 更新策略：
   - 入队成功后即可更新内存 recent records，保证控制台能尽快看到。
   - 或 writer 成功落盘后更新 recent records，保证内存只反映已持久化记录。
   - 推荐前者：控制台实时性更好，且 stats 本身不是强一致业务数据。
4. shutdown：
   - 给 `stats.Store` 增加 `Close() error`。
   - `main.go` 退出时关闭 stats store，尽量 flush 队列。
5. 丢弃记录观测：
   - 增加 dropped counter。
   - 管理事件记录 `stats.queue_dropped`，但做节流，避免队列满时刷爆日志。

### 配置建议

先使用默认值，不立即暴露配置：

- 队列长度：`4096`
- 批量大小：`128`
- flush 间隔：`1s`
- enqueue 策略：队列满直接丢弃并计数

如后续需要，可再增加：

- `stats_queue_size`
- `stats_flush_interval_ms`

**建议文件:**

- `stats/store.go`
- `stats/store_test.go`
- `stats/query.go`
- `main.go`
- 可选 `admin/api.go`
- 可选 `types/admin.ts` 和 stats 页面展示 dropped 计数

**测试重点:**

- `Append` 在队列未满时返回快，记录最终写入 JSONL。
- `Close` 会 flush 队列。
- 日期变化会轮转文件。
- 队列满时不阻塞，dropped counter 增加。
- 最近记录数量仍受 `maxRecentRecords` 限制。
- stats disabled 时行为不变。

**验收标准:**

- `go test ./stats ./proxy ./admin ./...` 通过。
- 高并发 append benchmark 中，`Append` 耗时明显低于同步 open/write/close。
- 代理请求在 stats 写盘慢时仍能返回。

**风险与控制:**

- 异步写入意味着进程异常退出时可能丢少量统计记录。接受这个取舍，因为请求稳定性优先于统计完整性。
- 必须实现 `Close()`，正常退出尽量 flush。
- 队列满不阻塞代理请求，这是明确策略。

### P2 实施记录

实施时间：2026-06-07。

已完成：

- `stats.Store.Append` 改为规范化记录后进入有界队列，队列满时丢弃统计记录并增加 dropped counter，不阻塞代理请求。
- 后台 writer 持有按日期轮转的 JSONL 文件句柄，定时 flush，并在日期变化时切换文件。
- 新增 `Flush()` 与 `Close()`，`Close()` 会 flush 队列并关闭当前文件。
- `main.go` shutdown 时关闭 stats store，测试和 benchmark 中也补齐 `Close()`，避免 Windows 下临时文件被后台 writer 持有。
- recent records 在入队成功后更新，控制台 recent 数据保持即时可见。

保留到后续阶段：

- `stats.queue_dropped` 管理事件节流与控制台展示，放到 P5/P6 的可观测性工作里处理。

短 benchmark 结果：

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkStoreAppend-12` | 418.7 | 198 | 4 |
| `BenchmarkStoreAppendParallel-12` | 510.3 | 248 | 6 |

## P3. 重试前 bounded drain

**目标:** 在需要重试前读取并丢弃小体积上游错误响应体，提高连接复用机会，同时不让大错误体拖慢代理。

**推荐设计:**

新增 helper，例如：

- `drainAndClose(resp.Body, limit int64)`
- 默认 limit 使用 `64 KiB`

调用位置：

- 429
- 401
- retryable provider status：502、503、504
- quota 403 判断后会重试的分支

注意：

- 非重试响应不走 drain，继续正常透传给客户端。
- quota 403 已经读取前缀用于判断，需要避免重复消费逻辑混乱。
- drain 失败不影响重试，只记录 debug 或忽略。

**建议文件:**

- `proxy/handler.go`
- `proxy/handler_test.go`

**测试重点:**

- 429/401/502 响应体小于 limit 时被读取。
- 响应体大于 limit 时最多读取 limit，不阻塞到完整读取。
- quota 403 仍能正确分类并重试。
- 非 quota 403 仍透传给客户端。

**验收标准:**

- `go test ./proxy -run "Retry|Quota|Forbidden|Drain" -v` 通过。
- 重试场景下连接复用能力提升，不引入额外大响应体延迟。

**风险与控制:**

- 不能为了连接复用无界读取 body。
- drain helper 必须小而明确，避免和 usage capture / quota inspect 逻辑纠缠。

### P3 实施记录

实施时间：2026-06-07。

实际落地：

- 新增 `upstreamRetryDrainBytes: 64 KiB`。
- 新增 `drainAndClose(body, limit)`，最多读取 `limit` 字节后关闭 body。
- 在 429、401、502、503、504 重试分支返回前执行 bounded drain。
- 在 quota 403 分类确认需要换 key 重试后，对剩余响应体执行 bounded drain。
- 非 quota 403 仍使用 `readResponsePrefix` 的 replay body 透传给客户端，行为不变。

新增验证：

- `TestServeHTTPDrainsRetryableStatusBodyBeforeRetry`
- `TestServeHTTPDrainsQuotaForbiddenBodyBeforeRetry`
- `TestDrainAndCloseReadsSmallBodyAndCloses`
- `TestDrainAndCloseLimitsLargeBodyAndCloses`

运行命令：

```powershell
go test ./proxy -run "TestServeHTTPDrainsRetryableStatusBodyBeforeRetry|TestServeHTTPDrainsQuotaForbiddenBodyBeforeRetry|TestDrainAndClose" -v
```

## P4. Provider 级熔断

**目标:** 当 active provider 明显不可用时，临时快速失败，避免每个请求都重新等待连接超时或网关错误，降低故障期间 p95/p99 延迟并保护本地代理。

**推荐设计:** 在 provider 维度维护轻量 health/circuit breaker，不改 key 池语义。

### 状态机

Provider circuit 状态：

- `closed`：正常转发。
- `open`：快速失败，返回 503。
- `half_open`：冷却到期后允许少量探测请求。

状态流转：

1. `closed` 下连续 provider-scope 失败达到阈值，进入 `open`。
2. `open` 到达 `open_until` 后进入 `half_open`。
3. `half_open` 只允许一个或少量探测请求。
4. 探测成功，回到 `closed` 并清空失败计数。
5. 探测失败，回到 `open`，冷却时间可指数退避到上限。

### 触发范围

只计入 provider-scope 错误：

- DNS failure
- dial failure
- TLS handshake/certificate failure
- 502
- 503
- 504

不计入：

- 401
- 429
- quota 403
- 非 quota 403
- 客户端取消
- stream 写客户端失败

### 默认参数

先使用内置默认，不急着暴露配置：

- 连续失败阈值：`3`
- 初始 open 冷却：`5s`
- 最大 open 冷却：`60s`
- half-open 并发探测：`1`

后续如需要再加入配置字段：

- `provider_circuit_enabled`
- `provider_circuit_failure_threshold`
- `provider_circuit_cooling_seconds`
- `provider_circuit_max_cooling_seconds`

### 响应行为

当 circuit open：

- 直接返回 503。
- 错误消息建议为 `proxy: active provider is temporarily unavailable`。
- 记录事件：
  - `provider.circuit_opened`
  - `provider.circuit_half_open`
  - `provider.circuit_closed`
  - `provider.circuit_rejected`

### 放置位置

推荐新增独立包或文件：

- 方案一：`proxy/circuit.go`
  - 优点：只服务代理运行时，边界简单。
  - 缺点：admin 若要展示状态，需要从 proxy 暴露 snapshot。
- 方案二：`pool/provider_health.go`
  - 优点：provider 状态和 pool 状态更集中。
  - 缺点：pool 会混入代理故障分类语义。

推荐方案一：放在 `proxy/`，因为熔断依据来自转发结果和 retry scope，属于代理行为。

**建议文件:**

- `proxy/circuit.go`
- `proxy/circuit_test.go`
- `proxy/handler.go`
- `proxy/handler_test.go`
- `logx/logx.go`
- 可选 `admin/api.go`
- 可选 `web/src/types/admin.ts`
- 可选 `web/src/pages/dashboard-page.tsx`

**测试重点:**

- 连续 provider failures 后进入 open。
- open 状态下请求不触达上游，直接 503。
- key-scope 错误不会打开 circuit。
- half-open 成功后关闭 circuit。
- half-open 失败后重新 open。
- circuit 状态不持久化到 `state.json`。
- 热重载 active provider 后 circuit 使用新 runtime 快照，不影响旧请求。

**验收标准:**

- provider 故障场景下，熔断打开后请求延迟从连接/响应头超时级别降为本地快速 503。
- `go test ./proxy ./pool ./admin ./...` 通过。
- `go test -race ./proxy ./pool` 通过。

**风险与控制:**

- 阈值过低会误伤短暂抖动，默认连续 3 次失败再 open。
- 不做跨 provider failover，避免改变产品语义。
- half-open 并发必须限制，否则冷却到期瞬间可能又打爆上游。

### P4 实施记录

实施时间：2026-06-07。

实际落地：

- 新增 `proxy/circuit.go`，provider circuit 状态为 `closed`、`open`、`half_open`。
- 默认连续 provider-scope 失败阈值为 `3`，初始 open 冷却 `5s`，最大冷却 `60s`，half-open 并发探测数 `1`。
- 只对 `retryScopeProvider` 计入 provider circuit，包括 DNS/dial/TLS/provider gateway 类失败和 502/503/504；401、429、quota 403、非 quota 403、connection-scope failure 和 stream 写客户端失败不打开 circuit。
- circuit open 后，新请求在进入上游前本地返回 503，消息为 `proxy: active provider is temporarily unavailable`。
- half-open 探测成功后关闭 circuit 并清空失败计数；half-open 探测失败后重新 open，并按冷却时间退避到上限。
- half-open 探测遇到 connection-scope、客户端取消等中性结果时释放探测槽位，不把 circuit 卡死在 half-open。
- 热重载会生成新的 runtime circuit；旧 runtime 上的 circuit 状态不污染新 active provider。
- 新增事件：
  - `provider.circuit_opened`
  - `provider.circuit_half_open`
  - `provider.circuit_closed`
  - `provider.circuit_rejected`

新增验证：

- `TestProviderCircuitOpensRejectsAndHalfOpenCloses`
- `TestProviderCircuitLimitsHalfOpenProbeConcurrency`
- `TestProviderCircuitHalfOpenFailureReopensWithBackoff`
- `TestProviderCircuitNeutralOutcomeReleasesHalfOpenProbe`
- `TestServeHTTPProviderCircuitOpensAndRejectsRequests`
- `TestServeHTTPKeyScopeErrorsDoNotOpenProviderCircuit`
- `TestUpdateConfigUsesFreshProviderCircuit`

短 benchmark 结果：

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkServeHTTPProviderUnavailable-12` | 9845 | 8927 | 64 |

观察：

- 该 benchmark 在 circuit 打开后主要测本地快速拒绝路径，因此与 P0 的 provider unavailable 基线不再是同一语义；它用于确认 P4 打开后的快速失败开销。
- Dashboard 展示 provider circuit 当前状态留到 P5，当前阶段先通过事件流暴露打开、half-open、关闭和拒绝请求。

## P5. stats 查询缓存与控制台展示

**目标:** 降低统计页面定时查询对 JSONL 文件扫描的压力，并把新增健康信号展示给用户。

**推荐设计:**

1. stats 查询短 TTL 缓存：
   - 按 `window + model + status + page + page_size` 缓存。
   - TTL 建议 `2s` 到 `5s`。
   - `Append` 后可以不立即失效，接受几秒延迟。
2. 内存聚合增强：
   - 如果 JSONL 规模继续增大，再考虑按时间窗口维护内存 aggregate。
   - 第一版不直接上复杂聚合桶。
3. 控制台展示：
   - Dashboard 增加 provider circuit 状态。
   - Events 页面展示熔断打开/关闭/拒绝事件。
   - Stats 页面可展示 dropped stats count。

**建议文件:**

- `stats/query.go`
- `stats/logs.go`
- `stats/query_test.go`
- `admin/api.go`
- `web/src/types/admin.ts`
- `web/src/pages/dashboard-page.tsx`
- `web/src/pages/events-page.tsx`
- `web/src/pages/stats-page.tsx`

**测试重点:**

- 缓存命中时不重复扫描文件。
- TTL 到期后重新查询。
- 查询结果分页和过滤不变。
- 前端构建通过。

**验收标准:**

- `go test ./stats ./admin` 通过。
- `Push-Location web; npm run build; Pop-Location` 通过。
- 控制台不会因为 stats 文件变大而明显卡顿。

**风险与控制:**

- 不为了缓存牺牲太多实时性；统计本来允许秒级延迟。
- 第一版只做短 TTL，不做复杂时序数据库。

### P5 实施记录

实施时间：2026-06-08。

实际落地：

- `stats.Store.recordsSince` 增加 2s 短 TTL 缓存，Summary、Models、Logs 共用同一批文件扫描结果。
- 缓存按查询 since 所在 TTL bucket 扫描，再按原始 since 做内存过滤，避免因为 bucket 对齐改变查询边界。
- `Append` 后不主动失效缓存，接受最多 2s 的统计页面延迟，以减少控制台轮询重复扫描 JSONL。
- `proxy.Handler` 新增 `ProviderCircuitSnapshot()`，向管理端暴露 active provider 的 circuit 状态、连续失败数、half-open 探测数和当前冷却秒数。
- Dashboard API 新增 `provider_circuit` 和 `stats` 健康摘要，`stats` 包含 `enabled` 与 `dropped_records`。
- Stats summary API 新增 `dropped_records`，Stats 页面直接展示统计队列丢弃计数。
- Dashboard 展示 provider 熔断状态与统计丢弃计数；Events 页面沿用通用事件流，无需专项改动即可展示 P4 新增的熔断事件。
- 前端生产资源已重新构建并嵌入新 `modelmux.exe`，运行服务已重启。

新增验证：

- `TestStoreSummarySinceUsesShortTTLCache`
- `TestStoreQueryLogsUsesShortTTLCacheAndKeepsFilters`
- `TestDashboardIncludesProviderCircuitAndStatsHealth`
- `TestStatsSummaryIncludesDroppedRecords`

运行命令：

```powershell
go test ./stats ./admin -run "Summary|Models|Logs|DashboardIncludesProviderCircuit|DroppedRecords" -v
go test ./...
go vet ./...
go test -race ./...
Push-Location web; npm run build; Pop-Location
go build -trimpath -ldflags="-s -w" -o modelmux.exe .
```

短 benchmark 结果：

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkStoreSummarySince-12` | 1497068 | 1248911 | 1393 |
| `BenchmarkStoreModelsSince-12` | 1010472 | 1229172 | 1021 |
| `BenchmarkStoreQueryLogs-12` | 1535759 | 2312594 | 1439 |

观察：

- P0 中 10k 记录 stats 查询约 52ms 到 55ms；P5 缓存命中后降到约 1ms 到 1.5ms。
- 该 benchmark 主要衡量同窗口轮询场景下的缓存命中收益，不代表 TTL 过期后的冷查询成本。
- 嵌入版 `/console/dashboard` 已用 Playwright 检查桌面与移动布局，Dashboard 显示 `熔断 正常` 和 `统计丢弃 0 条`。

## P6. 长期压测与运维可观测

**目标:** 把稳定性优化变成可持续维护的能力，而不是一次性改动。

**建议内容:**

1. 增加本地压测说明：
   - 普通请求压测。
   - streaming 请求压测。
   - provider 502/503/504 故障压测。
   - stats 高并发写入压测。
2. 增加关键指标：
   - provider circuit 状态。
   - stats queue length。
   - stats dropped records。
   - active provider 的 provider-scope failure count。
3. 增加事件节流：
   - 熔断拒绝请求不应每次都刷一条高成本日志。
   - stats dropped 事件需要聚合或节流。

**建议文件:**

- `README.md` 可选，用户明确要求后再改。
- `docs/plans/` 或后续专门运维文档。
- `admin/events.go`
- `logx/logx.go`

**验收标准:**

- 关键故障能从 `/admin/api/v1/events` 看出来。
- 压测和故障恢复有固定命令可复现。
- 不新增高成本日志噪声。

### P6 实施记录

实施时间：2026-06-08。

实际落地：

- `stats.Store` 新增 `QueueDepth()` 与 `QueueCapacity()`，用于展示异步 stats writer 当前排队长度与队列容量。
- Dashboard API 的 `stats` 摘要新增 `queue_depth`、`queue_capacity`；Stats summary API 同步新增这两个字段。
- Dashboard 将 stats 状态展示为 `队列深度/队列容量 · 丢弃数`；Stats 页面顶部也展示同一组指标。
- proxy 在发现 stats dropped count 增长时发出 `stats.queue_dropped` 事件，事件 data 包含：
  - `dropped_records`
  - `dropped_delta`
  - `queue_depth`
  - `queue_capacity`
- `stats.queue_dropped` 事件按 30s 窗口节流，窗口内继续增长的 dropped count 会在下一条事件中聚合为 delta。
- `provider.circuit_rejected` 改为 circuit 内部节流聚合，不再对 open 状态下每个本地拒绝请求都发事件；事件 data 新增：
  - `rejected_count`
  - `rejected_delta`
- provider circuit snapshot 已在 P5 暴露，`consecutive_failures` 作为 active provider 的 provider-scope failure count。

固定复现命令：

```powershell
# 普通请求与 key 轮换基准
go test ./proxy -run '^$' -bench 'BenchmarkServeHTTPSuccess|BenchmarkServeHTTPUnauthorizedRetry|BenchmarkServeHTTPRateLimitRetry' -benchtime=100ms -count=1

# streaming 响应基准
go test ./proxy -run '^$' -bench 'BenchmarkServeHTTPStreamingResponse' -benchtime=100ms -count=1

# provider 502/503/504 与 circuit 快速失败基准
go test ./proxy -run '^$' -bench 'BenchmarkServeHTTPProviderUnavailable' -benchtime=100ms -count=1
go test ./proxy -run 'ProviderCircuit|TestServeHTTPProviderCircuit' -v

# stats 高并发写入与查询缓存基准
go test ./stats -run '^$' -bench 'BenchmarkStoreAppendParallel|BenchmarkStoreSummarySince|BenchmarkStoreQueryLogs' -benchtime=100ms -count=1

# 运维观测接口
Invoke-RestMethod -Uri 'http://127.0.0.1:18081/admin/api/v1/dashboard' | ConvertTo-Json -Depth 6
Invoke-RestMethod -Uri 'http://127.0.0.1:18081/admin/api/v1/events?limit=200' | ConvertTo-Json -Depth 6
```

新增验证：

- `TestStoreReportsQueueDepthAndCapacity`
- `TestDashboardIncludesProviderCircuitAndStatsHealth` 扩展 queue 指标断言
- `TestStatsSummaryIncludesDroppedRecords` 扩展 queue 指标断言
- `TestRecordCallStatsEmitsThrottledDroppedStatsEvent`
- `TestProviderCircuitThrottlesRejectedEvents`

短 benchmark 结果：

| Benchmark | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| `BenchmarkServeHTTPProviderUnavailable-12` | 4029 | 7444 | 39 |
| `BenchmarkServeHTTPStreamingResponse-12` | 125233 | 28222 | 235 |
| `BenchmarkStoreAppendParallel-12` | 461.5 | 274 | 6 |
| `BenchmarkStoreSummarySince-12` | 1195872 | 1212904 | 901 |
| `BenchmarkStoreQueryLogs-12` | 1480111 | 2250055 | 984 |

观察：

- provider circuit 打开后的本地快速失败路径仍保持微秒级开销。
- stats append 并发路径保持非阻塞队列写入。
- stats 查询 benchmark 仍主要反映缓存命中场景，冷查询成本由 JSONL 文件大小决定。
- 关键故障现在可以通过 `/admin/api/v1/events` 看到，Dashboard 同时展示 circuit 状态、provider-scope 连续失败数、stats 队列深度和 dropped count。

## 6. 建议实施顺序

严格按下面顺序推进：

1. P0：先补基线测试和 benchmark。
2. P1：连接复用优化。
3. P2：stats 异步写入。
4. P3：bounded drain。
5. P4：provider circuit breaker。
6. P5：stats 查询缓存和控制台展示。
7. P6：长期压测与可观测补强。

如果时间有限，第一轮建议只做 P0 到 P3。它们收益明确，行为风险低。P4 是稳定性提升的关键，但需要更认真审核默认阈值和 half-open 行为。

## 7. 验证矩阵

| 场景 | 必跑命令 | 目的 |
|---|---|---|
| 后端基础回归 | `go test ./...` | 确认所有包行为不回归 |
| 静态检查 | `go vet ./...` | 捕获明显 Go 问题 |
| 并发/状态变更 | `go test -race ./proxy ./pool ./stats` | 检查异步 stats 和 circuit 并发安全 |
| focused proxy | `go test ./proxy -run "Retry|Stream|Provider|Circuit|Drain" -v` | 验证代理关键路径 |
| focused stats | `go test ./stats -run "Store|Summary|Models|Logs" -v` | 验证 JSONL 写入与查询 |
| 前端变更 | `Push-Location web; npm run build; Pop-Location` | 验证控制台类型和构建 |
| 嵌入 UI | `go build -trimpath -ldflags="-s -w" -o modelmux.exe .` | 验证 web/dist 能嵌入二进制 |

## 8. 成功指标

优化完成后，至少应满足：

- 普通请求 p95/p99 比基线更稳定，突发并发下连接重建减少。
- stats 写盘慢或 stats 文件变大时，代理请求不明显受影响。
- provider 持续不可用时，熔断打开后请求快速返回 503。
- key 级错误和 provider 级错误继续严格分离。
- streaming 响应仍按 chunk flush，首 token 和中间 token 不被缓冲。
- 热重载仍保持 runtime 快照语义，不影响旧请求。

## 9. 明确不做

本轮不做这些事情：

- 不做跨 provider 自动 failover。
- 不引入数据库。
- 不引入复杂任务调度系统。
- 不扩大 admin 默认监听范围。
- 不把 API key 明文写入 state 或日志。
- 不用提高 retry 次数来掩盖上游故障。
- 不给 proxy server 增加固定 `WriteTimeout`。

## 10. 审核决策点

实施前需要确认三个策略：

1. stats 队列满时是否允许丢统计记录。
   - 推荐：允许丢弃，优先保证代理请求稳定。
2. provider circuit breaker 是否默认启用。
   - 推荐：默认启用，但阈值保守，后续可加配置开关。
3. HTTP/2 是否默认启用。
   - 推荐：默认启用；如果发现上游兼容问题，再增加 provider 级关闭开关。

## 11. 第一轮建议交付范围

为了控制风险，第一轮建议交付：

- P0 基线测试与 benchmark。
- P1 Transport 连接复用优化。
- P2 stats 异步写入。
- P3 bounded drain。

第二轮再交付：

- P4 provider circuit breaker。
- P5 控制台展示和 stats 查询缓存。
- P6 压测与可观测补强。

这样拆分的好处是：第一轮主要减少延迟抖动，不改变故障策略；第二轮再引入 provider 快速失败，方便独立审核和回滚。
