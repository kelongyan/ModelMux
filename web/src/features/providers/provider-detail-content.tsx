import { Button, Card, Empty, Popconfirm, Space, Table, Tag, Typography } from "antd";
import type { TableColumnsType } from "antd";

import type { AdminKeyStatus, AdminProviderDetailResponse } from "../../types/admin";
import { renderKeyState } from "./provider-utils";

type ProviderDetailContentProps = {
  detail: AdminProviderDetailResponse;
  selectedKeyIDs: string[];
  onSelectedKeyIDsChange: (keyIDs: string[]) => void;
  onResetKey: (keyID: string) => void;
  resettingKey: boolean;
  onOpenAppendKeys: () => void;
  onOpenReplaceKeys: () => void;
  onDeleteSelectedKeys: () => void;
  deletingKeys: boolean;
  onOpenEditModels: () => void;
  onFetchModels: () => void;
  fetchingModels: boolean;
};

export function ProviderDetailContent({
  detail,
  selectedKeyIDs,
  onSelectedKeyIDsChange,
  onResetKey,
  resettingKey,
  onOpenAppendKeys,
  onOpenReplaceKeys,
  onDeleteSelectedKeys,
  deletingKeys,
  onOpenEditModels,
  onFetchModels,
  fetchingModels,
}: ProviderDetailContentProps): JSX.Element {
  const models = detail.models ?? [];
  const keyColumns: TableColumnsType<AdminKeyStatus> = [
    {
      title: "Key 标识",
      dataIndex: "masked_key",
      key: "masked_key",
      width: 290,
      render: (maskedKey: string, record) => (
        <div className="provider-key-cell">
          <strong>{maskedKey}</strong>
          <div className="table-subtext provider-key-id">{record.key_id}</div>
        </div>
      ),
    },
    {
      title: "状态",
      dataIndex: "state",
      key: "state",
      width: 92,
      render: (state: AdminKeyStatus["state"]) => renderKeyState(state),
    },
    {
      title: "失效原因",
      dataIndex: "invalid_reason",
      key: "invalid_reason",
      width: 116,
      render: (reason: string | undefined, record) => renderInvalidReason(reason, record.state),
    },
    { title: "请求数", dataIndex: "req_count", key: "req_count", width: 86 },
    { title: "错误数", dataIndex: "err_count", key: "err_count", width: 86 },
    {
      title: "操作",
      key: "actions",
      width: 104,
      render: (_: unknown, record) => (
        <Button size="small" loading={resettingKey} onClick={() => onResetKey(record.key_id)}>
          重置状态
        </Button>
      ),
    },
  ];

  return (
    <Space direction="vertical" size={14} className="console-stack">
      <Card className="surface-card reveal-card reveal-delay-0" bordered={false}>
        <div className="detail-stats-row">
          <div className="detail-stat">
            <span>总 Key</span>
            <strong>{detail.total_keys}</strong>
          </div>
          <div className="detail-stat">
            <span>可用</span>
            <strong>{detail.active_keys}</strong>
          </div>
          <div className="detail-stat">
            <span>冷却</span>
            <strong>{detail.cooling_keys}</strong>
          </div>
          <div className="detail-stat">
            <span>失效</span>
            <strong>{detail.invalid_keys}</strong>
          </div>
        </div>
        <div className="detail-target-url">
          <span className="detail-target-label">Target URL</span>
          <a href={detail.target_url} target="_blank" rel="noreferrer" className="detail-target-link">{detail.target_url}</a>
        </div>
      </Card>

      <Card className="surface-card reveal-card reveal-delay-1" bordered={false}>
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">模型记录</Typography.Text>
            <Typography.Title level={4} className="section-title">
              已记录模型 {models.length > 0 ? `(${models.length})` : ""}
            </Typography.Title>
          </div>
          <Space wrap>
            <Button size="small" onClick={onOpenEditModels}>
              编辑
            </Button>
            <Button size="small" loading={fetchingModels} onClick={onFetchModels}>
              从上游拉取
            </Button>
          </Space>
        </div>
        {models.length === 0 ? (
          <Empty description="暂无模型记录，点击「编辑」手动添加或「从上游拉取」自动获取" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          <div className="model-tags">
            {models.map((model) => (
              <Tag key={model} className="model-tag">
                {model}
              </Tag>
            ))}
          </div>
        )}
      </Card>

      <Card className="surface-card reveal-card reveal-delay-2" bordered={false}>
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">Key 管理</Typography.Text>
            <Typography.Title level={4} className="section-title">
              当前 Keys
            </Typography.Title>
          </div>
          <Space wrap>
            <Button size="small" onClick={onOpenAppendKeys}>追加 Keys</Button>
            <Button size="small" danger onClick={onOpenReplaceKeys}>
              替换全部
            </Button>
            <Popconfirm
              title={`确认删除选中的 ${selectedKeyIDs.length} 个 key？`}
              description="至少需要保留一个 key。"
              okText="删除"
              cancelText="取消"
              onConfirm={onDeleteSelectedKeys}
              disabled={selectedKeyIDs.length === 0}
            >
              <Button size="small" danger disabled={selectedKeyIDs.length === 0} loading={deletingKeys}>
                删除选中
              </Button>
            </Popconfirm>
          </Space>
        </div>
        <Table
          className="provider-table"
          columns={keyColumns}
          dataSource={detail.keys}
          pagination={false}
          rowKey="key_id"
          rowSelection={{
            selectedRowKeys: selectedKeyIDs,
            onChange: (rowKeys) => onSelectedKeyIDsChange(rowKeys.map(String)),
          }}
          scroll={{ x: 820 }}
        />
      </Card>
    </Space>
  );
}

function renderInvalidReason(reason: string | undefined, state: AdminKeyStatus["state"]): JSX.Element | string {
  if (state !== "invalid") {
    return "-";
  }
  switch (reason) {
    case "quota_exhausted":
      return <Tag color="red">余额不足</Tag>;
    case "unauthorized":
      return <Tag color="volcano">认证失败</Tag>;
    default:
      return reason ? <Tag color="default">{reason}</Tag> : <Tag color="default">未知</Tag>;
  }
}
