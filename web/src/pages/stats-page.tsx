import { useQuery } from "@tanstack/react-query";
import { Button, Result, Space, Spin } from "antd";
import { useMemo, useState } from "react";

import { fetchStatsLogs, fetchStatsModels, fetchStatsSummary } from "../api/admin";
import { queryKeys } from "../api/query-keys";
import { StatsLogsCard } from "../features/stats/stats-logs-card";
import { StatsSummaryCard } from "../features/stats/stats-summary-card";
import type { AdminStatsSummary, AdminStatsWindow } from "../types/admin";

export function StatsPage(): JSX.Element {
  const [window, setWindow] = useState<AdminStatsWindow>("24h");
  const [model, setModel] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  const summaryQuery = useQuery({
    queryKey: queryKeys.statsSummary(window),
    queryFn: () => fetchStatsSummary(window),
    refetchInterval: 10_000,
  });

  const modelsQuery = useQuery({
    queryKey: queryKeys.statsModels(window),
    queryFn: () => fetchStatsModels(window),
    refetchInterval: 10_000,
  });

  const logsQuery = useQuery({
    queryKey: queryKeys.statsLogs({ window, model, status, page, pageSize }),
    queryFn: () =>
      fetchStatsLogs({ window, model: model || undefined, status: status || undefined, page, page_size: pageSize }),
    refetchInterval: 10_000,
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

  if (summaryQuery.isLoading || logsQuery.isLoading || modelsQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (summaryQuery.isError || logsQuery.isError || modelsQuery.isError) {
    const error = summaryQuery.error ?? logsQuery.error ?? modelsQuery.error;
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
  const logs = logsQuery.data?.records ?? [];
  const total = logsQuery.data?.total ?? 0;

  return (
    <Space direction="vertical" size={20} className="console-stack">
      <StatsSummaryCard
        summary={summary}
        window={window}
        onWindowChange={(nextWindow) => {
          setWindow(nextWindow);
          setPage(1);
        }}
        onRefresh={() => void refetchAll()}
      />

      <StatsLogsCard
        logs={logs}
        total={total}
        loading={logsQuery.isFetching}
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
  };
}
