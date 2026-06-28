import { CopyOutlined, DownOutlined, EllipsisOutlined, UpOutlined } from "@ant-design/icons";
import { Button, Card, Dropdown, Empty, Popconfirm, Space, Table, Tag, Tooltip, Typography, message } from "antd";
import type { MenuProps, TableColumnsType } from "antd";
import { useMemo, useState } from "react";

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
  const [modelsExpanded, setModelsExpanded] = useState(false);
  const models = detail.models ?? [];
  const configuredKeys = detail.total_keys + detail.disabled_keys;
  const PREVIEW_COUNT = 6;
  const previewModels = models.slice(0, PREVIEW_COUNT);
  const hasMore = models.length > PREVIEW_COUNT;

  function copyTargetUrl(): void {
    navigator.clipboard
      .writeText(detail.target_url)
      .then(() => messageApi.success("已复制 Target URL"))
      .catch(() => messageApi.error("复制失败"));
  }

  function buildKeyActionMenu(record: AdminKeyStatus): MenuProps["items"] {
    const isDisabled = record.disabled || record.state === "disabled";
    return [
      {
        key: "edit",
        label: "编辑",
        onClick: () => onEditKeyMetadata(record),
      },
      {
        key: "toggle",
        label: isDisabled ? "启用" : "停用",
        disabled: updatingKeyMetadata,
        onClick: () => onToggleKeyDisabled(record, !isDisabled),
      },
      {
        key: "test",
        label: testingKeyID === record.key_id ? "测试中…" : "测试",
        disabled: testingKeyID === record.key_id,
        onClick: () => onTestKey(record.key_id),
      },
      {
        key: "reset",
        label: "重置状态",
        disabled: isDisabled || resettingKey,
        onClick: () => onResetKey(record.key_id),
      },
    ];
  }

  const keyColumns: TableColumnsType<AdminKeyStatus> = useMemo(
    () => [
      {
        title: "Key 标识",
        dataIndex: "masked_key",
        key: "masked_key",
        width: 260,
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
        width: 100,
        render: (label: string | undefined, record) => {
          const note = record.note;
          const content = label || "-";
          if (!note) {
            return content;
          }
          return (
            <Tooltip title={`备注：${note}`} placement="topLeft">
              <span className="key-label-with-note">{content}</span>
            </Tooltip>
          );
        },
      },
      {
        title: "状态",
        dataIndex: "state",
        key: "state",
        width: 80,
        render: (state: AdminKeyStatus["state"], record) => {
          const stateEl = renderKeyState(state, record.invalid_reason);
          // Tooltip 显示低频信息：失效原因、冷却、最近 401
          const tooltipLines: string[] = [];
          if (state === "invalid" && record.invalid_reason) {
            const reasonMap: Record<string, string> = {
              quota_exhausted: "余额不足",
              unauthorized: "认证失败",
            };
            tooltipLines.push(`原因：${reasonMap[record.invalid_reason] ?? record.invalid_reason}`);
          }
          if (state === "cooling" && record.cool_until) {
            tooltipLines.push(`冷却至：${formatDateTime(record.cool_until)}`);
          }
          if (record.last_401_at) {
            tooltipLines.push(`最近 401：${formatDateTime(record.last_401_at)}`);
          }
          if (tooltipLines.length === 0) {
            return stateEl;
          }
          return (
            <Tooltip title={tooltipLines.join("\n")} placement="topLeft">
              {stateEl}
            </Tooltip>
          );
        },
      },
      {
        title: "统计",
        key: "stats",
        width: 130,
        render: (_: unknown, record) => {
          const latency = record.avg_latency_ms > 0 ? formatLatencySec(record.avg_latency_ms) : "-";
          return (
            <Tooltip
              title={
                <div>
                  <div>请求数：{record.req_count}</div>
                  <div>错误数：{record.err_count}</div>
                  <div>平均延迟：{latency}</div>
                </div>
              }
            >
              <span className="key-stats-compact">
                {record.req_count}/{record.err_count}/{latency}
              </span>
            </Tooltip>
          );
        },
      },
      {
        title: "操作",
        key: "actions",
        width: 60,
        render: (_: unknown, record) => (
          <Dropdown menu={{ items: buildKeyActionMenu(record) }} trigger={["click"]}>
            <Button type="text" size="small" icon={<EllipsisOutlined />} className="key-action-trigger" />
          </Dropdown>
        ),
      },
    ],
    [updatingKeyMetadata, testingKeyID, resettingKey, onEditKeyMetadata, onToggleKeyDisabled, onTestKey, onResetKey],
  );

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
        {models.length === 0 ? (
          <Empty description="暂无模型记录，可手动编辑或同步上游模型" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          <div className="model-dropdown">
            <div className="model-tags">
              {previewModels.map((model) => (
                <Tag key={model} className="model-tag">
                  {model}
                </Tag>
              ))}
              {hasMore && !modelsExpanded && (
                <Tag className="model-tag model-tag--more">+{models.length - PREVIEW_COUNT} 个</Tag>
              )}
            </div>
            {hasMore && (
              <Button
                type="text"
                size="small"
                className="model-dropdown-toggle"
                icon={modelsExpanded ? <UpOutlined /> : <DownOutlined />}
                onClick={() => setModelsExpanded(!modelsExpanded)}
              >
                {modelsExpanded ? "收起" : `查看全部 ${models.length} 个模型`}
              </Button>
            )}
            {modelsExpanded && (
              <div className="model-dropdown-full">
                <div className="model-tags">
                  {models.slice(PREVIEW_COUNT).map((model) => (
                    <Tag key={model} className="model-tag">
                      {model}
                    </Tag>
                  ))}
                </div>
              </div>
            )}
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
          className="provider-table provider-table--compact"
          columns={keyColumns}
          dataSource={detail.keys}
          pagination={false}
          rowKey="key_id"
          rowSelection={{
            selectedRowKeys: selectedKeyIDs,
            onChange: (rowKeys) => onSelectedKeyIDsChange(rowKeys.map(String)),
          }}
          scroll={{ x: 630 }}
        />
      </Card>
    </Space>
  );
}
