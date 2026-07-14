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
