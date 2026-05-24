import { Spin, Layout, Space, Tag, Typography } from "antd";
import { lazy, Suspense, useMemo } from "react";
import { Navigate, NavLink, Route, Routes, useLocation } from "react-router-dom";

const AboutPage = lazy(() => import("./pages/about-page").then((module) => ({ default: module.AboutPage })));
const DashboardPage = lazy(() => import("./pages/dashboard-page").then((module) => ({ default: module.DashboardPage })));
const EventsPage = lazy(() => import("./pages/events-page").then((module) => ({ default: module.EventsPage })));
const ProvidersPage = lazy(() => import("./pages/providers-page").then((module) => ({ default: module.ProvidersPage })));
const SettingsPage = lazy(() => import("./pages/settings-page").then((module) => ({ default: module.SettingsPage })));

type NavigationItem = {
  key: string;
  label: string;
  badge?: string;
};

const navigationItems: NavigationItem[] = [
  { key: "/dashboard", label: "总览", badge: "LIVE" },
  { key: "/providers", label: "提供商" },
  { key: "/settings", label: "设置" },
  { key: "/events", label: "事件" },
  { key: "/about", label: "关于" },
];

// App 渲染控制台主框架，并把各个功能页挂到统一的路由容器中。
export function App(): JSX.Element {
  const location = useLocation();
  const routeKey = useMemo(() => location.pathname, [location.pathname]);

  return (
    <Layout className="console-shell">
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
                {selected && item.badge ? <small>{item.badge}</small> : null}
              </NavLink>
            );
          })}
        </nav>
        <div className="console-sidebar-note">
          <span>阶段 5</span>
          <p>事件中心、关于页和备份导出已经接入，当前控制台已接近首版完整交付状态。</p>
        </div>
      </aside>
      <Layout className="console-main">
        <header className="console-header">
          <div>
            <Typography.Text className="console-header-label">当前页面</Typography.Text>
            <Typography.Title level={2} className="console-header-title">
              {resolveRouteTitle(location.pathname)}
            </Typography.Title>
          </div>
          <Space size="middle">
            <Tag color="gold">Embedded UI</Tag>
            <Tag color="cyan">Local Admin API</Tag>
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
