import type { AdminDashboardResponse } from "../types/admin";
import { HealthDot } from "../components/health-dot";

type HeaderStatusProps = {
  data: AdminDashboardResponse | undefined;
  loading: boolean;
};

export function HeaderStatus({ data, loading }: HeaderStatusProps): JSX.Element {
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
        <span className="header-metrics-sep" />
        <span>冷却 <strong>{data.cooling_keys}</strong></span>
        <span className="header-metrics-sep" />
        <span>失效 <strong>{data.invalid_keys}</strong></span>
        <span className="header-metrics-sep" />
        <span>Provider <strong>{data.provider_count}</strong></span>
      </div>
    </div>
  );
}
