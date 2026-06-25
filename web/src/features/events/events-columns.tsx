import type { TableColumnsType } from "antd";

import { formatDateTime } from "../../components/format-time";
import type { AdminEvent } from "../../types/admin";
import { formatLatencySec } from "../stats/stats-format";

export function buildEventColumns(): TableColumnsType<AdminEvent> {
  return [
    {
      title: "时间",
      dataIndex: "at",
      key: "at",
      width: 162,
      sorter: (a, b) => new Date(a.at).getTime() - new Date(b.at).getTime(),
      render: (value: string) => <span className="events-table-time">{formatDateTime(value)}</span>,
    },
    {
      title: "级别",
      dataIndex: "level",
      key: "level",
      width: 88,
      sorter: (a, b) => (a.level ?? "").localeCompare(b.level ?? ""),
      render: (level: string) => <span className={`events-rail-level events-rail-level--${level}`}>{level}</span>,
    },
    {
      title: "事件",
      dataIndex: "event",
      key: "event",
      width: 160,
      ellipsis: { showTitle: true },
      sorter: (a, b) => (a.event ?? "").localeCompare(b.event ?? ""),
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "类别",
      dataIndex: "category",
      key: "category",
      width: 116,
      ellipsis: { showTitle: true },
      sorter: (a, b) => (a.category ?? "").localeCompare(b.category ?? ""),
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "Provider",
      dataIndex: "provider_id",
      key: "provider_id",
      width: 110,
      sorter: (a, b) => (a.provider_id ?? "").localeCompare(b.provider_id ?? ""),
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "模型",
      dataIndex: "model",
      key: "model",
      width: 170,
      ellipsis: { showTitle: true },
      sorter: (a, b) => (a.model ?? "").localeCompare(b.model ?? ""),
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      width: 82,
      sorter: (a, b) => (a.status ?? 0) - (b.status ?? 0),
      render: (value: number | undefined) => (value ? <span className={`events-status events-status--${statusTone(value)}`}>{value}</span> : "-"),
    },
    {
      title: "耗时",
      dataIndex: "latency_ms",
      key: "latency_ms",
      width: 88,
      sorter: (a, b) => (a.latency_ms ?? 0) - (b.latency_ms ?? 0),
      render: (value: number | undefined) => (typeof value === "number" ? formatLatencySec(value) : "-"),
    },
    {
      title: "重试",
      dataIndex: "attempts",
      key: "attempts",
      width: 76,
      sorter: (a, b) => (a.attempts ?? 0) - (b.attempts ?? 0),
      render: (value: number | undefined) => (typeof value === "number" ? value : "-"),
    },
    {
      title: "消息",
      dataIndex: "message",
      key: "message",
      ellipsis: { showTitle: true },
    },
  ];
}

function statusTone(status: number): "success" | "warning" | "error" {
  if (status >= 500) return "error";
  if (status >= 400) return "warning";
  return "success";
}
