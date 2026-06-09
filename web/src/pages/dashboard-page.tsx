import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Empty, Result, Space, Spin, Typography, message } from "antd";
import { startTransition } from "react";
import { useNavigate } from "react-router-dom";

import { activateProvider, fetchDashboard, triggerReload } from "../api/admin";
import { queryKeys } from "../api/query-keys";
import { formatClockShort } from "../components/format-time";
import { HealthDot } from "../components/health-dot";
import type { AdminDashboardResponse, AdminProviderCircuit, AdminProviderSummary, AdminStatsHealth } from "../types/admin";

export function DashboardPage(): JSX.Element {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [messageApi, contextHolder] = message.useMessage();

  const dashboardQuery = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: fetchDashboard,
    refetchInterval: 5000,
  });

  const reloadMutation = useMutation({
    mutationFn: triggerReload,
    onSuccess: async () => {
      messageApi.success("已触发配置重载");
      startTransition(() => {
        void queryClient.invalidateQueries({ queryKey: queryKeys.dashboard });
      });
    },
    onError: (error: Error) => {
      messageApi.error(`重载失败：${error.message}`);
    },
  });

  const activateMutation = useMutation({
    mutationFn: activateProvider,
    onMutate: async (providerID) => {
      await queryClient.cancelQueries({ queryKey: queryKeys.dashboard });
      const previous = queryClient.getQueryData<AdminDashboardResponse>(queryKeys.dashboard);
      if (previous) {
        const next = applyOptimisticActivate(previous, providerID);
        queryClient.setQueryData(queryKeys.dashboard, next);
      }
      return { previous };
    },
    onSuccess: async (_, providerID) => {
      messageApi.success(`已切换到 ${providerID}`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: queryKeys.dashboard }),
        queryClient.invalidateQueries({ queryKey: queryKeys.providers }),
      ]);
    },
    onError: (error: Error, _id, context) => {
      messageApi.error(`切换失败：${error.message}`);
      if (context?.previous) {
        queryClient.setQueryData(queryKeys.dashboard, context.previous);
      }
    },
  });

  if (dashboardQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (dashboardQuery.isError || !dashboardQuery.data) {
    return (
      <Result
        status="error"
        title="总览数据加载失败"
        subTitle={dashboardQuery.error instanceof Error ? dashboardQuery.error.message : "未知错误"}
        extra={
          <Button onClick={() => void dashboardQuery.refetch()} type="primary">
            重新获取
          </Button>
        }
      />
    );
  }

  const dashboard = dashboardQuery.data;
  const lastUpdated = formatClockShort(dashboardQuery.dataUpdatedAt);

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={16} className="console-stack">
        <Card className="surface-card dashboard-overview-card reveal-card reveal-delay-0" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Dashboard</Typography.Text>
              <Typography.Title level={3} className="section-title">
                控制台总览
              </Typography.Title>
            </div>
            <Space wrap>
              <Typography.Text className="dashboard-updated-at">{`更新于 ${lastUpdated}`}</Typography.Text>
              <Button
                size="small"
                type="primary"
                loading={reloadMutation.isPending}
                onClick={() => reloadMutation.mutate()}
              >
                立即重载
              </Button>
              <Button size="small" onClick={() => void dashboardQuery.refetch()}>刷新</Button>
            </Space>
          </div>
          <div className="dashboard-overview-grid">
            <div className="dashboard-active-panel">
              <span className="dashboard-panel-label">当前活跃 Provider</span>
              <div className="dashboard-active-provider-row">
                <HealthDot state={computeOverviewState(dashboard)} pulse={dashboard.active_keys > 0} />
                <strong className="dashboard-active-provider-name" title={dashboard.active_provider}>
                  {dashboard.active_provider || "未配置"}
                </strong>
              </div>
              <span className="dashboard-panel-desc">
                {dashboard.provider_count === 0
                  ? "还没有配置 provider，先去提供商页面新增上游。"
                  : `当前共 ${dashboard.provider_count} 个 provider，切换操作会直接写回配置。`}
              </span>
              <div className="dashboard-health-strip">
                <HealthSignal
                  label="熔断"
                  value={formatCircuitState(dashboard.provider_circuit)}
                  state={circuitHealthDot(dashboard.provider_circuit)}
                />
                <HealthSignal
                  label="Stats 队列"
                  value={formatStatsHealth(dashboard.stats)}
                  state={statsHealthDot(dashboard.stats)}
                />
              </div>
            </div>
            <div className="dashboard-stat-strip">
              <OverviewStat label="可用 Key" value={dashboard.active_keys} tone="green" />
              <OverviewStat label="冷却中" value={dashboard.cooling_keys} tone="amber" />
              <OverviewStat label="已失效" value={dashboard.invalid_keys} tone="red" />
              <OverviewStat label="Provider" value={dashboard.provider_count} tone="teal" />
            </div>
          </div>
        </Card>

        <Card className="surface-card reveal-card reveal-delay-1" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Providers</Typography.Text>
              <Typography.Title level={3} className="section-title">
                Provider 列表
              </Typography.Title>
            </div>
          </div>

          {dashboard.providers.length === 0 ? (
            <Empty description="还未配置任何 provider" />
          ) : (
            <div className="provider-list">
              {dashboard.providers.map((provider) => (
                <ProviderRow
                  key={provider.id}
                  provider={provider}
                  activating={activateMutation.isPending && activateMutation.variables === provider.id}
                  onActivate={() => activateMutation.mutate(provider.id)}
                  onOpenDetail={() => navigate(`/providers?provider=${encodeURIComponent(provider.id)}`)}
                />
              ))}
            </div>
          )}
        </Card>

      </Space>
    </>
  );
}

type OverviewStatProps = {
  label: string;
  value: number;
  tone: "green" | "amber" | "red" | "teal";
};

function OverviewStat({ label, value, tone }: OverviewStatProps): JSX.Element {
  return (
    <div className={`dashboard-stat-card dashboard-stat-card--${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

type HealthSignalProps = {
  label: string;
  value: string;
  state: "active" | "cooling" | "invalid" | "idle";
};

function HealthSignal({ label, value, state }: HealthSignalProps): JSX.Element {
  return (
    <div className="dashboard-health-signal">
      <span>{label}</span>
      <strong>
        <HealthDot state={state} pulse={state === "active"} />
        {value}
      </strong>
    </div>
  );
}

type ProviderRowProps = {
  provider: AdminProviderSummary;
  activating: boolean;
  onActivate: () => void;
  onOpenDetail: () => void;
};

function ProviderRow({ provider, activating, onActivate, onOpenDetail }: ProviderRowProps): JSX.Element {
  const tone = computeCardTone(provider);
  const className = `provider-row provider-row--${tone}${provider.active ? " provider-row--current" : ""}`;
  const configuredKeys = provider.total_keys + provider.disabled_keys;

  return (
    <div className={className}>
      <div className="provider-row-status">
        <HealthDot state={tone} pulse={provider.active && tone === "active"} />
        <span className={`provider-row-badge provider-row-badge--${tone}`}>{providerStateLabel(provider, tone)}</span>
      </div>
      <div className="provider-row-info">
        <span className="provider-row-name">{provider.id}</span>
        <a className="provider-row-url" href={provider.target_url} target="_blank" rel="noreferrer">
          {provider.target_url}
        </a>
      </div>
      <div className="provider-row-keys">
        <span className="provider-row-key-item provider-row-key-item--active">{provider.active_keys} 可用</span>
        <span className="provider-row-key-item provider-row-key-item--cooling">{provider.cooling_keys} 冷却</span>
        <span className="provider-row-key-item provider-row-key-item--invalid">{provider.invalid_keys} 失效</span>
        {provider.disabled_keys > 0 ? (
          <span className="provider-row-key-item provider-row-key-item--total">{provider.disabled_keys} 停用</span>
        ) : null}
        <span className="provider-row-key-item provider-row-key-item--total">共 {configuredKeys}</span>
      </div>
      <div className="provider-row-actions">
        {provider.active ? (
          <span className="provider-row-active-label">已活跃</span>
        ) : (
          <Button size="small" type="primary" loading={activating} onClick={onActivate}>
            切换
          </Button>
        )}
        <Button size="small" type="link" onClick={onOpenDetail}>详情</Button>
      </div>
    </div>
  );
}

function providerStateLabel(provider: AdminProviderSummary, tone: "active" | "cooling" | "invalid" | "idle"): string {
  if (provider.active) {
    return "当前活跃";
  }
  if (tone === "invalid") {
    return "不可用";
  }
  if (tone === "cooling") {
    return "波动中";
  }
  return "待命";
}

function computeOverviewState(dashboard: AdminDashboardResponse): "active" | "cooling" | "invalid" | "idle" {
  if (dashboard.provider_circuit?.state === "open") {
    return "invalid";
  }
  if (dashboard.provider_circuit?.state === "half_open") {
    return "cooling";
  }
  if (!dashboard.active_provider) {
    return "idle";
  }
  if (dashboard.active_keys === 0) {
    return dashboard.cooling_keys > 0 ? "cooling" : "invalid";
  }
  if (dashboard.cooling_keys > 0 || dashboard.invalid_keys > 0) {
    return "cooling";
  }
  return "active";
}

function applyOptimisticActivate(prev: AdminDashboardResponse, providerID: string): AdminDashboardResponse {
  const target = prev.providers.find((p) => p.id === providerID);
  if (!target) {
    return prev;
  }
  const providers = prev.providers.map((p) => ({ ...p, active: p.id === providerID }));
  return {
    ...prev,
    active_provider: providerID,
    active_keys: target.active_keys,
    cooling_keys: target.cooling_keys,
    invalid_keys: target.invalid_keys,
    providers,
  };
}

function computeCardTone(provider: AdminProviderSummary): "active" | "cooling" | "invalid" | "idle" {
  if (provider.active_keys === 0 && provider.cooling_keys === 0) {
    return "invalid";
  }
  if (provider.active_keys === 0) {
    return "cooling";
  }
  if (provider.cooling_keys > 0 || provider.invalid_keys > 0) {
    return provider.active ? "cooling" : "idle";
  }
  return provider.active ? "active" : "idle";
}

function formatCircuitState(circuit: AdminProviderCircuit | undefined): string {
  if (!circuit) {
    return "未挂载";
  }
  switch (circuit.state) {
    case "open":
      return `${circuit.consecutive_failures} 次失败`;
    case "half_open":
      return "探测中";
    case "closed":
      return "正常";
    default:
      return circuit.state || "未知";
  }
}

function circuitHealthDot(circuit: AdminProviderCircuit | undefined): "active" | "cooling" | "invalid" | "idle" {
  if (!circuit) {
    return "idle";
  }
  if (circuit.state === "open") {
    return "invalid";
  }
  if (circuit.state === "half_open") {
    return "cooling";
  }
  return "active";
}

function formatStatsHealth(stats: AdminStatsHealth | undefined): string {
  if (!stats?.enabled) {
    return "未启用";
  }
  return `${stats.queue_depth}/${stats.queue_capacity} · 丢 ${stats.dropped_records}`;
}

function statsHealthDot(stats: AdminStatsHealth | undefined): "active" | "cooling" | "invalid" | "idle" {
  if (!stats?.enabled) {
    return "idle";
  }
  if (stats.dropped_records > 0) {
    return "cooling";
  }
  if (stats.queue_capacity > 0 && stats.queue_depth * 4 >= stats.queue_capacity * 3) {
    return "cooling";
  }
  return "active";
}
