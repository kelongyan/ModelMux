import { Card, Select, Space, Table, Typography } from "antd";

import type { AdminCallRecord } from "../../types/admin";
import { buildStatsLogColumns } from "./stats-columns";
import { statsStatusOptions } from "./stats-options";

type StatsLogsCardProps = {
  logs: AdminCallRecord[];
  total: number;
  loading: boolean;
  model: string;
  modelOptions: Array<{ label: string; value: string }>;
  status: string;
  page: number;
  pageSize: number;
  onModelChange: (model: string) => void;
  onStatusChange: (status: string) => void;
  onPageChange: (page: number, pageSize: number) => void;
};

export function StatsLogsCard({
  logs,
  total,
  loading,
  model,
  modelOptions,
  status,
  page,
  pageSize,
  onModelChange,
  onStatusChange,
  onPageChange,
}: StatsLogsCardProps): JSX.Element {
  return (
    <Card className="surface-card reveal-card reveal-delay-1" bordered={false}>
      <div className="section-heading">
        <Typography.Title level={4} className="section-title">调用日志</Typography.Title>
        <Space wrap size={12}>
          <Select
            value={model}
            options={modelOptions}
            onChange={onModelChange}
            placeholder="全部模型"
            allowClear
            style={{ minWidth: 160 }}
          />
          <Select
            value={status}
            options={statsStatusOptions}
            onChange={onStatusChange}
            style={{ minWidth: 120 }}
          />
        </Space>
      </div>

      <Table<AdminCallRecord>
        rowKey="id"
        columns={buildStatsLogColumns()}
        dataSource={logs}
        size="middle"
        scroll={{ x: 1080 }}
        loading={loading}
        pagination={{
          current: page,
          pageSize,
          total,
          showSizeChanger: true,
          pageSizeOptions: ["10", "20", "50"],
          showTotal: (count) => `共 ${count} 条记录`,
          onChange: onPageChange,
        }}
      />
    </Card>
  );
}
