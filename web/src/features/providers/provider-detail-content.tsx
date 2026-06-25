import { CopyOutlined } from "@ant-design/icons";
import { Button, Card, Empty, Popconfirm, Space, Table, Tag, Typography, message } from "antd";
import type { TableColumnsType } from "antd";

import { CooldownText } from "../../components/cooldown-text";
import { formatDateTime } from "../../components/format-time";
import type { AdminKeyStatus, AdminProviderDetailResponse } from "../../types/admin";
import { formatLatencySec } from "../stats/stats-format";
import { isQuotaExhaustedKey } from "./provider-key-status";
import { renderKeyState } from "./provider-utils";

type ProviderDetailContentProps = {
  detail: AdminProviderDetailResponse;
  selectedKeyIDs: string[];
  onSelectedKeyIDsChange: (keyIDs: string[]) => void;
  onResetKey: (keyID: string) => void;
  resettingKey: boolean;
  onResetAllKeys: () => void;
  resettingAllKeys: boolean;
  onEditKeyMetadata: (key: AdminKeyStatus) => void;
  onToggleKeyDisabled: (key: AdminKeyStatus, disabled: boolean) => void;
  updatingKeyMetadata: boolean;
  onTestKey: (keyID: string) => void;
  testingKeyID: string | null;
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
  onResetAllKeys,
  resettingAllKeys,
  onEditKeyMetadata,
  onToggleKeyDisabled,
  updatingKeyMetadata,
  onTestKey,
  testingKeyID,
  onOpenAppendKeys,
  onOpenReplaceKeys,
  onDeleteSelectedKeys,
  deletingKeys,
  onOpenEditModels,
  onFetchModels,
  fetchingModels,
}: ProviderDetailContentProps): JSX.Element {
  const [messageApi, contextHolder] = message.useMessage();
  const models = detail.models ?? [];
  const configuredKeys = detail.total_keys + detail.disabled_keys;

  function copyTargetUrl(): void {
    navigator.clipboard
      .writeText(detail.target_url)
      .then(() => messageApi.success("已复制 Target URL"))
      .catch(() => messageApi.error("复制失败"));
  }

  const keyColumns: TableColumnsType<AdminKeyStatus> = [
    {
      title: "Key 标识",
      dataIndex: "masked_key",
      key: "masked_key",
      width: 280,
      render: (maskedKey: string, record) => (
        <div className={isQuotaExhaustedKey(record) ? "provider-key-cell provider-key-cell--quota" : "provider-key-cell"}>
          <strong>
            {isQuotaExhaustedKey(record) ? <span className="provider-key-quota-dot" title="余额不足" aria-label="余额不足" /> : null}
            {maskedKey}
          </strong>
          <div className="table-subtext provider-key-id">{record.key_id}</div>
        </div>
      ),
    },
    {
      title: "标签",
      dataIndex: "label",
      key: "label",
      width: 120,
      render: (label: string | undefined) => label || "-",
    },
    {
      title: "备注",
      dataIndex: "note",
      key: "note",
      width: 180,
      ellipsis: true,
      render: (note: string | undefined) => note || "-",
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
    {
      title: "冷却",
      dataIndex: "cool_until",
      key: "cool_until",
      width: 96,
      render: (coolUntil: string | undefined, record) =>
        record.state === "cooling" ? <CooldownText until={coolUntil} /> : "-",
    },
    {
      title: "最近 401",
      dataIndex: "last_401_at",
      key: "last_401_at",
      width: 150,
      render: (value: string | undefined) => renderDateTime(value),
    },
    { title: "请求数", dataIndex: "req_count", key: "req_count", width: 86 },
    { title: "错误数", dataIndex: "err_count", key: "err_count", width: 86 },
    {
      title: "平均延迟",
      dataIndex: "avg_latency_ms",
      key: "avg_latency_ms",
      width: 102,
      render: (value: number) => (value > 0 ? formatLatencySec(value) : "-"),
    },
    {
      title: "操作",
      key: "actions",
      width: 260,
      render: (_: unknown, record) => (
        <Space size={6} wrap>
          <Button size="small" onClick={() => onEditKeyMetadata(record)}>
            编辑
          </Button>
          <Button
            size="small"
            loading={updatingKeyMetadata}
            onClick={() => onToggleKeyDisabled(record, !(record.disabled ?? record.state === "disabled"))}
          >
            {record.disabled || record.state === "disabled" ? "启用" : "停用"}
          </Button>
          <Button size="small" loading={testingKeyID === record.key_id} onClick={() => onTestKey(record.key_id)}>
            测试
          </Button>
          <Button
            size="small"
            disabled={record.disabled || record.state === "disabled"}
            loading={resettingKey}
            onClick={() => onResetKey(record.key_id)}
          >
            重置
          </Button>
        </Space>
      ),
    },
  ];

  return (
    <Space direction="vertical" size={14} className="provider-detail-body">
      {contextHolder}

      {/* Key 统计 + Target URL */}
      <div className="detail-stats-row">
        <div className="detail-stat">
          <span>总 Key</span>
          <strong>{configuredKeys}</strong>
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
        <div className="detail-stat">
          <span>停用</span>
          <strong>{detail.disabled_keys}</strong>
        </div>
        <div className="detail-stat detail-stat--quota">
          <span>余额不足</span>
          <strong>{detail.quota_exhausted_keys}</strong>
        </div>
      </div>
      <div className="detail-target-url">
        <span className="detail-target-label">Target URL</span>
        <a href={detail.target_url} target="_blank" rel="noreferrer" className="detail-target-link">
          {detail.target_url}
        </a>
        <Button type="text" size="small" icon={<CopyOutlined />} onClick={copyTargetUrl} />
      </div>

      {/* 模型记录 */}
      <Card className="surface-card" bordered={false} size="small">
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">模型记录</Typography.Text>
            <Typography.Title level={5} className="section-title">
              已记录模型 {models.length > 0 ? `(${models.length})` : ""}
            </Typography.Title>
          </div>
          <Space wrap>
            <Button size="small" onClick={onOpenEditModels}>
              手动编辑
            </Button>
            <Button size="small" type="primary" loading={fetchingModels} onClick={onFetchModels}>
              同步模型
            </Button>
          </Space>
        </div>
        <div className="model-record-strip">
          <div className="model-record-stat">
            <span>记录数</span>
            <strong>{models.length}</strong>
          </div>
          <div className="model-record-stat">
            <span>保存方式</span>
            <strong>审阅后保存</strong>
          </div>
        </div>
        {models.length === 0 ? (
          <Empty description="暂无模型记录，可手动编辑或同步上游模型" image={Empty.PRESENTED_IMAGE_SIMPLE} />
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

      {/* Key 管理 */}
      <Card className="surface-card" bordered={false} size="small">
        <div className="section-heading">
          <div>
            <Typography.Text className="placeholder-kicker">Key 管理</Typography.Text>
            <Typography.Title level={5} className="section-title">
              当前 Keys
            </Typography.Title>
          </div>
          <Space wrap>
            <Button size="small" onClick={onOpenAppendKeys}>
              追加 Keys
            </Button>
            <Button size="small" danger onClick={onOpenReplaceKeys}>
              替换全部
            </Button>
            <Popconfirm
              title="确认重置全部启用 key 的状态？"
              description="停用 key 不会被重新启用。"
              okText="重置"
              cancelText="取消"
              onConfirm={onResetAllKeys}
            >
              <Button size="small" loading={resettingAllKeys}>
                重置全部
              </Button>
            </Popconfirm>
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
          scroll={{ x: 1390 }}
        />
      </Card>
    </Space>
  );
}

function renderInvalidReason(reason: string | undefined, state: AdminKeyStatus["state"]): JSX.Element | string {
  if (state === "disabled") {
    return <Tag color="default">手动停用</Tag>;
  }
  if (state === "cooling") {
    return <Tag color="gold">临时冷却</Tag>;
  }
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

function renderDateTime(value: string | undefined): JSX.Element | string {
  if (!value) {
    return "-";
  }
  return <Typography.Text className="table-subtext">{formatDateTime(value)}</Typography.Text>;
}
