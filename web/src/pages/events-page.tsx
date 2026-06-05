import { useQuery } from "@tanstack/react-query";
import { Button, Card, Empty, Input, Result, Select, Space, Spin, Table, Typography } from "antd";
import type { TableColumnsType } from "antd";
import { useMemo, useState } from "react";

import { fetchRecentEvents } from "../api/admin";
import { formatDateTime } from "../components/format-time";
import type { AdminEvent } from "../types/admin";

const eventLevelOptions = [
  { label: "全部级别", value: "all" },
  { label: "info", value: "info" },
  { label: "warn", value: "warn" },
  { label: "error", value: "error" },
];

export function EventsPage(): JSX.Element {
  const [keyword, setKeyword] = useState("");
  const [level, setLevel] = useState("all");

  const eventsQuery = useQuery({
    queryKey: ["events", 200],
    queryFn: () => fetchRecentEvents(200),
    refetchInterval: 8000,
  });

  const filteredEvents = useMemo(() => {
    if (!eventsQuery.data) {
      return [];
    }
    return eventsQuery.data.events.filter((event) => {
      if (level !== "all" && event.level !== level) {
        return false;
      }
      if (!keyword.trim()) {
        return true;
      }
      const query = keyword.trim().toLowerCase();
      return [
        event.message,
        event.category,
        event.event,
        event.provider_id ?? "",
        event.request_id ?? "",
        event.model ?? "",
        String(event.status ?? ""),
        JSON.stringify(event.data ?? {}),
      ]
        .join(" ")
        .toLowerCase()
        .includes(query);
    });
  }, [eventsQuery.data, keyword, level]);

  const columns = useMemo<TableColumnsType<AdminEvent>>(
    () => [
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
        render: (lv: string) => <span className={`events-rail-level events-rail-level--${lv}`}>{lv}</span>,
      },
      {
        title: "Provider",
        dataIndex: "provider_id",
        key: "provider_id",
        width: 120,
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
        width: 92,
        render: (value: number | undefined) => (value ? String(value) : "-"),
      },
      {
        title: "消息",
        dataIndex: "message",
        key: "message",
        ellipsis: { showTitle: true },
      },
    ],
    [],
  );

  if (eventsQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (eventsQuery.isError || !eventsQuery.data) {
    return (
      <Result
        status="error"
        title="事件列表加载失败"
        subTitle={eventsQuery.error instanceof Error ? eventsQuery.error.message : "未知错误"}
      />
    );
  }

  return (
    <Space direction="vertical" size={16} className="console-stack">
      <Card className="surface-card" bordered={false}>
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">Events</Typography.Text>
            <Typography.Title level={3} className="section-title">
              最近事件
            </Typography.Title>
          </div>
          <Button onClick={() => void eventsQuery.refetch()}>刷新</Button>
        </div>

        <div className="events-toolbar-grid">
          <Input
            allowClear
            className="events-search"
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索 message / provider / request id / model"
          />
          <Select className="events-level-select" value={level} options={eventLevelOptions} onChange={setLevel} />
        </div>

        {filteredEvents.length === 0 ? (
          <Empty description="没有匹配的事件" />
        ) : (
          <Table
            className="events-table"
            columns={columns}
            dataSource={filteredEvents}
            pagination={{ pageSize: 50, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
            scroll={{ x: 980 }}
            size="small"
            rowKey={(event) => `${event.seq}-${event.event}`}
            rowClassName={(event) => `events-table-row events-table-row--${event.level}`}
            expandable={{
              expandedRowRender: (event) => (
                <pre className="events-table-payload">{JSON.stringify(buildExpandedPayload(event), null, 2)}</pre>
              ),
              rowExpandable: () => true,
            }}
          />
        )}
      </Card>
    </Space>
  );
}

function buildExpandedPayload(event: AdminEvent): Record<string, unknown> {
  return {
    category: event.category,
    event: event.event,
    request_id: event.request_id,
    client_request_id: event.client_request_id,
    provider_id: event.provider_id,
    key_id: event.key_id,
    key_hint: event.key_hint,
    method: event.method,
    path: event.path,
    model: event.model,
    stream: event.stream,
    status: event.status,
    latency_ms: event.latency_ms,
    attempts: event.attempts,
    retry_scope: event.retry_scope,
    ...(event.data ?? {}),
  };
}
