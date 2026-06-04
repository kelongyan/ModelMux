import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Spin, Layout, Typography, message } from "antd";
import { lazy, Suspense, useCallback, useMemo } from "react";
import { Navigate, NavLink, Route, Routes, useLocation, useNavigate } from "react-router-dom";

import { fetchDashboard, triggerReload } from "./api/admin";
import { HealthDot } from "./components/health-dot";
import { useGlobalShortcuts } from "./components/use-global-shortcuts";
import type { AdminDashboardResponse } from "./types/admin";

const AboutPage = lazy(() => import("./pages/about-page").then((module) => ({ default: module.AboutPage })));
const DashboardPage = lazy(() => import("./pages/dashboard-page").then((module) => ({ default: module.DashboardPage })));
const EventsPage = lazy(() => import("./pages/events-page").then((module) => ({ default: module.EventsPage })));
const ProvidersPage = lazy(() => import("./pages/providers-page").then((module) => ({ default: module.ProvidersPage })));
const SettingsPage = lazy(() => import("./pages/settings-page").then((module) => ({ default: module.SettingsPage })));
const StatsPage = lazy(() => import("./pages/stats-page").then((module) => ({ default: module.StatsPage })));

type NavigationItem = {
  key: string;
  label: string;
  hint: string;
};

const navigationItems: NavigationItem[] = [
  { key: "/dashboard", label: "总览", hint: "g d" },
  { key: "/providers", label: "提供商", hint: "g p" },
  { key: "/stats", label: "调用统计", hint: "g t" },
  { key: "/settings", label: "设置", hint: "g s" },
  { key: "/events", label: "事件", hint: "g e" },
  { key: "/about", label: "关于", hint: "g a" },
];

export function App(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [messageApi, contextHolder] = message.useMessage();
  const routeKey = useMemo(() => location.pathname, [location.pathname]);

  const dashboardQuery = useQuery({
    queryKey: ["dashboard"],
    queryFn: fetchDashboard,
    refetchInterval: 10000,
  });

  const handleReload = useCallback(async () => {
    try {
      await triggerReload();
      messageApi.success("已触发配置重载");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["dashboard"] }),
        queryClient.invalidateQueries({ queryKey: ["events"] }),
        queryClient.invalidateQueries({ queryKey: ["providers"] }),
      ]);
    } catch (error) {
      messageApi.error(`重载失败：${error instanceof Error ? error.message : "未知错误"}`);
    }
  }, [messageApi, queryClient]);

  useGlobalShortcuts({
    onReload: handleReload,
    onGoto: (path) => navigate(path),
  });

  const sidebarHealth = computeSidebarHealth(dashboardQuery.data, dashboardQuery.isLoading);

  return (
    <Layout className="console-shell">
      {contextHolder}
      <aside className="console-sidebar">
        <ConsoleBrand />
        <SidebarHealthBadge state={sidebarHealth.state} label={sidebarHealth.label} hint={sidebarHealth.hint} />
        <nav className="console-nav">
          {navigationItems.map((item) => {
            const selected = location.pathname === item.key;
            return (
              <NavLink
                key={item.key}
                to={item.key}
                className={selected ? "console-nav-link is-active" : "console-nav-link"}
              >
                <span>{item.label}</span>
                <small className="console-nav-hint">{item.hint}</small>
              </NavLink>
            );
          })}
        </nav>
      </aside>
      <Layout className="console-main">
        <header className="console-header">
          <div className="console-header-copy">
            <Typography.Text className="console-header-label">当前页面</Typography.Text>
            <Typography.Title level={2} className="console-header-title">
              {resolveRouteTitle(location.pathname)}
            </Typography.Title>
            <Typography.Paragraph className="console-header-subtitle">
              {resolveRouteDescription(location.pathname)}
            </Typography.Paragraph>
          </div>
          <HeaderSnapshot data={dashboardQuery.data} loading={dashboardQuery.isLoading} />
        </header>
        <main className="console-content">
          <Suspense fallback={<RouteFallback />}>
            <Routes key={routeKey}>
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/providers" element={<ProvidersPage />} />
              <Route path="/stats" element={<StatsPage />} />
              <Route path="/settings" element={<SettingsPage />} />
              <Route path="/events" element={<EventsPage />} />
              <Route path="/about" element={<AboutPage />} />
            </Routes>
          </Suspense>
        </main>
      </Layout>
    </Layout>
  );
}

function ConsoleBrand(): JSX.Element {
  return (
    <div className="console-brand">
      <p className="console-kicker">ModelMux</p>
      <h1>控制台</h1>
      <span>本地模型代理的控制平面</span>
      <small className="console-brand-meta">单 Provider 轮换 · 本地优先 · 内嵌管理台</small>
    </div>
  );
}

type SidebarHealth = {
  state: "active" | "cooling" | "invalid" | "idle";
  label: string;
  hint: string;
};

function SidebarHealthBadge({ state, label, hint }: SidebarHealth): JSX.Element {
  return (
    <div className={`sidebar-health sidebar-health--${state}`}>
      <div className="sidebar-health-row">
        <HealthDot state={state} pulse={state === "active"} />
        <span className="sidebar-health-label" title={label}>
          {label}
        </span>
      </div>
      <span className="sidebar-health-hint">{hint}</span>
    </div>
  );
}

type HeaderSnapshotProps = {
  data: AdminDashboardResponse | undefined;
  loading: boolean;
};

function HeaderSnapshot({ data, loading }: HeaderSnapshotProps): JSX.Element {
  const summary = computeSidebarHealth(data, loading);

  return (
    <div className="console-header-status">
      <div className="console-header-status-row">
        <div className="console-header-provider">
          <HealthDot state={summary.state} pulse={summary.state === "active"} />
          <div>
            <span className="console-header-provider-label">当前活跃</span>
            <strong className="console-header-provider-name" title={summary.label}>
              {summary.label}
            </strong>
          </div>
        </div>
        <div className="console-header-meta">
          <span>{loading ? "正在同步状态" : `${data?.provider_count ?? 0} 个 Provider`}</span>
          <span>Ctrl / ⌘ + R 重载</span>
        </div>
      </div>
      <div className="console-header-metrics">
        <div className="console-header-metric">
          <span>可用</span>
          <strong>{data ? data.active_keys : "—"}</strong>
        </div>
        <div className="console-header-metric">
          <span>冷却</span>
          <strong>{data ? data.cooling_keys : "—"}</strong>
        </div>
        <div className="console-header-metric">
          <span>失效</span>
          <strong>{data ? data.invalid_keys : "—"}</strong>
        </div>
        <div className="console-header-metric">
          <span>概览</span>
          <strong>{summary.hint}</strong>
        </div>
      </div>
      <p className="console-shortcut-hint">快捷键：先按 g，再按 d / p / t / s / e / a</p>
    </div>
  );
}

function computeSidebarHealth(data: AdminDashboardResponse | undefined, loading: boolean): SidebarHealth {
  if (loading || !data) {
    return { state: "idle", label: "正在加载…", hint: "等待控制面返回状态" };
  }
  if (!data.active_provider) {
    return { state: "invalid", label: "未配置 Provider", hint: "请先在提供商页新增配置" };
  }
  if (data.active_keys === 0) {
    return {
      state: "invalid",
      label: data.active_provider,
      hint: data.cooling_keys > 0 ? "无可用 Key（仅剩 cooling）" : "无可用 Key",
    };
  }
  if (data.cooling_keys > 0 || data.invalid_keys > 0) {
    return {
      state: "cooling",
      label: data.active_provider,
      hint: `可用 ${data.active_keys} · 冷却 ${data.cooling_keys} · 失效 ${data.invalid_keys}`,
    };
  }
  return {
    state: "active",
    label: data.active_provider,
    hint: `可用 ${data.active_keys} · 当前运行稳定`,
  };
}

function RouteFallback(): JSX.Element {
  return (
    <div className="console-loading">
      <Typography.Text className="table-subtext">正在加载页面…</Typography.Text>
      <Spin size="large" />
    </div>
  );
}

function resolveRouteTitle(pathname: string): string {
  const matched = navigationItems.find((item) => pathname === item.key);
  return matched ? matched.label : "总览";
}

function resolveRouteDescription(pathname: string): string {
  switch (pathname) {
    case "/providers":
      return "管理 provider、key 池和活跃切换，适合做容量与可用性维护。";
    case "/stats":
      return "查看调用量、模型使用和最近请求明细，观察流量与成本趋势。";
    case "/settings":
      return "维护运行参数、日志与状态持久化配置，并区分热生效与重启项。";
    case "/events":
      return "筛选运行事件与请求上下文，定位异常链路和 provider 问题。";
    case "/about":
      return "查看运行环境、版本信息与备份导出入口。";
    default:
      return "聚焦当前活跃 provider、key 池健康度和最近异常，适合日常值守。";
  }
}
