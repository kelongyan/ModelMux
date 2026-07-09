import { Button, Card, Col, Row, Segmented, Space, Spin, Typography } from "antd";

import type { AdminStatsSummary, AdminStatsWindow } from "../../types/admin";
import { formatNumber, formatLatencySec, formatPercent } from "./stats-format";
import { statsWindowOptions } from "./stats-options";

type StatsSummaryCardProps = {
  summary: AdminStatsSummary;
  droppedRecords: number;
  queueDepth: number;
  queueCapacity: number;
  window: AdminStatsWindow;
  loading?: boolean;
  onWindowChange: (window: AdminStatsWindow) => void;
  onRefresh: () => void;
};

type Tone = "blue" | "green" | "purple" | "red";

const toneAccent: Record<Tone, string> = {
  blue: "var(--mm-primary)",
  green: "var(--mm-success)",
  purple: "#39C5CF",
  red: "var(--mm-signal)",
};

export function StatsSummaryCard({
  summary,
  droppedRecords,
  queueDepth,
  queueCapacity,
  window,
  loading,
  onWindowChange,
  onRefresh,
}: StatsSummaryCardProps): JSX.Element {
  return (
    <Card className="surface-card reveal-card reveal-delay-0" bordered={false}>
      <Spin spinning={!!loading}>
      <div className="section-heading">
        <div>
          <Typography.Text className="placeholder-kicker">Usage</Typography.Text>
          <Typography.Title level={3} className="section-title">调用统计</Typography.Title>
        </div>
        <Space wrap>
          <Typography.Text className={droppedRecords > 0 ? "stats-dropped-counter stats-dropped-counter--warn" : "stats-dropped-counter"}>
            队列 {formatNumber(queueDepth)}/{formatNumber(queueCapacity)} · 丢弃 {formatNumber(droppedRecords)}
          </Typography.Text>
          <Segmented value={window} options={statsWindowOptions} onChange={(value) => onWindowChange(value as AdminStatsWindow)} />
          <Button onClick={onRefresh}>刷新</Button>
        </Space>
      </div>

      <Row gutter={[20, 20]}>
        <KPI label="调用数" value={formatNumber(summary.total_calls)} detail={`已识别 ${formatNumber(summary.usage_known_calls)} 次`} tone="blue" />
        <KPI label="总 Token" value={formatNumber(summary.total_tokens)} detail={`输入 ${formatNumber(summary.prompt_tokens)} · 输出 ${formatNumber(summary.completion_tokens)}`} tone="purple" />
        <KPI label="输入 Token" value={formatNumber(summary.prompt_tokens)} tone="blue" />
        <KPI label="输出 Token" value={formatNumber(summary.completion_tokens)} tone="purple" />
        <KPI label="成功率" value={formatPercent(summary.success_calls, summary.total_calls)} detail={`${formatNumber(summary.success_calls)} 成功 / ${formatNumber(summary.failed_calls)} 失败`} tone="green" />
        <KPI label="平均延迟" value={formatLatencySec(summary.avg_latency_ms)} tone="red" />
      </Row>
      </Spin>
    </Card>
  );
}

function KPI({ label, value, detail, tone }: { label: string; value: string; detail?: string; tone: Tone }): JSX.Element {
  return (
    <Col xs={24} sm={12} lg={8} xl={4}>
      <div className="stats-kpi" style={{ "--kpi-accent": toneAccent[tone] } as React.CSSProperties}>
        <span>{label}</span>
        <strong>{value}</strong>
        <small>{detail ?? "\u00A0"}</small>
      </div>
    </Col>
  );
}
