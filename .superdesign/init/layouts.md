# Layout Components — ModelMux Aurora Console

## ConsoleShell — app shell (sidebar + header + content)

Source: `web/src/app/console-shell.tsx`

```tsx
import { Layout } from "antd";
import { NavLink, useLocation } from "react-router-dom";

import type { AdminDashboardResponse } from "../types/admin";
import { HeaderStatus } from "./header-status";
import { navigationItems, type NavigationIconName } from "./navigation";

type ConsoleShellProps = {
  children: React.ReactNode;
  dashboard: AdminDashboardResponse | undefined;
  dashboardLoading: boolean;
};

export function ConsoleShell({ children, dashboard, dashboardLoading }: ConsoleShellProps): JSX.Element {
  const location = useLocation();

  return (
    <Layout className="console-shell">
      <aside className="console-sidebar">
        <ConsoleBrand />
        <nav className="console-nav" aria-label="主导航">
          {navigationItems.map((item) => {
            const selected = location.pathname === item.key;
            return (
              <NavLink
                key={item.key}
                to={item.key}
                className={selected ? "console-nav-link is-active" : "console-nav-link"}
              >
                <NavIcon name={item.icon} />
                <span className="console-nav-label">{item.label}</span>
              </NavLink>
            );
          })}
        </nav>
        <div className="console-sidebar-foot">
          <span className="console-sidebar-chip">LOCAL PROXY</span>
          <span className="console-sidebar-meta">Aurora Console</span>
        </div>
      </aside>
      <Layout className="console-main">
        <header className="console-header">
          <HeaderStatus data={dashboard} loading={dashboardLoading} />
        </header>
        <main className="console-content">
          {children}
        </main>
      </Layout>
    </Layout>
  );
}

function ConsoleBrand(): JSX.Element {
  return (
    <div className="console-brand">
      <div className="console-brand-mark" aria-hidden="true">
        <span className="console-brand-mark-core" />
      </div>
      <div className="console-brand-copy">
        <p className="console-kicker">ModelMux</p>
        <h1>控制台</h1>
        <p className="console-brand-sub">LOCAL PROXY · OPS</p>
      </div>
    </div>
  );
}

function NavIcon({ name }: { name: NavigationIconName }): JSX.Element {
  const common = {
    className: "console-nav-icon",
    viewBox: "0 0 24 24",
    fill: "none",
    stroke: "currentColor",
    strokeWidth: 1.7,
    strokeLinecap: "round" as const,
    strokeLinejoin: "round" as const,
    "aria-hidden": true,
  };

  switch (name) {
    case "dashboard":
      return (
        <svg {...common}>
          <rect x="3.5" y="3.5" width="7" height="7" rx="1.5" />
          <rect x="13.5" y="3.5" width="7" height="4.5" rx="1.5" />
          <rect x="13.5" y="10.5" width="7" height="10" rx="1.5" />
          <rect x="3.5" y="13" width="7" height="7.5" rx="1.5" />
        </svg>
      );
    case "providers":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="2.2" />
          <circle cx="5.5" cy="7" r="1.8" />
          <circle cx="18.5" cy="7" r="1.8" />
          <circle cx="5.5" cy="17" r="1.8" />
          <circle cx="18.5" cy="17" r="1.8" />
          <path d="M7.2 8.1 10.2 10.7M16.8 8.1 13.8 10.7M7.2 15.9 10.2 13.3M16.8 15.9 13.8 13.3" />
        </svg>
      );
    case "stats":
      return (
        <svg {...common}>
          <path d="M4 19V5" />
          <path d="M4 19h16" />
          <path d="M8 15l3.2-4.2 3 2.4L18 8" />
          <circle cx="18" cy="8" r="1.2" fill="currentColor" stroke="none" />
        </svg>
      );
    case "settings":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="3" />
          <path d="M12 3.8v2.2M12 18v2.2M4.9 6.4l1.6 1.5M17.5 16.1l1.6 1.5M3.8 12h2.2M18 12h2.2M4.9 17.6l1.6-1.5M17.5 7.9l1.6-1.5" />
        </svg>
      );
    case "events":
      return (
        <svg {...common}>
          <path d="M5 7h14M5 12h10M5 17h12" />
          <circle cx="18.5" cy="12" r="1.4" fill="currentColor" stroke="none" />
        </svg>
      );
    case "about":
      return (
        <svg {...common}>
          <circle cx="12" cy="12" r="8.2" />
          <path d="M12 10.4v5.2" />
          <circle cx="12" cy="7.6" r="0.9" fill="currentColor" stroke="none" />
        </svg>
      );
  }
}
```

## HeaderStatus — top bar with LIVE pill, health dot, key metrics, theme toggle

Source: `web/src/app/header-status.tsx`

```tsx
import type { AdminDashboardResponse } from "../types/admin";
import { HealthDot } from "../components/health-dot";
import { ThemeToggle } from "./theme-toggle";

type HeaderStatusProps = {
  data: AdminDashboardResponse | undefined;
  loading: boolean;
};

export function HeaderStatus({ data, loading }: HeaderStatusProps): JSX.Element {
  if (loading || !data) {
    return (
      <div className="console-header-status">
        <div className="header-status-left">
          <span className="header-live-pill header-live-pill--loading">
            <span className="header-live-dot" />
            SYNC
          </span>
          <span className="header-status-loading">正在加载运行状态…</span>
        </div>
        <ThemeToggle />
      </div>
    );
  }

  const state: "active" | "cooling" | "invalid" | "idle" =
    !data.active_provider ? "idle" :
    data.active_keys === 0 ? (data.cooling_keys > 0 ? "cooling" : "invalid") :
    data.cooling_keys > 0 || data.invalid_keys > 0 ? "cooling" : "active";

  const liveLabel =
    state === "active" ? "LIVE" :
    state === "cooling" ? "DEGRADED" :
    state === "invalid" ? "DOWN" :
    "IDLE";

  return (
    <div className="console-header-status">
      <div className="header-status-left">
        <span className={`header-live-pill header-live-pill--${state}`}>
          <span className="header-live-dot" />
          {liveLabel}
        </span>
        <HealthDot state={state} pulse={state === "active"} />
        <div className="header-status-provider-block">
          <span className="header-status-kicker">Active Provider</span>
          <strong className="header-status-provider" title={data.active_provider || "未配置"}>
            {data.active_provider || "未配置 Provider"}
          </strong>
        </div>
      </div>
      <div className="header-status-right">
        <div className="header-status-metrics" aria-label="Key 池状态">
          <span className="header-metric">
            <span className="header-metric-label">可用</span>
            <strong className="header-metric-value header-metric-value--ok">{data.active_keys}</strong>
          </span>
          <span className="header-metrics-sep" />
          <span className="header-metric">
            <span className="header-metric-label">冷却</span>
            <strong className="header-metric-value header-metric-value--warn">{data.cooling_keys}</strong>
          </span>
          <span className="header-metrics-sep" />
          <span className="header-metric">
            <span className="header-metric-label">失效</span>
            <strong className="header-metric-value header-metric-value--err">{data.invalid_keys}</strong>
          </span>
          <span className="header-metrics-sep" />
          <span className="header-metric">
            <span className="header-metric-label">Provider</span>
            <strong className="header-metric-value">{data.provider_count}</strong>
          </span>
        </div>
        <ThemeToggle />
      </div>
    </div>
  );
}
```

## Navigation config

Source: `web/src/app/navigation.ts`

```ts
export type NavigationItem = {
  key: string;
  label: string;
  icon: NavigationIconName;
};

export type NavigationIconName =
  | "dashboard"
  | "providers"
  | "stats"
  | "settings"
  | "events"
  | "about";

export const navigationItems: NavigationItem[] = [
  { key: "/dashboard", label: "总览", icon: "dashboard" },
  { key: "/providers", label: "提供商", icon: "providers" },
  { key: "/stats", label: "调用统计", icon: "stats" },
  { key: "/settings", label: "设置", icon: "settings" },
  { key: "/events", label: "事件", icon: "events" },
  { key: "/about", label: "关于", icon: "about" },
];
```

## ThemeToggle — light/dark switch (circular button, sun/moon SVG)

Source: `web/src/app/theme-toggle.tsx`

```tsx
import { useThemeMode } from "./theme-mode";

export function ThemeToggle(): JSX.Element {
  const { isDark, toggleMode } = useThemeMode();

  return (
    <button
      type="button"
      className="theme-toggle"
      aria-label={isDark ? "切换到浅色模式" : "切换到深色模式"}
      title={isDark ? "切换到浅色模式" : "切换到深色模式"}
      onClick={toggleMode}
    >
      {isDark ? <SunIcon /> : <MoonIcon />}
    </button>
  );
}

function SunIcon(): JSX.Element {
  return (
    <svg className="theme-toggle-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2.5M12 19.5V22M4.93 4.93 6.7 6.7M17.3 17.3l1.77 1.77M2 12h2.5M19.5 12H22M4.93 19.07 6.7 17.3M17.3 6.7l1.77-1.77" />
    </svg>
  );
}

function MoonIcon(): JSX.Element {
  return (
    <svg className="theme-toggle-icon" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path d="M20.1 14.8A8.2 8.2 0 0 1 9.2 3.9a8.7 8.7 0 1 0 10.9 10.9Z" />
    </svg>
  );
}
```

## Layout CSS (key structure)

Source: `web/src/styles/layout.css` — full file ~522 lines. Key structural facts:

- `.console-shell` — flex row; `::before` = aurora radial-gradient wash (accent violet top-left + cyan top-right, animated 18s drift); `::after` = 24px observatory grid with radial mask
- `.console-sidebar` — fixed 250px, sticky, full-height, gradient `surface-strong → accent 8%` background, panel shadow
- `.console-brand` — 40px aurora-gradient logo tile + "ModelMux / 控制台 / LOCAL PROXY · OPS" copy
- `.console-nav-link` — 44px rows, active state = aurora-soft bg + glow + left 3px aurora indicator bar
- `.console-header` — sticky, backdrop-blur 12px, max-width 1600px
- `.header-live-pill` — pill with states: `--active` (green LIVE + pulse dot), `--cooling` (amber DEGRADED), `--invalid`/`--idle` (red DOWN), `--loading` (violet SYNC)
- `.header-status-metrics` — pill container with 4 metrics (可用/冷却/失效/Provider), tabular-nums mono values
- `.theme-toggle` — 34px circular glass button, spring scale on hover, rotate on active
- `.console-content` — padding 20px 24px 32px, max-width 1600px