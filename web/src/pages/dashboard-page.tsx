import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Empty, Result, Space, Spin, Tag, Tooltip, Typography, message } from "antd";
import { startTransition } from "react";
import { useNavigate } from "react-router-dom";

import { activateProvider, fetchDashboard, triggerReload } from "../api/admin";
import { formatClockShort, formatRelativeTime } from "../components/format-time";
import { HealthDot } from "../components/health-dot";
import { KeyPoolDots } from "../components/key-pool-dots";
import type { AdminDashboardResponse, AdminEvent, AdminProviderSummary } from "../types/admin";

const dashboardQueryKey = ["dashboard"];

// DashboardPage 使用左右两栏布局：左侧 provider 卡片矩阵，右侧 KPI + 实时事件流。
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
      <div className="dashboard-grid">
        <section className="dashboard-left">
          <div className="dashboard-section-head">
            <div>
              <Typography.Text className="placeholder-kicker">Providers</Typography.Text>
              <Typography.Title level={3} className="section-title">
                提供商矩阵
              </Typography.Title>
            </div>
            <Space wrap>
              <Button
                type="primary"
                loading={reloadMutation.isPending}
                onClick={() => reloadMutation.mutate()}
              >
                立即重载
              </Button>
              <Button onClick={() => void dashboardQuery.refetch()}>手动刷新</Button>
            </Space>
          </div>

          {dashboard.providers.length === 0 ? (
            <Card className="surface-card" bordered={false}>
              <Empty description="还未配置任何 provider" />
            </Card>
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
        </section>

        <aside className="dashboard-right">
          <KpiBlock dashboard={dashboard} lastUpdated={lastUpdated} />
          <EventsRail events={events.slice(0, 12)} onViewAll={() => navigate("/events")} />
        </aside>
      </div>
    </>
  );
}

type ProviderCardProps = {
  provider: AdminProviderSummary;
  recentIssue: AdminEvent | undefined;
  activating: boolean;
  onActivate: () => void;
  onOpenDetail: () => void;
};

// ProviderCard 渲染左栏的单个 provider，是 Dashboard 最高频的交互单元。
function ProviderCard({ provider, recentIssue, activating, onActivate, onOpenDetail }: ProviderCardProps): JSX.Element {
  const tone = computeCardTone(provider);
  const className = `provider-card provider-card--${tone}${provider.active ? " provider-card--current" : ""}`;
  return (
    <article className={className}>
      <header className="provider-card-head">
        <div className="provider-card-title">
          <HealthDot state={tone} pulse={provider.active && tone === "active"} />
          <h3>{provider.id}</h3>
          {provider.active ? <Tag color="green">当前活跃</Tag> : <Tag>待命</Tag>}
        </div>
        <a className="provider-card-url" href={provider.target_url} target="_blank" rel="noreferrer">
          {provider.target_url}
        </a>
      </header>

      <div className="provider-card-pool">
        <KeyPoolDots
          active={provider.active_keys}
          cooling={provider.cooling_keys}
          invalid={provider.invalid_keys}
          max={32}
        />
        <div className="provider-card-stats">
          <span>
            <strong>{provider.active_keys}</strong> 可用
          </span>
          <span>
            <strong>{provider.cooling_keys}</strong> 冷却
          </span>
          <span>
            <strong>{provider.invalid_keys}</strong> 失效
          </span>
          <span className="provider-card-stats-total">{`总 ${provider.total_keys}`}</span>
        </div>
      </div>

      {recentIssue ? (
        <div className="provider-card-issue" title={recentIssue.message}>
          <Tag color={levelColor(recentIssue.level)}>{recentIssue.level}</Tag>
          <span className="provider-card-issue-msg">{recentIssue.message}</span>
          <span className="provider-card-issue-time">{formatRelativeTime(recentIssue.at)}</span>
        </div>
      ) : null}

      <footer className="provider-card-actions">
        {provider.active ? (
          <Tooltip title="已是当前活跃 provider">
            <Button disabled>已活跃</Button>
          </Tooltip>
        ) : (
          <Button type="primary" loading={activating} onClick={onActivate}>
            设为活跃
          </Button>
        )}
        <Button onClick={onOpenDetail}>详情 →</Button>
      </footer>
    </article>
  );
}

type KpiBlockProps = {
  dashboard: AdminDashboardResponse;
  lastUpdated: string;
};

// KpiBlock 渲染右栏顶部的 KPI 大字，做主数据速读。
function KpiBlock({ dashboard, lastUpdated }: KpiBlockProps): JSX.Element {
  return (
    <Card className="surface-card kpi-card" bordered={false}>
      <div className="dashboard-section-head">
        <div>
          <Typography.Text className="placeholder-kicker">LIVE</Typography.Text>
          <Typography.Title level={4} className="section-title">
            当前 Provider 概况
          </Typography.Title>
        </div>
        <Tag>{lastUpdated}</Tag>
      </div>
      <div className="kpi-active-row">
        <HealthDot state={dashboard.active_keys > 0 ? "active" : "invalid"} pulse />
        <strong className="kpi-active-name" title={dashboard.active_provider}>
          {dashboard.active_provider || "未配置"}
        </strong>
      </div>
      <div className="kpi-grid">
        <div className="kpi-cell kpi-cell--green">
          <span className="kpi-label">可用</span>
          <strong className="kpi-value">{dashboard.active_keys}</strong>
        </div>
        <div className="kpi-cell kpi-cell--amber">
          <span className="kpi-label">冷却</span>
          <strong className="kpi-value">{dashboard.cooling_keys}</strong>
        </div>
        <div className="kpi-cell kpi-cell--red">
          <span className="kpi-label">失效</span>
          <strong className="kpi-value">{dashboard.invalid_keys}</strong>
        </div>
        <div className="kpi-cell kpi-cell--teal">
          <span className="kpi-label">Provider</span>
          <strong className="kpi-value">{dashboard.provider_count}</strong>
        </div>
      </div>
    </Card>
  );
}

type EventsRailProps = {
  events: AdminEvent[];
  onViewAll: () => void;
};

// EventsRail 渲染右栏滚动事件列表，提供"最近发生了什么"的速读。
function EventsRail({ events, onViewAll }: EventsRailProps): JSX.Element {
  return (
    <Card className="surface-card events-rail-card" bordered={false}>
      <div className="dashboard-section-head">
        <div>
          <Typography.Text className="placeholder-kicker">Recent Events</Typography.Text>
          <Typography.Title level={4} className="section-title">
            最近事件
          </Typography.Title>
        </div>
        <Button size="small" onClick={onViewAll}>
          全部 →
        </Button>
      </div>
      {events.length === 0 ? (
        <Empty description="暂无事件" image={Empty.PRESENTED_IMAGE_SIMPLE} />
      ) : (
        <ul className="events-rail-list">
          {events.map((event) => (
            <li key={`${event.seq}-${event.event}`} className={`events-rail-item events-rail-item--${event.level}`}>
              <span className="events-rail-time">{formatClockShort(new Date(event.at).getTime())}</span>
              <Tag color={levelColor(event.level)}>{event.level}</Tag>
              <span className="events-rail-msg" title={event.message}>
                {event.message}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

// applyOptimisticActivate 在乐观更新阶段把 provider 卡片切换到目标项，避免等待轮询。
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

// computeCardTone 把 provider 概况折算为卡片的色调，控制点位与边框。
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

// findRecentIssue 在事件流中找一条与 provider 匹配的最近异常，作为卡片底部一句话。
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

// levelColor 把事件 level 映射成 Antd Tag 配色。
function levelColor(level: string): string {
  if (level === "error") {
    return "red";
  }
  if (level === "warn") {
    return "gold";
  }
  return "blue";
}
