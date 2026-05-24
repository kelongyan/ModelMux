import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Col, Empty, Result, Row, Space, Spin, Tag, Typography, message } from "antd";
import { startTransition } from "react";

import { fetchDashboard, triggerReload } from "../api/admin";
import type { AdminDashboardResponse, AdminProviderSummary } from "../types/admin";

const dashboardQueryKey = ["dashboard"];

// DashboardPage 渲染真实数据驱动的首页，包含轮询刷新与手动 reload 快捷操作。
export function DashboardPage(): JSX.Element {
  const queryClient = useQueryClient();
  const dashboardQuery = useQuery({
    queryKey: dashboardQueryKey,
    queryFn: fetchDashboard,
    refetchInterval: 5000,
  });

  const reloadMutation = useMutation({
    mutationFn: triggerReload,
    onSuccess: async () => {
      message.success("已触发配置重载");
      startTransition(() => {
        void queryClient.invalidateQueries({ queryKey: dashboardQueryKey });
      });
    },
    onError: (error: Error) => {
      message.error(`重载失败：${error.message}`);
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
  const metrics = buildDashboardMetrics(dashboard);

  return (
    <Space direction="vertical" size={24} className="console-stack">
      <section className="hero-panel">
        <div className="hero-copy">
          <Typography.Text className="hero-kicker">总览</Typography.Text>
          <Typography.Title level={3} className="hero-title">
            实时状态
          </Typography.Title>
          <Space wrap>
            <Button
              type="primary"
              loading={reloadMutation.isPending}
              onClick={() => reloadMutation.mutate()}
            >
              立即重载配置
            </Button>
            <Button onClick={() => void dashboardQuery.refetch()}>手动刷新</Button>
          </Space>
        </div>
        <div className="hero-status">
          <span>当前活跃 Provider</span>
          <strong>{dashboard.active_provider}</strong>
          <small>{`最近刷新：${formatClock(dashboardQuery.dataUpdatedAt)}`}</small>
        </div>
      </section>

      <Row gutter={[18, 18]}>
        {metrics.map((metric) => (
          <Col xs={24} sm={12} xl={6} key={metric.label}>
            <MetricCard label={metric.label} value={metric.value} accent={metric.accent} />
          </Col>
        ))}
      </Row>

      <Row gutter={[18, 18]}>
        <Col xs={24} xl={15}>
          <Card className="surface-card" bordered={false} title="Provider 状态速览">
            {dashboard.providers.length === 0 ? (
              <Empty description="当前没有可展示的 provider" />
            ) : (
              <div className="provider-preview-list">
                {dashboard.providers.map((provider) => (
                  <ProviderSummaryItem key={provider.id} provider={provider} />
                ))}
              </div>
            )}
          </Card>
        </Col>
        <Col xs={24} xl={9}>
          <Card className="surface-card" bordered={false} title="最近事件">
            {dashboard.events.length === 0 ? (
              <Empty description="当前没有事件" />
            ) : (
              <ul className="event-preview-list">
                {dashboard.events.map((event) => (
                  <li key={`${event.seq}-${event.event}`}>
                    <strong>{event.message}</strong>
                    <span>{`${event.category} · ${event.event}`}</span>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </Col>
      </Row>
    </Space>
  );
}

type MetricCardProps = {
  label: string;
  value: string;
  accent: "amber" | "teal" | "green" | "red";
};

// MetricCard 渲染首页统计卡片，统一控制视觉风格。
function MetricCard({ label, value, accent }: MetricCardProps): JSX.Element {
  return (
    <Card className={`metric-card accent-${accent}`} bordered={false}>
      <Typography.Text className="metric-label">{label}</Typography.Text>
      <Typography.Title level={2} className="metric-value">
        {value}
      </Typography.Title>
    </Card>
  );
}

type ProviderSummaryItemProps = {
  provider: AdminProviderSummary;
};

// ProviderSummaryItem 渲染单个 provider 的状态摘要，供 dashboard 快速浏览。
function ProviderSummaryItem({ provider }: ProviderSummaryItemProps): JSX.Element {
  return (
    <div className="provider-preview-item">
      <div>
        <div className="provider-preview-head">
          <strong>{provider.id}</strong>
          {provider.active ? <Tag color="green">当前活跃</Tag> : <Tag>待命</Tag>}
        </div>
        <p>{provider.target_url}</p>
        <div className="provider-stat-row">
          <span>{`总 key ${provider.total_keys}`}</span>
          <span>{`可用 ${provider.active_keys}`}</span>
          <span>{`冷却 ${provider.cooling_keys}`}</span>
          <span>{`失效 ${provider.invalid_keys}`}</span>
        </div>
      </div>
    </div>
  );
}

type DashboardMetric = {
  label: string;
  value: string;
  accent: "amber" | "teal" | "green" | "red";
};

// buildDashboardMetrics 把 dashboard 响应整理为 UI 直接可消费的统计卡片模型。
function buildDashboardMetrics(dashboard: AdminDashboardResponse): DashboardMetric[] {
  return [
    { label: "当前 Provider", value: dashboard.active_provider, accent: "amber" },
    { label: "Provider 总数", value: String(dashboard.provider_count), accent: "teal" },
    { label: "可用 Key", value: String(dashboard.active_keys), accent: "green" },
    { label: "冷却 / 失效", value: `${dashboard.cooling_keys} / ${dashboard.invalid_keys}`, accent: "red" },
  ];
}

// formatClock 把 query 更新时间格式化为前端页头中的简短时钟文本。
function formatClock(timestamp: number): string {
  if (!timestamp) {
    return "--:--:--";
  }
  return new Date(timestamp).toLocaleTimeString("zh-CN", {
    hour12: false,
  });
}
