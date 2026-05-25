import { useQuery } from "@tanstack/react-query";
import { Button, Card, Col, Empty, Input, Result, Row, Select, Space, Spin, Table, Tag, Typography } from "antd";
import type { TableColumnsType } from "antd";
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

  const columns = useMemo<TableColumnsType<AdminEvent>>(
    () => [
      {
        title: "时间",
        dataIndex: "at",
        key: "at",
        width: 170,
        render: (value: string) => <span className="events-table-time">{formatDateTime(value)}</span>,
      },
      {
        title: "级别",
        dataIndex: "level",
        key: "level",
        width: 86,
        render: (lv: string) => <Tag color={levelColor(lv)}>{lv}</Tag>,
      },
      {
        title: "分类",
        dataIndex: "category",
        key: "category",
        width: 110,
      },
      {
        title: "事件",
        dataIndex: "event",
        key: "event",
        width: 160,
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
    <Space direction="vertical" size={20} className="console-stack">
      <Row gutter={[18, 18]}>
        <Col xs={24} xl={7}>
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
              <Select value={level} options={eventLevelOptions} onChange={setLevel} />
              <div className="events-summary-row">
                <Tag color="cyan">{`总数 ${eventsQuery.data.events.length}`}</Tag>
                <Tag>{`已筛选 ${filteredEvents.length}`}</Tag>
              </div>
            </Space>
          </Card>
        </Col>

        <Col xs={24} xl={17}>
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
              <Table
                className="events-table"
                columns={columns}
                dataSource={filteredEvents}
                pagination={{ pageSize: 50, showSizeChanger: true, showTotal: (total) => `共 ${total} 条` }}
                size="small"
                rowKey={(event) => `${event.seq}-${event.event}`}
                rowClassName={(event) => `events-table-row events-table-row--${event.level}`}
                expandable={{
                  expandedRowRender: (event) =>
                    event.data ? (
                      <pre className="events-table-payload">{JSON.stringify(event.data, null, 2)}</pre>
                    ) : (
                      <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="无附加数据" />
                    ),
                  rowExpandable: (event) => Boolean(event.data),
                }}
              />
            )}
          </Card>
        </Col>
      </Row>
    </Space>
  );
}

// levelColor 把事件 level 映射成 Antd Tag 配色。
function levelColor(level: string): string {
  if (level === "error") {
    return "red";
  }
  if (level === "warn") {
    return "gold";
  }
  return "blue";
}

// formatDateTime 把事件时间格式化为本地可读文本。
function formatDateTime(value: string): string {
  return new Date(value).toLocaleString("zh-CN", { hour12: false });
}
