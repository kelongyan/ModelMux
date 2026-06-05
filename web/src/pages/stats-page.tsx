import { useQuery } from "@tanstack/react-query";
import { Button, Card, Col, Result, Row, Segmented, Space, Spin, Typography } from "antd";
import type { ReactElement } from "react";
import { useState } from "react";
import {
  Bar,
  BarChart,
  Cell,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { fetchStatsModels, fetchStatsSummary } from "../api/admin";
import type { AdminModelStats, AdminStatsSummary, AdminStatsWindow } from "../types/admin";

/* ── window picker ── */

const windowOptions: Array<{ label: string; value: AdminStatsWindow }> = [
  { label: "1 小时", value: "1h" },
  { label: "24 小时", value: "24h" },
  { label: "7 天", value: "7d" },
  { label: "30 天", value: "30d" },
];

/* ── palette ── */

const C = {
  blue:   "#6366f1",
  green:  "#10b981",
  red:    "#f43f5e",
  purple: "#8b5cf6",
  amber:  "#f59e0b",
  cyan:   "#06b6d4",
};

/* ── chart geometry ── */

const CHART_H = 300;
const MARGIN  = { top: 20, right: 24, left: 0, bottom: 8 };
const RADIUS: [number, number, number, number] = [8, 8, 0, 0];
const BAR_GAP = 6;
const CATEGORY_GAP = "24%";

/* ==================================================================
   Page
   ================================================================== */

export function StatsPage(): JSX.Element {
  const [window, setWindow] = useState<AdminStatsWindow>("24h");

  const summaryQuery = useQuery({
    queryKey: ["stats-summary", window],
    queryFn: () => fetchStatsSummary(window),
    refetchInterval: 10_000,
  });

  const modelsQuery = useQuery({
    queryKey: ["stats-models", window],
    queryFn: () => fetchStatsModels(window),
    refetchInterval: 10_000,
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
  const models  = modelsQuery.data?.models  ?? [];

  return (
    <Space direction="vertical" size={14} className="console-stack">

      {/* ── KPI row ── */}
      <Card className="surface-card" bordered={false}>
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">Usage</Typography.Text>
            <Typography.Title level={3} className="section-title">调用统计</Typography.Title>
          </div>
          <Space wrap>
            <Segmented value={window} options={windowOptions} onChange={(v) => setWindow(v as AdminStatsWindow)} />
            <Button onClick={() => void refetchAll()}>刷新</Button>
          </Space>
        </div>

        <Row gutter={[12, 12]}>
          <KPI label="调用数"          value={fmtNum(summary.total_calls)}         tone="blue"   />
          <KPI label="成功率"          value={fmtPct(summary.success_calls, summary.total_calls)} tone="green" />
          <KPI label="总 Token"        value={fmtNum(summary.total_tokens)}        tone="purple" />
          <KPI label="Prompt Token"    value={fmtNum(summary.prompt_tokens)}       tone="cyan"   />
          <KPI label="Completion Token" value={fmtNum(summary.completion_tokens)}  tone="amber"  />
          <KPI label="有 Token 记录"   value={fmtNum(summary.usage_known_calls)}   tone="blue"   />
          <KPI label="平均延迟"        value={`${Math.round(summary.avg_latency_ms)} ms`} tone="red" />
        </Row>
      </Card>

      {/* ── Charts ── */}
      {models.length === 0 ? null : (
        <div className="stats-chart-grid">

          <ChartCard title="模型调用量">
            <Chart data={models}>
              <Bar dataKey="calls" name="调用量" fill={C.blue} radius={RADIUS} barSize="60%" activeBar={false} />
            </Chart>
          </ChartCard>

          <ChartCard title="成功 / 失败对比">
            <Chart data={models}>
              <Legend content={<LegendRow pairs={[["成功", C.green], ["失败", C.red]]} />} />
              <Bar dataKey="success_calls" name="成功" fill={C.green} radius={RADIUS} barSize="55%" stackId="a" activeBar={false} />
              <Bar dataKey="failed_calls"  name="失败" fill={C.red}   radius={RADIUS} barSize="55%" stackId="a" activeBar={false} />
            </Chart>
          </ChartCard>

          <ChartCard title="Token 用量">
            <Chart data={models}>
              <Legend content={<LegendRow pairs={[["Prompt", C.purple], ["Completion", C.amber]]} />} />
              <Bar dataKey="prompt_tokens"     name="Prompt"     fill={C.purple} radius={RADIUS} barSize="55%" stackId="a" activeBar={false} />
              <Bar dataKey="completion_tokens" name="Completion" fill={C.amber}  radius={RADIUS} barSize="55%" stackId="a" activeBar={false} />
            </Chart>
          </ChartCard>

          <ChartCard title="平均延迟 (ms)">
            <Chart data={models}>
              <Bar dataKey="avg_latency_ms" name="平均延迟" fill={C.cyan} radius={RADIUS} barSize="60%" activeBar={false} />
            </Chart>
          </ChartCard>

        </div>
      )}
    </Space>
  );

  async function refetchAll() {
    await Promise.all([summaryQuery.refetch(), modelsQuery.refetch()]);
  }
}

/* ==================================================================
   Chart shell — tabIndex={-1} 彻底干掉 focus ring
   ================================================================== */

function Chart({ data, children }: { data: AdminModelStats[]; children: React.ReactNode }): ReactElement {
  return (
    <ResponsiveContainer width="100%" height={CHART_H}>
      <BarChart
        data={data}
        margin={MARGIN}
        barCategoryGap={CATEGORY_GAP}
        barGap={BAR_GAP}
        tabIndex={-1}
      >
        <XAxis
          dataKey="model"
          tick={{ fontSize: 12, fill: "#475569", fontWeight: 500 }}
          tickLine={false}
          axisLine={{ stroke: "#e2e8f0", strokeWidth: 1 }}
          tickFormatter={truncate}
          interval={0}
          dy={6}
        />
        <YAxis
          tick={{ fontSize: 11, fill: "#94a3b8" }}
          tickLine={false}
          axisLine={false}
          width={52}
          allowDecimals={false}
        />
        <Tooltip
          content={<Tip />}
          cursor={{ fill: "rgba(99,102,241,0.06)", radius: 4 }}
          wrapperStyle={{ outline: "none" }}
        />
        {children}
      </BarChart>
    </ResponsiveContainer>
  );
}

/* ==================================================================
   Dark tooltip
   ================================================================== */

function Tip({ active, payload, label }: any): ReactElement | null {
  if (!active || !payload || payload.length === 0) return null;

  return (
    <div className="stats-tip">
      <div className="stats-tip-label">{label}</div>
      {payload.map((e: any, i: number) => (
        <div key={i} className="stats-tip-row">
          <span className="stats-tip-dot" style={{ background: e.color }} />
          <span className="stats-tip-name">{e.name}</span>
          <span className="stats-tip-val">
            {typeof e.value === "number" ? fmtNum(e.value) : String(e.value)}
          </span>
        </div>
      ))}
    </div>
  );
}

/* ==================================================================
   Inline legend
   ================================================================== */

function LegendRow({ pairs }: { pairs: Array<[string, string]> }): ReactElement {
  return (
    <div className="stats-legend">
      {pairs.map(([name, color]) => (
        <div key={name} className="stats-legend-item">
          <span className="stats-legend-dot" style={{ background: color }} />
          <span>{name}</span>
        </div>
      ))}
    </div>
  );
}

/* ==================================================================
   KPI card
   ================================================================== */

type Tone = "blue" | "green" | "purple" | "amber" | "red" | "cyan";
const TONE_ACCENT: Record<Tone, string> = {
  blue:   "#6366f1",
  green:  "#10b981",
  purple: "#8b5cf6",
  amber:  "#f59e0b",
  red:    "#f43f5e",
  cyan:   "#06b6d4",
};

function KPI({ label, value, tone }: { label: string; value: string; tone: Tone }): JSX.Element {
  return (
    <Col xs={24} sm={12} xl={8}>
      <div className="stats-kpi" style={{ "--kpi-accent": TONE_ACCENT[tone] } as React.CSSProperties}>
        <span>{label}</span>
        <strong>{value}</strong>
      </div>
    </Col>
  );
}

/* ==================================================================
   Chart card wrapper
   ================================================================== */

function ChartCard({ title, children }: { title: string; children: React.ReactNode }): JSX.Element {
  return (
    <Card className="surface-card stats-chart-card" bordered={false}>
      <Typography.Text className="stats-chart-title">{title}</Typography.Text>
      {children}
    </Card>
  );
}

/* ==================================================================
   Helpers
   ================================================================== */

function emptySummary(): AdminStatsSummary {
  return {
    total_calls: 0, success_calls: 0, failed_calls: 0,
    usage_known_calls: 0, prompt_tokens: 0, completion_tokens: 0,
    total_tokens: 0, avg_latency_ms: 0,
  };
}

function fmtPct(part: number, total: number): string {
  return total <= 0 ? "0%" : `${Math.round((part / total) * 100)}%`;
}

function fmtNum(v: number): string {
  return new Intl.NumberFormat("zh-CN").format(v);
}

function truncate(v: string): string {
  return v.length > 18 ? v.slice(0, 16) + "…" : v;
}
