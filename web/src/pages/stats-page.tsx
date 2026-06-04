import { useQuery } from "@tanstack/react-query";
import { Button, Card, Col, Empty, Result, Row, Segmented, Space, Spin, Table, Tag, Typography } from "antd";
import type { TableColumnsType } from "antd";
import { useMemo, useState } from "react";

import { fetchStatsModels, fetchStatsRecent, fetchStatsSummary } from "../api/admin";
import { formatDateTime } from "../components/format-time";
import type { AdminCallRecord, AdminModelStats, AdminStatsSummary, AdminStatsWindow } from "../types/admin";

const windowOptions: Array<{ label: string; value: AdminStatsWindow }> = [
  { label: "1 小时", value: "1h" },
  { label: "24 小时", value: "24h" },
  { label: "7 天", value: "7d" },
  { label: "30 天", value: "30d" },
];

// StatsPage 展示调用明细持久化后的模型与 token 使用情况。
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
  const recentQuery = useQuery({
    queryKey: ["stats-recent", 100],
    queryFn: () => fetchStatsRecent(100),
    refetchInterval: 10000,
  });

  const modelColumns = useMemo<TableColumnsType<AdminModelStats>>(
    () => [
      {
        title: "模型",
        dataIndex: "model",
        key: "model",
        render: (model: string) => <strong>{displayModel(model)}</strong>,
      },
      {
        title: "调用",
        dataIndex: "calls",
        key: "calls",
        width: 92,
        render: (value: number) => formatNumber(value),
      },
      {
        title: "成功率",
        key: "success_rate",
        width: 110,
        render: (_: unknown, record) => successRateTag(record.success_calls, record.calls),
      },
      {
        title: "Token",
        dataIndex: "total_tokens",
        key: "total_tokens",
        width: 120,
        render: (value: number) => formatNumber(value),
      },
      {
        title: "平均延迟",
        dataIndex: "avg_latency_ms",
        key: "avg_latency_ms",
        width: 120,
        render: (value: number) => `${Math.round(value)} ms`,
      },
    ],
    [],
  );

  const recentColumns = useMemo<TableColumnsType<AdminCallRecord>>(
    () => [
      {
        title: "时间",
        dataIndex: "at",
        key: "at",
        width: 170,
        render: (value: string) => <span className="stats-time">{formatDateTime(value)}</span>,
      },
      {
        title: "模型",
        dataIndex: "model",
        key: "model",
        render: (model: string | undefined) => displayModel(model),
      },
      {
        title: "Provider",
        dataIndex: "provider_id",
        key: "provider_id",
        width: 120,
      },
      {
        title: "状态",
        key: "status",
        width: 100,
        render: (_: unknown, record) => <Tag color={record.success ? "green" : "red"}>{record.status || "ERR"}</Tag>,
      },
      {
        title: "模式",
        key: "stream",
        width: 92,
        render: (_: unknown, record) => (record.stream ? <Tag color="blue">stream</Tag> : <Tag>normal</Tag>),
      },
      {
        title: "Prompt",
        dataIndex: "prompt_tokens",
        key: "prompt_tokens",
        width: 100,
        render: (value: number | undefined) => tokenValue(value),
      },
      {
        title: "Completion",
        dataIndex: "completion_tokens",
        key: "completion_tokens",
        width: 120,
        render: (value: number | undefined) => tokenValue(value),
      },
      {
        title: "总 Token",
        dataIndex: "total_tokens",
        key: "total_tokens",
        width: 110,
        render: (value: number | undefined) => tokenValue(value),
      },
      {
        title: "来源",
        dataIndex: "usage_source",
        key: "usage_source",
        width: 100,
        render: (value: string | undefined) => usageSourceTag(value),
      },
      {
        title: "延迟",
        dataIndex: "latency_ms",
        key: "latency_ms",
        width: 100,
        render: (value: number) => `${value} ms`,
      },
      {
        title: "尝试",
        dataIndex: "attempts",
        key: "attempts",
        width: 80,
      },
    ],
    [],
  );

  if (summaryQuery.isLoading || modelsQuery.isLoading || recentQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (summaryQuery.isError || modelsQuery.isError || recentQuery.isError) {
    const error = summaryQuery.error ?? modelsQuery.error ?? recentQuery.error;
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
  const recent = recentQuery.data?.records ?? [];

  return (
    <Space direction="vertical" size={16} className="console-stack">
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

        <div className="events-summary-row">
          <span className="summary-pill">{`窗口 ${windowLabel(window)}`}</span>
          <span className="summary-pill">{`模型 ${models.length}`}</span>
          <span className="summary-pill">{`最近记录 ${recent.length}`}</span>
          <span className="summary-pill">{`成功 ${summary.success_calls} / 失败 ${summary.failed_calls}`}</span>
        </div>

        <Row gutter={[14, 14]}>
          <StatsKPI label="调用数" value={formatNumber(summary.total_calls)} tone="teal" />
          <StatsKPI label="成功率" value={formatPercent(summary.success_calls, summary.total_calls)} tone="green" />
          <StatsKPI label="总 Token" value={formatNumber(summary.total_tokens)} tone="amber" />
          <StatsKPI label="Prompt Token" value={formatNumber(summary.prompt_tokens)} tone="teal" />
          <StatsKPI label="Completion Token" value={formatNumber(summary.completion_tokens)} tone="green" />
          <StatsKPI label="有 Token 记录" value={formatNumber(summary.usage_known_calls)} tone="amber" />
          <StatsKPI label="平均延迟" value={`${Math.round(summary.avg_latency_ms)} ms`} tone="red" />
        </Row>
      </Card>

      <Row gutter={[18, 18]}>
        <Col xs={24} xl={10}>
          <Card className="surface-card" bordered={false}>
            <div className="section-heading">
              <div>
                <Typography.Text className="placeholder-kicker">Models</Typography.Text>
                <Typography.Title level={3} className="section-title">
                  模型排行
                </Typography.Title>
              </div>
              <Tag>{windowLabel(window)}</Tag>
            </div>
            {models.length === 0 ? (
              <Empty description="当前窗口暂无模型调用" />
            ) : (
              <Table
                className="stats-table"
                columns={modelColumns}
                dataSource={models}
                pagination={false}
                size="small"
                rowKey={(record) => record.model}
              />
            )}
          </Card>
        </Col>

        <Col xs={24} xl={14}>
          <Card className="surface-card" bordered={false}>
            <div className="section-heading">
              <div>
                <Typography.Text className="placeholder-kicker">Recent Calls</Typography.Text>
                <Typography.Title level={3} className="section-title">
                  最近调用
                </Typography.Title>
              </div>
              <Tag>{`最近 ${recent.length} 条`}</Tag>
            </div>
            {recent.length === 0 ? (
              <Empty description="暂无调用明细" />
            ) : (
              <Table
                className="stats-table"
                columns={recentColumns}
                dataSource={[...recent].reverse()}
                pagination={{ pageSize: 20, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
                scroll={{ x: 1210 }}
                size="small"
                rowKey={(record) => record.id}
              />
            )}
          </Card>
        </Col>
      </Row>
    </Space>
  );

  async function refetchAll() {
    await Promise.all([summaryQuery.refetch(), modelsQuery.refetch(), recentQuery.refetch()]);
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

function displayModel(model: string | undefined): string {
  return model && model !== "unknown" ? model : "未知模型";
}

function tokenValue(value: number | undefined | null): JSX.Element {
  if (value === undefined || value === null) {
    return <Tag>未知</Tag>;
  }
  return <span>{formatNumber(value)}</span>;
}

function usageSourceTag(source: string | undefined): JSX.Element {
  if (source === "upstream") {
    return <Tag color="green">upstream</Tag>;
  }
  return <Tag color="gold">unknown</Tag>;
}

function successRateTag(success: number, total: number): JSX.Element {
  const value = formatPercent(success, total);
  const color = total === 0 || success === total ? "green" : success === 0 ? "red" : "gold";
  return <Tag color={color}>{value}</Tag>;
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

function windowLabel(value: AdminStatsWindow): string {
  return windowOptions.find((item) => item.value === value)?.label ?? value;
}
