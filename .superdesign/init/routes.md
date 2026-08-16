# Routes — ModelMux Aurora Console

## Router config

Source: `web/src/App.tsx` — react-router-dom v6, `BrowserRouter` with basename from Vite `BASE_URL` (served at `/console/`). All routes are lazy-loaded; each is wrapped in `ErrorBoundary` and `PageTransition` (animation key = pathname).

```tsx
// From App.tsx (structure):
<ConsoleShell dashboard={dashboardQuery.data} dashboardLoading={dashboardQuery.isLoading}>
  <Suspense fallback={<RouteFallback />}>
    <PageTransition animationKey={location.pathname}>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/dashboard" element={<ErrorBoundary><DashboardPage /></ErrorBoundary>} />
        <Route path="/providers" element={<ErrorBoundary><ProvidersPage /></ErrorBoundary>} />
        <Route path="/stats" element={<ErrorBoundary><StatsPage /></ErrorBoundary>} />
        <Route path="/settings" element={<ErrorBoundary><SettingsPage /></ErrorBoundary>} />
        <Route path="/events" element={<ErrorBoundary><EventsPage /></ErrorBoundary>} />
        <Route path="/about" element={<ErrorBoundary><AboutPage /></ErrorBoundary>} />
        <Route path="*" element={<ErrorBoundary><NotFoundPage /></ErrorBoundary>} />
      </Routes>
    </PageTransition>
  </Suspense>
</ConsoleShell>
```

## Route table

| URL path | Page component | Layout | Summary |
|---|---|---|---|
| `/` | redirect → `/dashboard` | ConsoleShell | — |
| `/dashboard` | `pages/dashboard-page.tsx` → `DashboardPage` | ConsoleShell | 控制台总览：status card（active provider + health signals 可用/冷却/失效/Provider 计数）、provider list（每行：状态点 + badge、id + target_url、key 计数、切换/详情/删除操作）。轮询 5s（仅 tab 可见时）。 |
| `/providers` | `pages/providers-page.tsx` → `ProvidersPage` | ConsoleShell | Provider 管理：列表/表格 + 详情抽屉（key 管理、批量测试、导出导入）、新增/编辑 modal。支持 `?provider=<id>` 深链到详情。 |
| `/stats` | `pages/stats-page.tsx` → `StatsPage` | ConsoleShell | 调用统计：KPI summary cards、按模型统计表、时间线图（Recharts）、最近调用明细、日志查询。 |
| `/settings` | `pages/settings-page.tsx` → `SettingsPage` | ConsoleShell | 设置：分组表单（providers/keys、重试/超时、日志、持久化、统计），保存摘要 banner，热生效 vs 需重启标记。 |
| `/events` | `pages/events-page.tsx` → `EventsPage` | ConsoleShell | 事件流：过滤器（级别/分类）、表格 + 详情抽屉、自动刷新。 |
| `/about` | `pages/about-page.tsx` → `AboutPage` | ConsoleShell | 关于：版本、运行环境、备份导出（config/state 备份）。 |
| `*` | `pages/not-found-page.tsx` → `NotFoundPage` | ConsoleShell | 404 Result。 |

## Feature modules (per-route dependencies)

- `features/providers/` — provider-table.tsx, provider-detail-content.tsx, provider-modals.tsx, provider-key-status.ts, provider-utils.tsx, provider-types.ts, model-sync.ts
- `features/stats/` — stats-summary-card.tsx, stats-timeline-card.tsx (+css), stats-logs-card.tsx, stats-columns.tsx, stats-options.ts, stats-format.ts, stats-export.ts
- `features/settings/` — settings-group.tsx, settings-schema.tsx, settings-types.ts, save-summary-banner.tsx
- `features/events/` — events-columns.tsx, events-options.ts, events-utils.ts, event-detail.tsx

## API layer

- `api/http.ts` — fetch wrapper with auth header + JSON
- `api/admin.ts` — typed API functions (dashboard, providers, keys, settings, events, stats, backup)
- `api/events-stream.ts` — SSE stream (not currently wired to UI)
- `api/query-keys.ts` — TanStack Query key factory
- `types/admin.ts` — shared response types (`AdminDashboardResponse`, `AdminProviderSummary`, etc.)