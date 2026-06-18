import { useQuery } from "@tanstack/react-query";
import { Button, Card, Drawer, Empty, Input, Result, Select, Skeleton, Space, Switch, Table, Typography } from "antd";
import { useMemo, useState } from "react";

import { fetchRecentEvents } from "../api/admin";
import { queryKeys } from "../api/query-keys";
import { formatDateTime } from "../components/format-time";
import { EventDetail } from "../features/events/event-detail";
import { buildEventColumns } from "../features/events/events-columns";
import { eventLevelOptions } from "../features/events/events-options";
import { buildEventSelectOptions, filterEvents, summarizeEvents } from "../features/events/events-utils";
import type { AdminEvent } from "../types/admin";

const eventsLimit = 200;

export function EventsPage(): JSX.Element {
  const [keyword, setKeyword] = useState("");
  const [level, setLevel] = useState("all");
  const [category, setCategory] = useState("all");
  const [provider, setProvider] = useState("all");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [selectedEvent, setSelectedEvent] = useState<AdminEvent | null>(null);

  const eventsQuery = useQuery({
    queryKey: queryKeys.events(eventsLimit),
    queryFn: () => fetchRecentEvents(eventsLimit),
    refetchInterval: autoRefresh ? 8000 : false,
  });

  const events = eventsQuery.data?.events ?? [];

  const filteredEvents = useMemo(
    () => filterEvents(events, keyword, level, category, provider),
    [events, keyword, level, category, provider],
  );

  const summary = useMemo(() => summarizeEvents(events), [events]);
  const categoryOptions = useMemo(
    () => [{ label: "全部类别", value: "all" }, ...buildEventSelectOptions(events, "category")],
    [events],
  );
  const providerOptions = useMemo(
    () => [{ label: "全部 Provider", value: "all" }, ...buildEventSelectOptions(events, "provider_id")],
    [events],
  );

  const columns = useMemo(() => buildEventColumns(), []);
  const hasFilters = keyword.trim() !== "" || level !== "all" || category !== "all" || provider !== "all";

  if (eventsQuery.isLoading) {
    return (
      <div className="console-loading">
        <Skeleton active paragraph={{ rows: 8 }} />
      </div>
    );
  }

  if (eventsQuery.isError || !eventsQuery.data) {
    return (
      <Result
        status="error"
        title="事件列表加载失败"
        subTitle={eventsQuery.error instanceof Error ? eventsQuery.error.message : "未知错误"}
        extra={<Button onClick={() => void eventsQuery.refetch()}>重试</Button>}
      />
    );
  }

  return (
    <Space direction="vertical" size={16} className="console-stack">
      <Card className="surface-card reveal-card reveal-delay-0" bordered={false}>
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">Events</Typography.Text>
            <Typography.Title level={3} className="section-title">
              最近事件
            </Typography.Title>
            <Typography.Text className="events-section-description">
              最近 {eventsLimit} 条运行事件，{autoRefresh ? "每 8 秒自动刷新" : "自动刷新已暂停"}。
            </Typography.Text>
          </div>
          <Space wrap>
            <span className="events-refresh-toggle">
              <Typography.Text>自动刷新</Typography.Text>
              <Switch size="small" checked={autoRefresh} onChange={setAutoRefresh} />
            </span>
            <Button onClick={() => void eventsQuery.refetch()}>刷新</Button>
          </Space>
        </div>

        <div className="events-summary-grid">
          <SummaryTile label="全部" value={summary.total} tone="neutral" />
          <SummaryTile label="Error" value={summary.error} tone="error" />
          <SummaryTile label="Warn" value={summary.warn} tone="warning" />
          <SummaryTile label="Info" value={summary.info} tone="info" />
          <SummaryTile label="最近错误" value={summary.lastErrorAt ? formatDateTime(summary.lastErrorAt) : "-"} tone="neutral" />
        </div>

        <div className="events-toolbar-grid">
          <Input
            allowClear
            className="events-search"
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder="搜索 message / event / category / provider / request id / model / status"
          />
          <Select className="events-level-select" value={level} options={eventLevelOptions} onChange={setLevel} />
          <Select className="events-filter-select" value={category} options={categoryOptions} onChange={setCategory} />
          <Select className="events-filter-select" value={provider} options={providerOptions} onChange={setProvider} />
          <Button disabled={!hasFilters} onClick={clearFilters}>清空筛选</Button>
        </div>

        {filteredEvents.length === 0 ? (
          <Empty description="没有匹配的事件" />
        ) : (
          <Table
            className="events-table"
            columns={columns}
            dataSource={filteredEvents}
            pagination={{ pageSize: 50, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
            scroll={{ x: 1320 }}
            size="small"
            rowKey={(event) => String(event.seq)}
            rowClassName={(event) => `events-table-row events-table-row--${event.level}`}
            onRow={(event) => ({
              onClick: () => setSelectedEvent(event),
            })}
          />
        )}
      </Card>

      <Drawer
        className="event-detail-drawer"
        title={selectedEvent ? selectedEvent.message : "事件详情"}
        open={selectedEvent !== null}
        onClose={() => setSelectedEvent(null)}
        width={Math.min(720, typeof window !== "undefined" ? window.innerWidth * 0.92 : 720)}
      >
        {selectedEvent ? <EventDetail event={selectedEvent} /> : null}
      </Drawer>
    </Space>
  );

  function clearFilters() {
    setKeyword("");
    setLevel("all");
    setCategory("all");
    setProvider("all");
  }
}

type SummaryTileProps = {
  label: string;
  value: number | string;
  tone: "neutral" | "info" | "warning" | "error";
};

function SummaryTile({ label, value, tone }: SummaryTileProps): JSX.Element {
  return (
    <div className={`events-summary-tile events-summary-tile--${tone}`}>
      <Typography.Text className="events-summary-label">{label}</Typography.Text>
      <Typography.Text className="events-summary-value">{value}</Typography.Text>
    </div>
  );
}
