import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Empty, Result, Space, Spin, Typography, message } from "antd";
import { startTransition } from "react";
import { useNavigate } from "react-router-dom";

import { activateProvider, fetchDashboard, triggerReload } from "../api/admin";
import { formatClockShort, formatRelativeTime } from "../components/format-time";
import { HealthDot } from "../components/health-dot";
import { KeyPoolDots } from "../components/key-pool-dots";
import type { AdminDashboardResponse, AdminEvent, AdminProviderSummary } from "../types/admin";

const dashboardQueryKey = ["dashboard"];

export function DashboardPage(): JSX.Element {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [messageApi, contextHolder] = message.useMessage();

  const dashboardQuery = useQuery({
    queryKey: dashboardQueryKey,
    queryFn: fetchDashboard,
    refetchInterval: 5000,
  });

  const reloadMutation = useMutation({
    mutationFn: triggerReload,
    onSuccess: async () => {
      messageApi.success("已触发配置重载");
      startTransition(() => {
        void queryClient.invalidateQueries({ queryKey: dashboardQueryKey });
      });
    },
    onError: (error: Error) => {
      messageApi.error(`重载失败：${error.message}`);
    },
  });

  const activateMutation = useMutation({
    mutationFn: activateProvider,
    onMutate: async (providerID) => {
      await queryClient.cancelQueries({ queryKey: dashboardQueryKey });
      const previous = queryClient.getQueryData<AdminDashboardResponse>(dashboardQueryKey);
      if (previous) {
        const next = applyOptimisticActivate(previous, providerID);
        queryClient.setQueryData(dashboardQueryKey, next);
      }
      return { previous };
    },
    onSuccess: async (_, providerID) => {
      messageApi.success(`已切换到 ${providerID}`);
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: dashboardQueryKey }),
        queryClient.invalidateQueries({ queryKey: ["providers"] }),
      ]);
    },
    onError: (error: Error, _id, context) => {
      messageApi.error(`切换失败：${error.message}`);
      if (context?.previous) {
        queryClient.setQueryData(dashboardQueryKey, context.previous);
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
  const events = dashboard.events ?? [];
  const lastUpdated = formatClockShort(dashboardQuery.dataUpdatedAt);

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={16} className="console-stack">
        <Card className="surface-card dashboard-overview-card" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Dashboard</Typography.Text>
              <Typography.Title level={3} className="section-title">
                控制台总览
              </Typography.Title>
            </div>
            <Space wrap>
              <span className="dashboard-updated-at">{`更新于 ${lastUpdated}`}</span>
              <Button
                type="primary"
                loading={reloadMutation.isPending}
                onClick={() => reloadMutation.mutate()}
              >
                立即重载
              </Button>
              <Button onClick={() => void dashboardQuery.refetch()}>刷新</Button>
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
              <p className="table-subtext">
                {dashboard.provider_count === 0
                  ? "还没有配置 provider，先去提供商页面新增上游。"
                  : `当前共 ${dashboard.provider_count} 个 provider，切换操作会直接写回配置。`}
              </p>
            </div>
            <div className="dashboard-stat-strip">
              <OverviewStat label="可用 Key" value={dashboard.active_keys} tone="green" />
              <OverviewStat label="冷却中" value={dashboard.cooling_keys} tone="amber" />
              <OverviewStat label="已失效" value={dashboard.invalid_keys} tone="red" />
              <OverviewStat label="Provider" value={dashboard.provider_count} tone="teal" />
            </div>
          </div>
        </Card>

        <Card className="surface-card" bordered={false}>
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
            <div className="provider-grid">
              {dashboard.providers.map((provider) => (
                <ProviderCard
                  key={provider.id}
                  provider={provider}
                  recentIssue={findRecentIssue(events, provider.id)}
                  activating={activateMutation.isPending && activateMutation.variables === provider.id}
                  onActivate={() => activateMutation.mutate(provider.id)}
                  onOpenDetail={() => navigate(`/providers?provider=${encodeURIComponent(provider.id)}`)}
                />
              ))}
              <button type="button" className="provider-card provider-card--add" onClick={() => navigate("/providers")}>
                <span className="provider-card-add-plus">+</span>
                <span>新增 Provider</span>
              </button>
            </div>
          )}
        </Card>

        <Card className="surface-card events-rail-card" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Recent Events</Typography.Text>
              <Typography.Title level={3} className="section-title">
                最近事件
              </Typography.Title>
            </div>
            <Button size="small" onClick={() => navigate("/events")}>
              查看全部
            </Button>
          </div>
          {events.length === 0 ? (
            <Empty description="暂无事件" image={Empty.PRESENTED_IMAGE_SIMPLE} />
          ) : (
            <ul className="events-rail-list events-rail-list--full">
              {events.slice(0, 12).map((event) => (
                <li key={`${event.seq}-${event.event}`} className={`events-rail-item events-rail-item--${event.level}`}>
                  <span className="events-rail-time">{formatClockShort(new Date(event.at).getTime())}</span>
                  <span className={`events-rail-level events-rail-level--${event.level}`}>{event.level}</span>
                  <div className="events-rail-content">
                    <span className="events-rail-msg" title={event.message}>
                      {event.message}
                    </span>
                    <span className="events-rail-meta">
                      {event.provider_id ? `Provider ${event.provider_id}` : event.category}
                      {event.status ? ` · 状态 ${event.status}` : ""}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
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

type ProviderCardProps = {
  provider: AdminProviderSummary;
  recentIssue: AdminEvent | undefined;
  activating: boolean;
  onActivate: () => void;
  onOpenDetail: () => void;
};

function ProviderCard({ provider, recentIssue, activating, onActivate, onOpenDetail }: ProviderCardProps): JSX.Element {
  const tone = computeCardTone(provider);
  const className = `provider-card provider-card--${tone}${provider.active ? " provider-card--current" : ""}`;

  return (
    <article className={className}>
      <header className="provider-card-head">
        <div className="provider-card-title-row">
          <div className="provider-card-title">
            <HealthDot state={tone} pulse={provider.active && tone === "active"} />
            <h3>{provider.id}</h3>
          </div>
          <span className={`provider-card-state provider-card-state--${tone}`}>{providerStateLabel(provider, tone)}</span>
        </div>
        <a className="provider-card-url" href={provider.target_url} target="_blank" rel="noreferrer">
          {provider.target_url}
        </a>
      </header>

      <div className="provider-card-pool">
        <div className="provider-card-pool-indicator">
          <KeyPoolDots active={provider.active_keys} cooling={provider.cooling_keys} invalid={provider.invalid_keys} max={24} />
          <span className="table-subtext">Key 池健康度</span>
        </div>
        <div className="provider-card-stats-grid">
          <div className="provider-card-stat">
            <span>可用</span>
            <strong>{provider.active_keys}</strong>
          </div>
          <div className="provider-card-stat">
            <span>冷却</span>
            <strong>{provider.cooling_keys}</strong>
          </div>
          <div className="provider-card-stat">
            <span>失效</span>
            <strong>{provider.invalid_keys}</strong>
          </div>
          <div className="provider-card-stat">
            <span>总量</span>
            <strong>{provider.total_keys}</strong>
          </div>
        </div>
      </div>

      {recentIssue ? (
        <div className="provider-card-issue" title={recentIssue.message}>
          <span className="provider-card-issue-label">最近异常</span>
          <span className="provider-card-issue-msg">{recentIssue.message}</span>
          <span className="provider-card-issue-time">{formatRelativeTime(recentIssue.at)}</span>
        </div>
      ) : null}

      <footer className="provider-card-actions">
        {provider.active ? (
          <Button disabled>已活跃</Button>
        ) : (
          <Button type="primary" loading={activating} onClick={onActivate}>
            切换为活跃
          </Button>
        )}
        <Button onClick={onOpenDetail}>查看详情</Button>
      </footer>
    </article>
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

function findRecentIssue(events: AdminEvent[], providerID: string): AdminEvent | undefined {
  for (const event of events) {
    if (event.level === "info") {
      continue;
    }
    const eventProvider = event.provider_id ?? ((event.data?.provider_id as string | undefined) ?? "");
    if (eventProvider === providerID) {
      return event;
    }
  }
  return undefined;
}
