import { Layout } from "antd";
import { NavLink, useLocation } from "react-router-dom";

import type { AdminDashboardResponse } from "../types/admin";
import { HeaderStatus } from "./header-status";
import { navigationItems } from "./navigation";

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
      <p className="console-kicker">ModelMux</p>
      <h1>控制台</h1>
    </div>
  );
}
