import { useQuery } from "@tanstack/react-query";
import { Button, Card, Empty, Input, Result, Row, Col, Select, Space, Spin, Tag, Typography } from "antd";
import { useMemo, useState } from "react";

import { fetchRecentEvents } from "../api/admin";
import type { AdminEvent } from "../types/admin";

const eventLevelOptions = [
  { label: "全部级别", value: "all" },
  { label: "info", value: "info" },
  { label: "warn", value: "warn" },
  { label: "error", value: "error" },
];

// EventsPage 展示最近事件，并支持按级别和关键词进行本地过滤。
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
      return [event.message, event.category, event.event, JSON.stringify(event.data ?? {})]
        .join(" ")
        .toLowerCase()
        .includes(query);
    });
  }, [eventsQuery.data, keyword, level]);

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
    <Space direction="vertical" size={20} className="console-stack">
      <Row gutter={[18, 18]}>
        <Col xs={24} xl={8}>
          <Card className="surface-card sticky-card" bordered={false}>
            <div className="section-heading">
              <div>
                <Typography.Text className="placeholder-kicker">筛选器</Typography.Text>
                <Typography.Title level={3} className="section-title">
                  事件过滤
                </Typography.Title>
              </div>
              <Button onClick={() => void eventsQuery.refetch()}>刷新</Button>
            </div>

            <Space direction="vertical" size={14} className="console-stack">
              <Input
                allowClear
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder="按 message / category / event 搜索"
              />
              <Select
                value={level}
                options={eventLevelOptions}
                onChange={setLevel}
              />
              <Tag color="cyan">{`总数 ${eventsQuery.data.events.length}`}</Tag>
            </Space>
          </Card>
        </Col>

        <Col xs={24} xl={16}>
          <Card className="surface-card" bordered={false}>
            <div className="section-heading">
              <div>
                <Typography.Text className="placeholder-kicker">Recent Events</Typography.Text>
                <Typography.Title level={3} className="section-title">
                  最近事件
                </Typography.Title>
              </div>
            </div>

            {filteredEvents.length === 0 ? (
              <Empty description="没有匹配的事件" />
            ) : (
              <div className="event-card-list">
                {filteredEvents.map((event) => (
                  <EventCard key={`${event.seq}-${event.event}`} event={event} />
                ))}
              </div>
            )}
          </Card>
        </Col>
      </Row>
    </Space>
  );
}

type EventCardProps = {
  event: AdminEvent;
};

// EventCard 渲染单条事件，突出 message，并保留分类和附加数据上下文。
function EventCard({ event }: EventCardProps): JSX.Element {
  const levelColor = event.level === "error" ? "red" : event.level === "warn" ? "gold" : "blue";

  return (
    <Card className="surface-card event-card" bordered={false}>
      <div className="event-card-head">
        <Space wrap>
          <Tag color={levelColor}>{event.level}</Tag>
          <Tag>{event.category}</Tag>
          <Tag>{event.event}</Tag>
        </Space>
        <Typography.Text className="table-subtext">{formatDateTime(event.at)}</Typography.Text>
      </div>
      <Typography.Title level={5} className="event-card-title">
        {event.message}
      </Typography.Title>
      {event.data ? (
        <pre className="event-card-payload">{JSON.stringify(event.data, null, 2)}</pre>
      ) : null}
    </Card>
  );
}

// formatDateTime 把事件时间格式化为本地可读文本。
function formatDateTime(value: string): string {
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}
