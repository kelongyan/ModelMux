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
};

const navigationItems: NavigationItem[] = [
  { key: "/dashboard", label: "总览" },
  { key: "/providers", label: "提供商" },
  { key: "/stats", label: "调用统计" },
  { key: "/settings", label: "设置" },
  { key: "/events", label: "事件" },
  { key: "/about", label: "关于" },
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

  return (
    <Layout className="console-shell">
      {contextHolder}
      <aside className="console-sidebar">
        <ConsoleBrand />
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
              </NavLink>
            );
          })}
        </nav>
      </aside>
      <Layout className="console-main">
        <header className="console-header">
          <HeaderStatus data={dashboardQuery.data} loading={dashboardQuery.isLoading} />
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
    </div>
  );
}

type HeaderStatusProps = {
  data: AdminDashboardResponse | undefined;
  loading: boolean;
};

function HeaderStatus({ data, loading }: HeaderStatusProps): JSX.Element {
  if (loading || !data) {
    return (
      <div className="console-header-status">
        <span className="header-status-loading">正在加载…</span>
      </div>
    );
  }
  const state: "active" | "cooling" | "invalid" | "idle" =
    !data.active_provider ? "idle" :
    data.active_keys === 0 ? (data.cooling_keys > 0 ? "cooling" : "invalid") :
    data.cooling_keys > 0 || data.invalid_keys > 0 ? "cooling" : "active";

  return (
    <div className="console-header-status">
      <div className="header-status-left">
        <HealthDot state={state} pulse={state === "active"} />
        <strong className="header-status-provider" title={data.active_provider || "未配置"}>
          {data.active_provider || "未配置 Provider"}
        </strong>
      </div>
      <div className="header-status-metrics">
        <span>可用 <strong>{data.active_keys}</strong></span>
        <span>冷却 <strong>{data.cooling_keys}</strong></span>
        <span>失效 <strong>{data.invalid_keys}</strong></span>
        <span>Provider <strong>{data.provider_count}</strong></span>
      </div>
    </div>
  );
}

function RouteFallback(): JSX.Element {
  return (
    <div className="console-loading">
      <Typography.Text className="table-subtext">正在加载页面…</Typography.Text>
      <Spin size="large" />
    </div>
  );
}


