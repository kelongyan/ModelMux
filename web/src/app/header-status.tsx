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
