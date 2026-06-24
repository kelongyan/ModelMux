import { memo, useMemo } from "react";
import { Empty, Popconfirm, Table } from "antd";
import type { TableColumnsType } from "antd";

import type { AdminProviderSummary } from "../../types/admin";
import { renderProviderState } from "./provider-utils";

type ProviderTableProps = {
  providers: AdminProviderSummary[];
  activating: boolean;
  deleting: boolean;
  onOpenDetail: (providerID: string) => void;
  onActivate: (providerID: string) => void;
  onEdit: (provider: AdminProviderSummary) => void;
  onDelete: (providerID: string) => void;
};

export const ProviderTable = memo(function ProviderTable({
  providers,
  activating,
  deleting,
  onOpenDetail,
  onActivate,
  onEdit,
  onDelete,
}: ProviderTableProps): JSX.Element {
  if (providers.length === 0) {
    return <Empty description="当前没有 provider 配置" />;
  }

  const columns: TableColumnsType<AdminProviderSummary> = useMemo(() => [
    {
      title: "Provider",
      dataIndex: "id",
      key: "id",
      render: (_: string, record) => <div className="provider-table-id">{record.id}</div>,
    },
    {
      title: "状态",
      dataIndex: "active",
      key: "active",
      render: (_active: boolean, record) => renderProviderState(record),
    },
    {
      title: "总 Key",
      key: "configured_keys",
      render: (_: unknown, record) => record.total_keys + record.disabled_keys,
    },
    { title: "停用", dataIndex: "disabled_keys", key: "disabled_keys" },
    { title: "余额不足", dataIndex: "quota_exhausted_keys", key: "quota_exhausted_keys" },
    { title: "可用", dataIndex: "active_keys", key: "active_keys" },
    { title: "冷却", dataIndex: "cooling_keys", key: "cooling_keys" },
    { title: "失效", dataIndex: "invalid_keys", key: "invalid_keys" },
    {
      title: "操作",
      key: "actions",
      render: (_: unknown, record) => (
        <div className="provider-actions">
          <button className="provider-action provider-action--primary" onClick={() => onOpenDetail(record.id)}>
            详情
          </button>
          {!record.active ? (
            <button
              className="provider-action provider-action--activate"
              disabled={activating}
              onClick={() => onActivate(record.id)}
            >
              激活
            </button>
          ) : null}
          <button className="provider-action provider-action--edit" onClick={() => onEdit(record)}>
            编辑
          </button>
          <Popconfirm
            title={`确认删除 provider ${record.id}？`}
            description="删除后将同时移除其全部 keys。"
            okText="删除"
            cancelText="取消"
            onConfirm={() => onDelete(record.id)}
          >
            <button className="provider-action provider-action--danger" disabled={deleting}>
              删除
            </button>
          </Popconfirm>
        </div>
      ),
    },
  ], [activating, deleting, onOpenDetail, onActivate, onEdit, onDelete]);

  return (
    <Table
      className="provider-table"
      columns={columns}
      dataSource={providers}
      pagination={false}
      rowKey="id"
      rowClassName={(record) => (record.active ? "provider-table-row--active" : "")}
      scroll={{ x: 1020 }}
    />
  );
});
