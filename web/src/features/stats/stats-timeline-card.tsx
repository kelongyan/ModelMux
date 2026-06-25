import { useQuery } from "@tanstack/react-query";
import { Card, Empty, Segmented, Spin } from "antd";
import type { Dayjs } from "dayjs";
import { useMemo, useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { fetchStatsTimeline } from "../../api/admin";
import { queryKeys } from "../../api/query-keys";
import { useVisibilityRefetchInterval } from "../../components/use-visibility-polling";
import type { AdminTimelinePoint, AdminStatsWindow } from "../../types/admin";
import { formatNumber } from "./stats-format";
import { formatTimelineTime, pickTimelineGranularity } from "./stats-options";
import "./stats-timeline-card.css";

type TimelineView = "calls" | "latency" | "tokens";

interface StatsTimelineCardProps {
  window: AdminStatsWindow;
  dateRange?: [Dayjs | null, Dayjs | null];
}

const COLORS = {
  success: "#52c41a",
  failed: "#ff4d4f",
  latency: "#fa8c16",
  tokens: "#1677ff",
} as const;

export function StatsTimelineCard({ window, dateRange }: StatsTimelineCardProps): JSX.Element {
  const [view, setView] = useState<TimelineView>("calls");

  const pollInterval = useVisibilityRefetchInterval(10_000);
  const granularity = pickTimelineGranularity(window);

  const customRangeParams = useMemo(() => {
    if (dateRange?.[0] && dateRange?.[1]) {
      return {
        from: dateRange[0].toISOString(),
        to: dateRange[1].toISOString(),
      };
    }
    return undefined;
  }, [dateRange]);

  const timelineQuery = useQuery({
    queryKey: customRangeParams
      ? queryKeys.statsTimeline("custom" as AdminStatsWindow, granularity)
      : queryKeys.statsTimeline(window, granularity),
    queryFn: () => fetchStatsTimeline(window, granularity, customRangeParams),
    refetchInterval: pollInterval,
  });

  const timeline = timelineQuery.data?.timeline ?? [];

  const formattedData = useMemo(() => {
    return timeline.map((p: AdminTimelinePoint) => ({
      ...p,
      timeLabel: formatTimelineTime(p.time, window),
    }));
  }, [timeline, window]);

  const empty = !timelineQuery.isLoading && !timelineQuery.isError && formattedData.length === 0;

  return (
    <Card
      title="调用趋势"
      className="stats-timeline-card"
      extra={
        <Segmented
          size="small"
          value={view}
          options={[
            { label: "调用量", value: "calls" },
            { label: "延迟", value: "latency" },
            { label: "Token", value: "tokens" },
          ]}
          onChange={(val) => setView(val as TimelineView)}
        />
      }
    >
      <Spin spinning={timelineQuery.isLoading}>
        {empty ? (
          <Empty description="暂无趋势数据" />
        ) : (
          <div className="stats-timeline-chart">
            <ResponsiveContainer width="100%" height={320}>
              {view === "calls" ? (
                <AreaChart data={formattedData} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="successGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={COLORS.success} stopOpacity={0.3} />
                      <stop offset="100%" stopColor={COLORS.success} stopOpacity={0.02} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--mm-border-muted)" />
                  <XAxis
                    dataKey="timeLabel"
                    tick={{ fontSize: 12, fill: "var(--mm-text-subtle)" }}
                    tickLine={false}
                    axisLine={{ stroke: "var(--mm-border-muted)" }}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={{ fontSize: 12, fill: "var(--mm-text-subtle)" }}
                    tickLine={false}
                    axisLine={false}
                    width={50}
                  />
                  <Tooltip content={<TimelineTooltip />} />
                  <Legend wrapperStyle={{ fontSize: 13, paddingTop: 8 }} />
                  <Area
                    type="monotone"
                    dataKey="success_calls"
                    name="成功"
                    stroke={COLORS.success}
                    fill="url(#successGradient)"
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 2 }}
                  />
                  <Line
                    type="monotone"
                    dataKey="failed_calls"
                    name="失败"
                    stroke={COLORS.failed}
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 2 }}
                  />
                </AreaChart>
              ) : view === "latency" ? (
                <LineChart data={formattedData} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--mm-border-muted)" />
                  <XAxis
                    dataKey="timeLabel"
                    tick={{ fontSize: 12, fill: "var(--mm-text-subtle)" }}
                    tickLine={false}
                    axisLine={{ stroke: "var(--mm-border-muted)" }}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={{ fontSize: 12, fill: "var(--mm-text-subtle)" }}
                    tickLine={false}
                    axisLine={false}
                    width={55}
                    tickFormatter={(v: number) => (v >= 1000 ? `${(v / 1000).toFixed(1)}s` : `${v}ms`)}
                  />
                  <Tooltip content={<TimelineTooltip />} />
                  <Legend wrapperStyle={{ fontSize: 13, paddingTop: 8 }} />
                  <Line
                    type="monotone"
                    dataKey="avg_latency_ms"
                    name="平均延迟"
                    stroke={COLORS.latency}
                    strokeWidth={2.5}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 2 }}
                  />
                </LineChart>
              ) : (
                <AreaChart data={formattedData} margin={{ top: 8, right: 16, left: 0, bottom: 0 }}>
                  <defs>
                    <linearGradient id="tokenGradient" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={COLORS.tokens} stopOpacity={0.3} />
                      <stop offset="100%" stopColor={COLORS.tokens} stopOpacity={0.02} />
                    </linearGradient>
                  </defs>
                  <CartesianGrid strokeDasharray="3 3" stroke="var(--mm-border-muted)" />
                  <XAxis
                    dataKey="timeLabel"
                    tick={{ fontSize: 12, fill: "var(--mm-text-subtle)" }}
                    tickLine={false}
                    axisLine={{ stroke: "var(--mm-border-muted)" }}
                    interval="preserveStartEnd"
                  />
                  <YAxis
                    tick={{ fontSize: 12, fill: "var(--mm-text-subtle)" }}
                    tickLine={false}
                    axisLine={false}
                    width={60}
                    tickFormatter={(v: number) => formatNumber(v)}
                  />
                  <Tooltip content={<TimelineTooltip />} />
                  <Legend wrapperStyle={{ fontSize: 13, paddingTop: 8 }} />
                  <Area
                    type="monotone"
                    dataKey="total_tokens"
                    name="Token 消耗"
                    stroke={COLORS.tokens}
                    fill="url(#tokenGradient)"
                    strokeWidth={2}
                    dot={false}
                    activeDot={{ r: 4, strokeWidth: 2 }}
                  />
                </AreaChart>
              )}
            </ResponsiveContainer>
          </div>
        )}
      </Spin>
    </Card>
  );
}

function TimelineTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: Array<{ name: string; value: number; color: string; payload: Record<string, unknown> }>;
  label?: string;
}): JSX.Element | null {
  if (!active || !payload?.length) return null;

  const fullTime = payload[0]?.payload?.time as string | undefined;
  const fullLabel = fullTime ? new Date(fullTime).toLocaleString("zh-CN") : label ?? "";

  return (
    <div className="stats-timeline-tooltip">
      <div className="stats-timeline-tooltip-time">{fullLabel}</div>
      {payload.map((entry) => (
        <div key={entry.name} className="stats-timeline-tooltip-item">
          <span className="stats-timeline-tooltip-dot" style={{ background: entry.color }} />
          <span className="stats-timeline-tooltip-name">{entry.name}</span>
          <span className="stats-timeline-tooltip-value">
            {entry.name === "平均延迟"
              ? entry.value >= 1000
                ? `${(entry.value / 1000).toFixed(2)}s`
                : `${entry.value}ms`
              : formatNumber(entry.value)}
          </span>
        </div>
      ))}
    </div>
  );
}
