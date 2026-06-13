import { Tag } from "antd";
import type { ColumnsType } from "antd/es/table";

import type { AdminCallRecord } from "../../types/admin";
import { formatLocalDateTime, formatNumber, formatLatencySec, latencyClass } from "./stats-format";

export function buildStatsLogColumns(): ColumnsType<AdminCallRecord> {
  return [
    {
      title: "时间",
      dataIndex: "at",
      key: "at",
      width: 165,
      render: (value: string) => formatLocalDateTime(value),
    },
    {
      title: "模型",
      dataIndex: "model",
      key: "model",
      width: 180,
      ellipsis: true,
      render: (value: string) => value ? <span className="stats-model-name">{value}</span> : <span className="stats-text-muted">-</span>,
    },
    {
      title: "提供商",
      dataIndex: "provider_id",
      key: "provider_id",
      width: 110,
      ellipsis: true,
    },
    {
      title: "状态",
      key: "success",
      width: 110,
      render: (_: unknown, record) =>
        record.success
          ? <Tag color="success">成功</Tag>
          : <Tag color="error">失败 {record.status || ""}</Tag>,
    },
    {
      title: "延迟",
      dataIndex: "latency_ms",
      key: "latency_ms",
      width: 95,
      sorter: (a, b) => a.latency_ms - b.latency_ms,
      render: (value: number) => <span className={latencyClass(value)}>{formatLatencySec(value)}</span>,
    },
    {
      title: "总 Token",
      dataIndex: "total_tokens",
      key: "total_tokens",
      width: 95,
      render: (value?: number) => value != null ? formatNumber(value) : "-",
    },
    {
      title: "错误",
      dataIndex: "error",
      key: "error",
      width: 200,
      ellipsis: true,
      render: (value?: string) => value
        ? <span className="stats-text-error" title={value}>{value}</span>
        : <span className="stats-text-muted">-</span>,
    },
  ];
}
