import type { TableColumnsType } from "antd";

import { formatDateTime } from "../../components/format-time";
import type { AdminEvent } from "../../types/admin";

export function buildEventColumns(): TableColumnsType<AdminEvent> {
  return [
    {
      title: "时间",
      dataIndex: "at",
      key: "at",
      width: 162,
      render: (value: string) => <span className="events-table-time">{formatDateTime(value)}</span>,
    },
    {
      title: "级别",
      dataIndex: "level",
      key: "level",
      width: 88,
      render: (level: string) => <span className={`events-rail-level events-rail-level--${level}`}>{level}</span>,
    },
    {
      title: "事件",
      dataIndex: "event",
      key: "event",
      width: 160,
      ellipsis: { showTitle: true },
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "类别",
      dataIndex: "category",
      key: "category",
      width: 116,
      ellipsis: { showTitle: true },
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "Provider",
      dataIndex: "provider_id",
      key: "provider_id",
      width: 110,
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "模型",
      dataIndex: "model",
      key: "model",
      width: 170,
      ellipsis: { showTitle: true },
      render: (value: string | undefined) => value ?? "-",
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      width: 82,
      render: (value: number | undefined) => (value ? <span className={`events-status events-status--${statusTone(value)}`}>{value}</span> : "-"),
    },
    {
      title: "耗时",
      dataIndex: "latency_ms",
      key: "latency_ms",
      width: 88,
      render: (value: number | undefined) => (typeof value === "number" ? `${value}ms` : "-"),
    },
    {
      title: "重试",
      dataIndex: "attempts",
      key: "attempts",
      width: 76,
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
