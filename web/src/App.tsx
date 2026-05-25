import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Spin, Layout, Space, Tag, Tooltip, Typography, message } from "antd";
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

type NavigationItem = {
  key: string;
  label: string;
  hint: string;
};

const navigationItems: NavigationItem[] = [
  { key: "/dashboard", label: "总览", hint: "g d" },
  { key: "/providers", label: "提供商", hint: "g p" },
  { key: "/settings", label: "设置", hint: "g s" },
  { key: "/events", label: "事件", hint: "g e" },
  { key: "/about", label: "关于", hint: "g a" },
];

// App 渲染控制台主框架，并把各个功能页挂到统一的路由容器中。
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
          <div>
            <Typography.Text className="console-header-label">当前页面</Typography.Text>
            <Typography.Title level={2} className="console-header-title">
              {resolveRouteTitle(location.pathname)}
            </Typography.Title>
          </div>
          <Space size="middle" wrap>
            <Tooltip title="Ctrl / ⌘ + R 触发后端 reload">
              <Tag color="cyan">⌘R 重载</Tag>
            </Tooltip>
            <Tooltip title="先按 g 再按 d / p / s / e / a 切换页面">
              <Tag color="gold">g + d/p/s/e/a 切页</Tag>
            </Tooltip>
          </Space>
        </header>
        <main className="console-content">
          <Suspense fallback={<RouteFallback />}>
            <Routes key={routeKey}>
              <Route path="/" element={<Navigate to="/dashboard" replace />} />
              <Route path="/dashboard" element={<DashboardPage />} />
              <Route path="/providers" element={<ProvidersPage />} />
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

// ConsoleBrand 渲染控制台品牌区，保持侧边栏的视觉锚点。
function ConsoleBrand(): JSX.Element {
  return (
    <div className="console-brand">
      <p className="console-kicker">ModelMux</p>
      <h1>控制台</h1>
      <span>本地优先的模型提供商编排界面</span>
    </div>
  );
}

type SidebarHealth = {
  state: "active" | "cooling" | "invalid" | "idle";
  label: string;
  hint: string;
};

// SidebarHealthBadge 把当前 active provider 和健康度提升到侧边栏顶部，无需打开总览也能感知状态。
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

// computeSidebarHealth 把 dashboard 聚合响应折算为侧边栏徽章可消费的健康度。
function computeSidebarHealth(data: AdminDashboardResponse | undefined, loading: boolean): SidebarHealth {
  if (loading || !data) {
    return { state: "idle", label: "正在加载…", hint: "—" };
  }
  if (!data.active_provider) {
    return { state: "invalid", label: "未配置 Provider", hint: "请在提供商页面新增" };
  }
  if (data.active_keys === 0) {
    return {
      state: "invalid",
      label: data.active_provider,
      hint: data.cooling_keys > 0 ? "无可用 Key（仅 cooling）" : "无可用 Key",
    };
  }
  if (data.cooling_keys > 0 || data.invalid_keys > 0) {
    return {
      state: "cooling",
      label: data.active_provider,
      hint: `可用 ${data.active_keys}  冷却 ${data.cooling_keys}  失效 ${data.invalid_keys}`,
    };
  }
  return {
    state: "active",
    label: data.active_provider,
    hint: `可用 ${data.active_keys}  全员健康`,
  };
}

// RouteFallback 在路由切换时提供轻量加载态，避免空白页闪烁。
function RouteFallback(): JSX.Element {
  return (
    <div className="console-loading">
      <Space direction="vertical" align="center" size={12}>
        <Spin size="large" />
        <Typography.Text className="table-subtext">正在加载页面…</Typography.Text>
      </Space>
    </div>
  );
}

// resolveRouteTitle 根据当前路径返回页头标题，避免在多个页面中重复声明。
function resolveRouteTitle(pathname: string): string {
  const matched = navigationItems.find((item) => pathname === item.key);
  return matched ? matched.label : "总览";
}
