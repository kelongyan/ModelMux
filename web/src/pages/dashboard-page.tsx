import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Empty, Popconfirm, Result, Skeleton, Space, Typography, message } from "antd";
import { startTransition } from "react";
import { useNavigate } from "react-router-dom";

import { activateProvider, deleteProvider, fetchDashboard, triggerReload } from "../api/admin";
import { queryKeys } from "../api/query-keys";
import { formatClockShort } from "../components/format-time";
import { HealthDot } from "../components/health-dot";
import { useVisibilityRefetchInterval } from "../components/use-visibility-polling";
import type { AdminDashboardResponse, AdminProviderSummary } from "../types/admin";

export function DashboardPage(): JSX.Element {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [messageApi, contextHolder] = message.useMessage();
  const pollInterval = useVisibilityRefetchInterval(5000);

  const dashboardQuery = useQuery({
    queryKey: queryKeys.dashboard,
    queryFn: fetchDashboard,
    refetchInterval: pollInterval,
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

  const deleteMutation = useMutation({
    mutationFn: deleteProvider,
    onSuccess: async (_, providerID) => {
      messageApi.success(`已删除 provider ${providerID}`);
      startTransition(() => {
        void queryClient.invalidateQueries({ queryKey: queryKeys.dashboard });
        void queryClient.invalidateQueries({ queryKey: queryKeys.providers });
      });
    },
    onError: (error: Error) => {
      messageApi.error(`删除失败：${error.message}`);
    },
  });

  if (dashboardQuery.isLoading) {
    return (
      <div className="console-loading">
        <Skeleton active paragraph={{ rows: 10 }} />
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
      <Space direction="vertical" size={20} className="console-stack">

        {/* ── Status card ── */}
        <Card className="surface-card reveal-card reveal-delay-0" bordered={false}>
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
                size="middle"
                type="primary"
                loading={reloadMutation.isPending}
                onClick={() => reloadMutation.mutate()}
              >
                立即重载
              </Button>
              <Button size="middle" onClick={() => void dashboardQuery.refetch()}>刷新</Button>
            </Space>
          </div>

          {/* Active provider + health */}
          <div className="dashboard-hero-row">
            <div className="dashboard-active-panel">
              <span className="dashboard-panel-label">Active Mission · 当前活跃 Provider</span>
              <div className="dashboard-active-provider-row">
                <HealthDot state={computeOverviewState(dashboard)} pulse={dashboard.active_keys > 0} />
                <strong className="dashboard-active-provider-name" title={dashboard.active_provider}>
                  {dashboard.active_provider || "未配置"}
                </strong>
              </div>
              <span className="dashboard-panel-desc">
                {dashboard.provider_count === 0
                  ? "还没有配置 provider，先去提供商页面新增上游。"
                  : `共 ${dashboard.provider_count} 个 Provider · 可用 ${dashboard.active_keys} 个 Key · 冷却 ${dashboard.cooling_keys} · 失效 ${dashboard.invalid_keys}`}
              </span>
            </div>
            <div className="dashboard-health-strip">
              <div className="dashboard-health-signal dashboard-health-signal--ok">
                <span>可用 Key</span>
                <strong>{dashboard.active_keys}</strong>
              </div>
              <div className={`dashboard-health-signal ${dashboard.cooling_keys > 0 ? "dashboard-health-signal--warn" : ""}`}>
                <span>冷却 Key</span>
                <strong>{dashboard.cooling_keys}</strong>
              </div>
              <div className={`dashboard-health-signal ${dashboard.invalid_keys > 0 ? "dashboard-health-signal--err" : ""}`}>
                <span>失效 Key</span>
                <strong>{dashboard.invalid_keys}</strong>
              </div>
              <div className="dashboard-health-signal">
                <span>Provider</span>
                <strong>{dashboard.provider_count}</strong>
              </div>
            </div>
          </div>
        </Card>

        {/* ── Provider list ── */}
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
                  deleting={deleteMutation.isPending && deleteMutation.variables === provider.id}
                  onActivate={() => activateMutation.mutate(provider.id)}
                  onDelete={() => deleteMutation.mutate(provider.id)}
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

type ProviderRowProps = {
  provider: AdminProviderSummary;
  activating: boolean;
  deleting: boolean;
  onActivate: () => void;
  onDelete: () => void;
  onOpenDetail: () => void;
};

function ProviderRow({ provider, activating, deleting, onActivate, onDelete, onOpenDetail }: ProviderRowProps): JSX.Element {
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
        {!provider.active ? (
          <Popconfirm
            title={`确认删除 provider ${provider.id}？`}
            description="删除后将同时移除其全部 keys。"
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true }}
            onConfirm={onDelete}
          >
            <Button size="small" type="link" danger loading={deleting}>删除</Button>
          </Popconfirm>
        ) : null}
      </div>
    </div>
  );
}

function providerStateLabel(provider: AdminProviderSummary, tone: "active" | "cooling" | "invalid" | "idle"): string {
  if (provider.active) return "当前活跃";
  if (tone === "invalid") return "不可用";
  if (tone === "cooling") return "波动中";
  return "待命";
}

function computeOverviewState(dashboard: AdminDashboardResponse): "active" | "cooling" | "invalid" | "idle" {
  if (dashboard.provider_circuit?.state === "open") return "invalid";
  if (dashboard.provider_circuit?.state === "half_open") return "cooling";
  if (!dashboard.active_provider) return "idle";
  if (dashboard.active_keys === 0) return dashboard.cooling_keys > 0 ? "cooling" : "invalid";
  if (dashboard.cooling_keys > 0 || dashboard.invalid_keys > 0) return "cooling";
  return "active";
}

function applyOptimisticActivate(prev: AdminDashboardResponse, providerID: string): AdminDashboardResponse {
  const target = prev.providers.find((p) => p.id === providerID);
  if (!target) return prev;
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
  if (provider.active_keys === 0 && provider.cooling_keys === 0) return "invalid";
  if (provider.active_keys === 0) return "cooling";
  if (provider.cooling_keys > 0 || provider.invalid_keys > 0) return provider.active ? "cooling" : "idle";
  return provider.active ? "active" : "idle";
}
