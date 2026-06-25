import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Spin, Typography, message } from "antd";
import { lazy, Suspense, useCallback, useMemo, useState } from "react";
import { Navigate, Route, Routes, useLocation, useNavigate } from "react-router-dom";

import { fetchDashboard, triggerReload } from "./api/admin";
import { queryKeys } from "./api/query-keys";
import { ConsoleShell } from "./app/console-shell";
import { ErrorBoundary } from "./components/error-boundary";
import { PageTransition } from "./components/page-transition";
import { ShortcutsHelp } from "./components/shortcuts-help";
import { useGlobalShortcuts } from "./components/use-global-shortcuts";

const AboutPage = lazy(() => import("./pages/about-page").then((module) => ({ default: module.AboutPage })));
const DashboardPage = lazy(() => import("./pages/dashboard-page").then((module) => ({ default: module.DashboardPage })));
const EventsPage = lazy(() => import("./pages/events-page").then((module) => ({ default: module.EventsPage })));
const NotFoundPage = lazy(() => import("./pages/not-found-page").then((module) => ({ default: module.NotFoundPage })));
const ProvidersPage = lazy(() => import("./pages/providers-page").then((module) => ({ default: module.ProvidersPage })));
const SettingsPage = lazy(() => import("./pages/settings-page").then((module) => ({ default: module.SettingsPage })));
const StatsPage = lazy(() => import("./pages/stats-page").then((module) => ({ default: module.StatsPage })));

export function App(): JSX.Element {
  const location = useLocation();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [messageApi, contextHolder] = message.useMessage();
  const [shortcutsOpen, setShortcutsOpen] = useState(false);
  const routeKey = useMemo(() => location.pathname, [location.pathname]);

  const dashboardQuery = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: fetchDashboard,
  });

  const handleReload = useCallback(async () => {
    try {
      await triggerReload();
      messageApi.success("已触发配置重载");
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.dashboard }),
        queryClient.invalidateQueries({ queryKey: queryKeys.events(200) }),
        queryClient.invalidateQueries({ queryKey: queryKeys.providers }),
      ]);
    } catch (error) {
      messageApi.error(`重载失败：${error instanceof Error ? error.message : "未知错误"}`);
    }
  }, [messageApi, queryClient]);

  useGlobalShortcuts({
    onReload: handleReload,
    onGoto: (path) => navigate(path),
    onHelp: () => setShortcutsOpen(true),
  });

  return (
    <ConsoleShell dashboard={dashboardQuery.data} dashboardLoading={dashboardQuery.isLoading}>
      {contextHolder}
      <Suspense fallback={<RouteFallback />}>
        <PageTransition animationKey={location.pathname}>
          <Routes key={routeKey}>
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
      <ShortcutsHelp open={shortcutsOpen} onClose={() => setShortcutsOpen(false)} />
    </ConsoleShell>
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
