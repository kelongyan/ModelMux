import { useQuery } from "@tanstack/react-query";
import { Button, Card, Col, Result, Row, Segmented, Space, Spin, Typography } from "antd";
import { useState } from "react";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { fetchStatsModels, fetchStatsSummary } from "../api/admin";
import type { AdminStatsSummary, AdminStatsWindow } from "../types/admin";

const windowOptions: Array<{ label: string; value: AdminStatsWindow }> = [
  { label: "1 小时", value: "1h" },
  { label: "24 小时", value: "24h" },
  { label: "7 天", value: "7d" },
  { label: "30 天", value: "30d" },
];

// StatsPage 展示调用明细持久化后的 token 使用情况。
export function StatsPage(): JSX.Element {
  const [window, setWindow] = useState<AdminStatsWindow>("24h");

  const summaryQuery = useQuery({
    queryKey: ["stats-summary", window],
    queryFn: () => fetchStatsSummary(window),
    refetchInterval: 10000,
  });

  const modelsQuery = useQuery({
    queryKey: ["stats-models", window],
    queryFn: () => fetchStatsModels(window),
    refetchInterval: 10000,
  });

  if (summaryQuery.isLoading || modelsQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (summaryQuery.isError || modelsQuery.isError) {
    const error = summaryQuery.error ?? modelsQuery.error;
    return (
      <Result
        status="error"
        title="调用统计加载失败"
        subTitle={error instanceof Error ? error.message : "未知错误"}
        extra={<Button onClick={() => void refetchAll()}>重新获取</Button>}
      />
    );
  }

  const summary = summaryQuery.data?.summary ?? emptySummary();
  const models = modelsQuery.data?.models ?? [];

  return (
    <Space direction="vertical" size={12} className="console-stack">
      <Card className="surface-card" bordered={false}>
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">Usage</Typography.Text>
            <Typography.Title level={3} className="section-title">
              调用统计
            </Typography.Title>
          </div>
          <Space wrap>
            <Segmented
              value={window}
              options={windowOptions}
              onChange={(value) => setWindow(value as AdminStatsWindow)}
            />
            <Button onClick={() => void refetchAll()}>刷新</Button>
          </Space>
        </div>

        <Row gutter={[12, 12]}>
          <StatsKPI label="调用数" value={formatNumber(summary.total_calls)} tone="teal" />
          <StatsKPI label="成功率" value={formatPercent(summary.success_calls, summary.total_calls)} tone="green" />
          <StatsKPI label="总 Token" value={formatNumber(summary.total_tokens)} tone="amber" />
          <StatsKPI label="Prompt Token" value={formatNumber(summary.prompt_tokens)} tone="teal" />
          <StatsKPI label="Completion Token" value={formatNumber(summary.completion_tokens)} tone="green" />
          <StatsKPI label="有 Token 记录" value={formatNumber(summary.usage_known_calls)} tone="amber" />
          <StatsKPI label="平均延迟" value={`${Math.round(summary.avg_latency_ms)} ms`} tone="red" />
        </Row>
      </Card>

      {models.length > 0 && (
        <div className="stats-chart-grid">
          <ChartCard title="模型调用量">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={models} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis dataKey="model" tick={{ fontSize: 12 }} tickFormatter={truncateLabel} />
                <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                <Tooltip formatter={(v) => formatNumber(Number(v))} />
                <Bar dataKey="calls" name="调用量" fill="#3b82f6" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </ChartCard>

          <ChartCard title="成功 / 失败对比">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={models} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis dataKey="model" tick={{ fontSize: 12 }} tickFormatter={truncateLabel} />
                <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                <Tooltip formatter={(v) => formatNumber(Number(v))} />
                <Legend />
                <Bar dataKey="success_calls" name="成功" fill="#22c55e" radius={[4, 4, 0, 0]} stackId="a" />
                <Bar dataKey="failed_calls" name="失败" fill="#ef4444" radius={[4, 4, 0, 0]} stackId="a" />
              </BarChart>
            </ResponsiveContainer>
          </ChartCard>

          <ChartCard title="Token 用量">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={models} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis dataKey="model" tick={{ fontSize: 12 }} tickFormatter={truncateLabel} />
                <YAxis tick={{ fontSize: 12 }} allowDecimals={false} />
                <Tooltip formatter={(v) => formatNumber(Number(v))} />
                <Legend />
                <Bar dataKey="prompt_tokens" name="Prompt" fill="#8b5cf6" radius={[4, 4, 0, 0]} />
                <Bar dataKey="completion_tokens" name="Completion" fill="#f59e0b" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </ChartCard>

          <ChartCard title="平均延迟 (ms)">
            <ResponsiveContainer width="100%" height={260}>
              <BarChart data={models} margin={{ top: 8, right: 12, left: 0, bottom: 4 }}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis dataKey="model" tick={{ fontSize: 12 }} tickFormatter={truncateLabel} />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip formatter={(v) => `${Math.round(Number(v))} ms`} />
                <Bar dataKey="avg_latency_ms" name="平均延迟" fill="#06b6d4" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </ChartCard>
        </div>
      )}
    </Space>
  );

  async function refetchAll() {
    await Promise.all([summaryQuery.refetch(), modelsQuery.refetch()]);
  }
}

type StatsKPIProps = {
  label: string;
  value: string;
  tone: "teal" | "green" | "amber" | "red";
};

function StatsKPI({ label, value, tone }: StatsKPIProps): JSX.Element {
  return (
    <Col xs={24} sm={12} xl={8}>
      <div className={`stats-kpi stats-kpi--${tone}`}>
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
    </Col>
  );
}

function emptySummary(): AdminStatsSummary {
  return {
    total_calls: 0,
    success_calls: 0,
    failed_calls: 0,
    usage_known_calls: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    total_tokens: 0,
    avg_latency_ms: 0,
  };
}

function formatPercent(part: number, total: number): string {
  if (total <= 0) {
    return "0%";
  }
  return `${Math.round((part / total) * 100)}%`;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat("zh-CN").format(value);
}

function truncateLabel(value: string): string {
  return value.length > 14 ? value.slice(0, 12) + "…" : value;
}

function ChartCard({ title, children }: { title: string; children: React.ReactNode }): JSX.Element {
  return (
    <Card className="surface-card stats-chart-card" bordered={false}>
      <Typography.Text className="stats-chart-title">{title}</Typography.Text>
      {children}
    </Card>
  );
}
