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
  green: "#5db872",
  purple: "#B87333",
  red: "#c64545",
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
        <KPI label="调用数" value={formatNumber(summary.total_calls)} tone="blue" />
        <KPI label="总 Token" value={formatNumber(summary.total_tokens)} tone="purple" />
        <KPI label="成功率" value={formatPercent(summary.success_calls, summary.total_calls)} tone="green" />
        <KPI label="平均延迟" value={formatLatencySec(summary.avg_latency_ms)} tone="red" />
      </Row>
      </Spin>
    </Card>
  );
}

function KPI({ label, value, tone }: { label: string; value: string; tone: Tone }): JSX.Element {
  return (
    <Col xs={24} sm={12} md={6}>
      <div className="stats-kpi" style={{ "--kpi-accent": toneAccent[tone] } as React.CSSProperties}>
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
    </Col>
  );
}
