# Pages & Dependency Trees — ModelMux Aurora Console

All pages render inside `ConsoleShell` (sidebar + sticky header + content), wrapped by `ErrorBoundary` + `PageTransition`, lazy-loaded via React.lazy.

## Shared dependencies (all pages)

- `src/app/console-shell.tsx` → `src/app/header-status.tsx`, `src/app/navigation.ts`, `src/app/theme-toggle.tsx`, `src/components/health-dot.tsx`
- `src/app/theme-mode.tsx` → `src/app/app-theme.ts`
- `src/components/` — format-time.ts, cooldown-text.tsx, use-countdown.ts, use-visibility-polling.ts, use-global-shortcuts.ts, error-boundary.tsx, page-transition.tsx (+css), shortcuts-help.tsx, health-dot.tsx
- `src/api/` — http.ts, admin.ts, query-keys.ts
- `src/types/admin.ts` — all response types
- `src/styles/*.css` — global stylesheet stack

## /dashboard (DashboardPage)

Entry: `src/pages/dashboard-page.tsx` (301 lines)

Dependencies:
- `src/api/admin.ts` (fetchDashboard, activateProvider, deleteProvider, triggerReload)
- `src/api/query-keys.ts`
- `src/components/format-time.ts` (formatClockShort)
- `src/components/health-dot.tsx`
- `src/components/use-visibility-polling.ts`
- `src/types/admin.ts`
- antd: Button, Card, Empty, Popconfirm, Result, Skeleton, Space, Typography, message

Renders: Status card (section heading "Dashboard / 控制台总览" + 更新于 + 立即重载/刷新 buttons; hero row = active provider panel + health strip 可用/冷却/失效/Provider) → Provider list card (`.provider-list` of `.provider-row` rows with tone variants `--active/--cooling/--invalid/--idle/--current`).

## /providers (ProvidersPage)

Entry: `src/pages/providers-page.tsx`

Dependencies:
- `src/features/providers/provider-table.tsx` → `provider-utils.tsx` (renderProviderState), `src/types/admin.ts`
- `src/features/providers/provider-detail-content.tsx` → `../../components/cooldown-text.tsx`, `../../components/format-time.ts`, `../stats/stats-format.ts`, `provider-key-status.ts` (isQuotaExhaustedKey), `provider-utils.tsx` (renderKeyState), `src/types/admin.ts`
- `src/features/providers/provider-modals.tsx` → `model-sync.ts` (buildModelSaveList, summarizeModelSync), `src/types/admin.ts`
- `src/features/providers/provider-types.ts`, `provider-key-status.ts`, `model-sync.ts`
- antd: Button, Card, Empty, Form, Modal, Result, Skeleton, Space, Typography, message

Renders: Provider table (name/URL/state/key counts/actions), detail Drawer with key management (status tags, cooldown countdown, reset/test/delete, batch append/replace/delete, export/import), create/edit provider Modal. Deep-link `?provider=<id>`.

## /stats (StatsPage)

Entry: `src/pages/stats-page.tsx`

Dependencies:
- `src/features/stats/stats-summary-card.tsx` → `stats-format.ts`, `stats-options.ts`, `src/types/admin.ts`
- `src/features/stats/stats-timeline-card.tsx` (+ stats-timeline-card.css) → `src/api/admin.ts` (fetchStatsTimeline), `src/api/query-keys.ts`, `src/components/use-visibility-polling.ts`, `stats-format.ts`, `stats-options.ts`, `src/types/admin.ts`
- `src/features/stats/stats-logs-card.tsx` → `stats-columns.tsx`, `stats-export.ts`, `stats-options.ts`, `src/types/admin.ts`
- `src/features/stats/stats-columns.tsx` → `stats-format.ts`, `src/components/format-time.ts`
- `src/features/stats/stats-format.ts`, `stats-options.ts`, `stats-export.ts`
- `src/components/charts/progress-bar.tsx` (+ css), donut-chart.css, mini-trend.css
- antd: Button, DatePicker, Result, Space, Spin; recharts

Renders: Window selector (Segmented/DatePicker) → KPI summary cards → timeline chart card → model stats table → recent calls logs card (status tags, latency, tokens, CSV export).

## /settings (SettingsPage)

Entry: `src/pages/settings-page.tsx`

Dependencies:
- `src/features/settings/settings-schema.tsx` (buildFieldRules, fieldToLabel)
- `src/features/settings/settings-group.tsx` → `settings-schema.tsx`, `settings-types.ts`
- `src/features/settings/save-summary-banner.tsx` → `settings-schema.tsx`, `settings-types.ts`
- `src/features/settings/settings-types.ts`
- antd: Button, Card, Collapse, Form, Result, Skeleton, Space, Typography, message

Renders: Collapse groups of settings forms (providers/keys, retry/timeouts, logging, persistence, stats), save button + save summary banner showing which fields hot-reload vs need restart.

## /events (EventsPage)

Entry: `src/pages/events-page.tsx`

Dependencies:
- `src/features/events/events-columns.tsx` → `src/components/format-time.ts`, `../stats/stats-format.ts`, `src/types/admin.ts`
- `src/features/events/events-options.ts`, `events-utils.ts`
- `src/features/events/event-detail.tsx` → `events-utils.ts`, `../stats/stats-format.ts`
- antd: Button, Card, Drawer, Empty, Input, Result, Select, Skeleton, Space, Switch, Table, Typography

Renders: Filter bar (level/category search, auto-refresh switch) → events Table → detail Drawer with expanded payload.

## /about (AboutPage)

Entry: `src/pages/about-page.tsx`

Dependencies:
- `src/api/admin.ts` (backup endpoints)
- antd: Button, Card, Result, Skeleton, Space, Typography, message; @ant-design/icons (DownloadOutlined, FileProtectOutlined, CloudServerOutlined, BranchesOutlined)

Renders: Version/env cards, config/state backup export buttons.

## /404 (NotFoundPage)

Entry: `src/pages/not-found-page.tsx` — antd Result 404 + 返回按钮.

## App shell entry

`src/App.tsx` — lazy routes + dashboard query + reload shortcut handler + global keyboard shortcuts; `src/main.tsx` — providers (AppThemeProvider → ConfigProvider zhCN + antd theme, QueryClientProvider, BrowserRouter basename `/console/`).