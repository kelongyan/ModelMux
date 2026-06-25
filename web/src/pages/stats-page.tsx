import { useQuery } from "@tanstack/react-query";
import { Button, DatePicker, Result, Space, Spin } from "antd";
import type { Dayjs } from "dayjs";
import { useMemo, useState } from "react";

import { fetchStatsLogs, fetchStatsModels, fetchStatsSummary } from "../api/admin";
import { queryKeys } from "../api/query-keys";
import { useVisibilityRefetchInterval } from "../components/use-visibility-polling";
import { StatsLogsCard } from "../features/stats/stats-logs-card";
import { StatsSummaryCard } from "../features/stats/stats-summary-card";
import { StatsTimelineCard } from "../features/stats/stats-timeline-card";
import type { AdminStatsSummary, AdminStatsWindow } from "../types/admin";

const { RangePicker } = DatePicker;

export function StatsPage(): JSX.Element {
  const [window, setWindow] = useState<AdminStatsWindow>("24h");
  const [dateRange, setDateRange] = useState<[Dayjs | null, Dayjs | null]>([null, null]);
  const [model, setModel] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const pollInterval = useVisibilityRefetchInterval(10_000);

  // 计算自定义时间范围参数
  const customRangeParams = useMemo(() => {
    if (dateRange[0] && dateRange[1]) {
      return {
        from: dateRange[0].toISOString(),
        to: dateRange[1].toISOString(),
      };
    }
    return undefined;
  }, [dateRange]);

  const summaryQuery = useQuery({
    queryKey: customRangeParams
      ? queryKeys.statsSummary("custom" as AdminStatsWindow)
      : queryKeys.statsSummary(window),
    queryFn: () => fetchStatsSummary(window, customRangeParams),
    refetchInterval: pollInterval,
  });

  const modelsQuery = useQuery({
    queryKey: customRangeParams
      ? queryKeys.statsModels("custom" as AdminStatsWindow)
      : queryKeys.statsModels(window),
    queryFn: () => fetchStatsModels(window, customRangeParams),
    refetchInterval: pollInterval,
  });

  const logsQuery = useQuery({
    queryKey: customRangeParams
      ? queryKeys.statsLogs({ window: "custom" as AdminStatsWindow, model, status, page, pageSize })
      : queryKeys.statsLogs({ window, model, status, page, pageSize }),
    queryFn: () =>
      fetchStatsLogs({
        window,
        model: model || undefined,
        status: status || undefined,
        page,
        page_size: pageSize,
        ...customRangeParams,
      }),
    refetchInterval: pollInterval,
  });

  const modelOptions = useMemo(() => {
    const options = [{ label: "全部模型", value: "" }];
    for (const item of modelsQuery.data?.models ?? []) {
      if (item.model) {
        options.push({ label: item.model, value: item.model });
      }
    }
    return options;
  }, [modelsQuery.data]);

  const summary = summaryQuery.data?.summary ?? emptySummary();
  const droppedRecords = summaryQuery.data?.dropped_records ?? 0;
  const queueDepth = summaryQuery.data?.queue_depth ?? 0;
  const queueCapacity = summaryQuery.data?.queue_capacity ?? 0;
  const logs = logsQuery.data?.records ?? [];
  const total = logsQuery.data?.total ?? 0;

  return (
    <Space direction="vertical" size={20} className="console-stack">
      {summaryQuery.isError ? (
        <Result
          status="error"
          title="统计摘要加载失败"
          subTitle={summaryQuery.error instanceof Error ? summaryQuery.error.message : "未知错误"}
          extra={<Button onClick={() => void summaryQuery.refetch()}>重试</Button>}
        />
      ) : (
        <>
          <StatsSummaryCard
            summary={summary}
            droppedRecords={droppedRecords}
            queueDepth={queueDepth}
            queueCapacity={queueCapacity}
            window={window}
            loading={summaryQuery.isLoading}
            onWindowChange={(nextWindow) => {
              setWindow(nextWindow);
              setDateRange([null, null]);
              setPage(1);
            }}
            onRefresh={() => void refetchAll()}
          />

          <StatsTimelineCard window={window} dateRange={dateRange} />

          <Space wrap size={12}>
            <RangePicker
              showTime
              value={dateRange}
              onChange={(dates) => {
                setDateRange(dates as [Dayjs | null, Dayjs | null]);
                setPage(1);
              }}
              placeholder={["开始时间", "结束时间"]}
              style={{ width: 380 }}
            />
            {dateRange[0] && dateRange[1] && (
              <Button onClick={() => setDateRange([null, null])}>清除自定义范围</Button>
            )}
          </Space>
        </>
      )}

      {logsQuery.isError ? (
        <Result
          status="error"
          title="调用日志加载失败"
          subTitle={logsQuery.error instanceof Error ? logsQuery.error.message : "未知错误"}
          extra={<Button onClick={() => void logsQuery.refetch()}>重试</Button>}
        />
      ) : (
        <StatsLogsCard
          logs={logs}
          total={total}
          loading={logsQuery.isLoading || logsQuery.isFetching}
          model={model}
          modelOptions={modelOptions}
          status={status}
          page={page}
          pageSize={pageSize}
          onModelChange={(nextModel) => {
            setModel(nextModel);
            setPage(1);
          }}
          onStatusChange={(nextStatus) => {
            setStatus(nextStatus);
            setPage(1);
          }}
          onPageChange={(nextPage, nextPageSize) => {
            setPage(nextPage);
            setPageSize(nextPageSize);
          }}
        />
      )}
    </Space>
  );

  async function refetchAll() {
    await Promise.all([summaryQuery.refetch(), modelsQuery.refetch(), logsQuery.refetch()]);
  }
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
    p50_latency_ms: 0,
    p95_latency_ms: 0,
    p99_latency_ms: 0,
  };
}
