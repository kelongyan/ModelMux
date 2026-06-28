import { Alert, Button, Checkbox, Form, Input, Modal, Space, Table, Tag, Typography } from "antd";
import type { TableColumnsType } from "antd";
import type { FormInstance } from "antd/es/form";
import type { AdminKeysPreviewResponse, AdminKeyTestAllResult } from "../../types/admin";
import { buildModelSaveList, type ModelSaveMode, summarizeModelSync } from "./model-sync";

function pasteFromClipboard(form: FormInstance, fieldName: string): void {
  if (!navigator.clipboard?.readText) {
    return;
  }
  navigator.clipboard
    .readText()
    .then((text) => {
      if (text) {
        const existing = (form.getFieldValue(fieldName) as string) ?? "";
        const combined = existing ? existing + "\n" + text : text;
        form.setFieldValue(fieldName, combined);
      }
    })
    .catch(() => {
      // 用户拒绝剪贴板权限或浏览器不支持时静默忽略
    });
}

import type {
  KeyFormValues,
  KeyModalState,
  KeyMetadataFormValues,
  KeyMetadataModalState,
  KeyPreviewModalState,
  ModelFormValues,
  ModelModalState,
  ModelSyncModalState,
  ProviderFormValues,
  ProviderModalState,
} from "./provider-types";

type ProviderEditorModalProps = {
  state: ProviderModalState;
  form: FormInstance<ProviderFormValues>;
  confirmLoading: boolean;
  onCancel: () => void;
  onSubmit: (values: ProviderFormValues) => void;
};

export function ProviderEditorModal({
  state,
  form,
  confirmLoading,
  onCancel,
  onSubmit,
}: ProviderEditorModalProps): JSX.Element {
  return (
    <Modal
      destroyOnHidden
      open={state.open}
      title={state.mode === "create" ? "新增 Provider" : `编辑 Provider：${state.provider?.id ?? ""}`}
      okText={state.mode === "create" ? "创建" : "保存"}
      cancelText="取消"
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={confirmLoading}
    >
      <Form<ProviderFormValues> form={form} layout="vertical" onFinish={onSubmit}>
        <Form.Item
          label="Provider ID"
          name="id"
          rules={[
            { required: true, message: "请输入 provider id" },
            { pattern: /^[A-Za-z0-9_.-]+$/, message: "仅支持字母、数字、点、下划线和短横线" },
          ]}
          extra={state.mode === "edit" ? "阶段 3 暂不支持修改 provider id" : "建议使用稳定且易识别的 id，仅使用字母、数字、点、下划线和短横线。"}
        >
          <Input disabled={state.mode === "edit"} placeholder="例如 primary 或 backup" />
        </Form.Item>
        <Form.Item
          label="Target URL"
          name="target_url"
          rules={[
            { required: true, message: "请输入 target_url" },
            { type: "url", message: "请输入合法的绝对 URL" },
          ]}
        >
          <Input placeholder="https://your-provider.example.com" />
        </Form.Item>
        {state.mode === "create" ? (
          <Form.Item
            label="Keys"
            name="keys_text"
            rules={[{ required: true, message: "请至少输入一个 key" }]}
            extra={
              <Space>
                <span>一行一个 key，保存时会自动去重并忽略空行。</span>
                <Button type="link" size="small" onClick={() => pasteFromClipboard(form, "keys_text")}>
                  从剪贴板粘贴
                </Button>
              </Space>
            }
          >
            <Input.TextArea rows={8} placeholder={"sk-key-1\nsk-key-2"} />
          </Form.Item>
        ) : null}
      </Form>
    </Modal>
  );
}

type KeyEditorModalProps = {
  state: KeyModalState;
  form: FormInstance<KeyFormValues>;
  confirmLoading: boolean;
  onCancel: () => void;
  onSubmit: (values: KeyFormValues) => void;
};

export function KeyEditorModal({ state, form, confirmLoading, onCancel, onSubmit }: KeyEditorModalProps): JSX.Element {
  return (
    <Modal
      destroyOnHidden
      open={state.open}
      title={state.mode === "append" ? "追加 Keys" : "替换全部 Keys"}
      okText="预览"
      cancelText="取消"
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={confirmLoading}
    >
      <Form<KeyFormValues> form={form} layout="vertical" onFinish={onSubmit}>
        <Form.Item
          label="Keys"
          name="keys_text"
          rules={[{ required: true, message: "请至少输入一个 key" }]}
          extra={
            <Space direction="vertical" size={2}>
              <span>{state.mode === "append" ? "新 key 会自动去重后进入预览。" : "替换会覆盖当前 provider 下的全部 keys，并先进入预览。"}</span>
              <Button type="link" size="small" onClick={() => pasteFromClipboard(form, "keys_text")}>
                从剪贴板粘贴
              </Button>
            </Space>
          }
        >
          <Input.TextArea rows={10} placeholder={"sk-key-a\nsk-key-b"} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

type KeyMetadataModalProps = {
  state: KeyMetadataModalState;
  form: FormInstance<KeyMetadataFormValues>;
  confirmLoading: boolean;
  onCancel: () => void;
  onSubmit: (values: KeyMetadataFormValues) => void;
};

export function KeyMetadataModal({
  state,
  form,
  confirmLoading,
  onCancel,
  onSubmit,
}: KeyMetadataModalProps): JSX.Element {
  return (
    <Modal
      destroyOnHidden
      open={state.open}
      title={state.key ? `编辑 Key：${state.key.masked_key}` : "编辑 Key"}
      okText="保存"
      cancelText="取消"
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={confirmLoading}
    >
      <Form<KeyMetadataFormValues> form={form} layout="vertical" onFinish={onSubmit}>
        <Form.Item label="标签" name="label" extra="给这把 key 起一个容易识别的名字。">
          <Input placeholder="例如 主力 / 备用 / 便宜线路" />
        </Form.Item>
        <Form.Item label="备注" name="note" extra="可写使用场景、来源或者临时说明。">
          <Input.TextArea rows={4} placeholder="例如 仅用于测试流量" />
        </Form.Item>
        <Form.Item name="disabled" valuePropName="checked" style={{ marginBottom: 0 }}>
          <Checkbox>停用这把 key</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}

type KeyPreviewModalProps = {
  state: KeyPreviewModalState;
  confirmLoading: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function KeyPreviewModal({
  state,
  confirmLoading,
  onCancel,
  onConfirm,
}: KeyPreviewModalProps): JSX.Element {
  const preview = state.preview;
  return (
    <Modal
      destroyOnHidden
      open={state.open}
      title={state.mode === "append" ? "预览追加结果" : "预览替换结果"}
      okText={state.mode === "append" ? "确认追加" : "确认替换"}
      cancelText="返回编辑"
      okButtonProps={{ danger: state.mode === "replace" }}
      onCancel={onCancel}
      onOk={onConfirm}
      confirmLoading={confirmLoading}
      width={760}
    >
      {preview ? (
        <Space direction="vertical" size={12} style={{ width: "100%" }}>
          <Typography.Text>
            输入 {preview.input_count} 行，归一化后 {preview.normalized_count} 条，重复 {preview.duplicate_count} 条。
          </Typography.Text>
          <Space wrap>
            <Tag color="green">新增 {preview.new_count}</Tag>
            <Tag color="blue">已存在 {preview.existing_count}</Tag>
            {state.mode === "replace" ? <Tag color="red">移除 {preview.removed_count}</Tag> : null}
          </Space>
          <PreviewSection title="新增 keys" items={preview.new_keys} />
          <PreviewSection title="已存在 keys" items={preview.existing_keys} />
          {state.mode === "replace" ? <PreviewSection title="将被移除的 keys" items={preview.removed_keys} /> : null}
        </Space>
      ) : (
        <Typography.Text type="secondary">没有可显示的预览结果。</Typography.Text>
      )}
    </Modal>
  );
}

type ModelEditorModalProps = {
  state: ModelModalState;
  selectedProviderID: string | null;
  form: FormInstance<ModelFormValues>;
  confirmLoading: boolean;
  onCancel: () => void;
  onSubmit: (values: ModelFormValues) => void;
};

export function ModelEditorModal({
  state,
  selectedProviderID,
  form,
  confirmLoading,
  onCancel,
  onSubmit,
}: ModelEditorModalProps): JSX.Element {
  return (
    <Modal
      destroyOnHidden
      open={state.open}
      title={`模型记录：${selectedProviderID ?? ""}`}
      okText="保存"
      cancelText="取消"
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={confirmLoading}
    >
      <Form<ModelFormValues> form={form} layout="vertical" onFinish={onSubmit}>
        <Form.Item
          label="模型 ID"
          name="models_text"
          extra="一行一个模型 ID，保存时自动去重、排序并忽略空行。也可点击「从上游拉取」自动获取。"
        >
          <Input.TextArea rows={10} placeholder={"gpt-4o\ngpt-4o-mini\nclaude-3-5-sonnet-20241022"} />
        </Form.Item>
      </Form>
    </Modal>
  );
}

type ModelSyncRow = {
  id: string;
  status: "new" | "existing";
};

type ModelSyncModalProps = {
  state: ModelSyncModalState;
  selectedModelIDs: string[];
  searchValue: string;
  confirmLoading: boolean;
  onCancel: () => void;
  onSearchChange: (value: string) => void;
  onSelectedModelIDsChange: (modelIDs: string[]) => void;
  onSave: (mode: ModelSaveMode) => void;
  onOpenManualEdit: () => void;
};

export function ModelSyncModal({
  state,
  selectedModelIDs,
  searchValue,
  confirmLoading,
  onCancel,
  onSearchChange,
  onSelectedModelIDsChange,
  onSave,
  onOpenManualEdit,
}: ModelSyncModalProps): JSX.Element {
  const summary = summarizeModelSync(state.currentModels, state.fetchedModels);
  const newModelSet = new Set(summary.newModels);
  const selectedModelSet = new Set(selectedModelIDs);
  const search = searchValue.trim().toLowerCase();
  const rows = summary.fetchedModels.map<ModelSyncRow>((model) => ({
    id: model,
    status: newModelSet.has(model) ? "new" : "existing",
  }));
  const filteredRows = search ? rows.filter((row) => row.id.toLowerCase().includes(search)) : rows;
  const replaceModels = buildModelSaveList({
    mode: "replace",
    currentModels: summary.currentModels,
    selectedModels: selectedModelIDs,
  });
  const mergeModels = buildModelSaveList({
    mode: "merge",
    currentModels: summary.currentModels,
    selectedModels: selectedModelIDs,
  });
  const replaceRemovedCount = summary.currentModels.filter((model) => !selectedModelSet.has(model)).length;

  const columns: TableColumnsType<ModelSyncRow> = [
    {
      title: "模型 ID",
      dataIndex: "id",
      key: "id",
      render: (model: string) => <span className="model-sync-id">{model}</span>,
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      width: 104,
      render: (status: ModelSyncRow["status"]) =>
        status === "new" ? <Tag color="green">新增</Tag> : <Tag color="blue">已记录</Tag>,
    },
  ];

  return (
    <Modal destroyOnHidden open={state.open} title={`同步模型：${state.providerID ?? ""}`} footer={null} onCancel={onCancel} width={860}>
      <Space direction="vertical" size={14} style={{ width: "100%" }}>
        <div className="model-sync-summary">
          <ModelSyncMetric label="当前记录" value={summary.currentModels.length} />
          <ModelSyncMetric label="上游返回" value={summary.fetchedModels.length} />
          <ModelSyncMetric label="新增" value={summary.newModels.length} tone="success" />
          <ModelSyncMetric label="将移除" value={summary.removedModels.length} tone="danger" />
        </div>

        {summary.fetchedModels.length === 0 ? (
          <Alert
            type="warning"
            showIcon
            message="上游未返回模型"
            description="本次同步不会覆盖当前记录，可切换 key 后重试，或打开手动编辑。"
          />
        ) : (
          <>
            <Input.Search
              allowClear
              value={searchValue}
              placeholder="搜索模型 ID"
              onChange={(event) => onSearchChange(event.target.value)}
              onSearch={onSearchChange}
            />
            <Table<ModelSyncRow>
              size="small"
              className="model-sync-table"
              columns={columns}
              dataSource={filteredRows}
              rowKey="id"
              pagination={{ pageSize: 8, showSizeChanger: false }}
              rowSelection={{
                selectedRowKeys: selectedModelIDs,
                preserveSelectedRowKeys: true,
                onChange: (keys) => onSelectedModelIDsChange(keys.map(String)),
              }}
            />
            <Space direction="vertical" size={6} className="model-sync-impact">
              <Typography.Text type="secondary">
                已选择 {selectedModelIDs.length} 个；替换后保存 {replaceModels.length} 个，合并后保存 {mergeModels.length} 个。
              </Typography.Text>
              {replaceRemovedCount > 0 ? (
                <Typography.Text type="secondary">替换会移除当前记录中未被选中的 {replaceRemovedCount} 个模型。</Typography.Text>
              ) : null}
            </Space>
            {summary.removedModels.length > 0 ? <ModelIDTagSection title="上游未返回" items={summary.removedModels} /> : null}
          </>
        )}

        <div className="model-sync-actions">
          <Button onClick={onOpenManualEdit}>手动编辑</Button>
          <Space wrap>
            <Button disabled={selectedModelIDs.length === 0} loading={confirmLoading} onClick={() => onSave("merge")}>
              合并新增模型
            </Button>
            <Button type="primary" disabled={selectedModelIDs.length === 0} loading={confirmLoading} onClick={() => onSave("replace")}>
              替换为选中模型
            </Button>
          </Space>
        </div>
      </Space>
    </Modal>
  );
}

function ModelSyncMetric({
  label,
  value,
  tone = "default",
}: {
  label: string;
  value: number;
  tone?: "default" | "success" | "danger";
}): JSX.Element {
  return (
    <div className={`model-sync-metric model-sync-metric--${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function ModelIDTagSection({ title, items }: { title: string; items: string[] }): JSX.Element {
  return (
    <Space direction="vertical" size={6} style={{ width: "100%" }}>
      <Typography.Text strong>{title}</Typography.Text>
      <div className="model-tags model-sync-tags">
        {items.map((item) => (
          <Tag key={item} className="model-tag">
            {item}
          </Tag>
        ))}
      </div>
    </Space>
  );
}

type KeyTestAllResultModalProps = {
  open: boolean;
  results: AdminKeyTestAllResult[];
  allOK: boolean;
  onClose: () => void;
};

export function KeyTestAllResultModal({ open, results, allOK, onClose }: KeyTestAllResultModalProps): JSX.Element {
  const passedCount = results.filter((r) => r.ok).length;
  const failedCount = results.length - passedCount;

  const columns: TableColumnsType<AdminKeyTestAllResult> = [
    {
      title: "Key 标识",
      dataIndex: "masked_key",
      key: "masked_key",
      width: 200,
      render: (maskedKey: string, record) => (
        <div>
          <strong>{maskedKey}</strong>
          <div className="table-subtext provider-key-id">{record.key_id}</div>
        </div>
      ),
    },
    {
      title: "状态",
      key: "status",
      width: 100,
      render: (_: unknown, record) => {
        if (record.error === "key is disabled") {
          return <Tag>停用</Tag>;
        }
        return record.ok ? <Tag color="success">正常</Tag> : <Tag color="error">异常</Tag>;
      },
    },
    {
      title: "HTTP",
      dataIndex: "status_code",
      key: "status_code",
      width: 80,
      render: (value?: number) => (value ? String(value) : "-"),
    },
    {
      title: "延迟",
      dataIndex: "latency_ms",
      key: "latency_ms",
      width: 80,
      render: (value?: number) => (value ? `${(value / 1000).toFixed(1)}s` : "-"),
    },
    {
      title: "详情",
      dataIndex: "error",
      key: "error",
      ellipsis: true,
      render: (value: string | undefined, record) => {
        if (!value && record.ok) {
          return <span className="stats-text-muted">-</span>;
        }
        return <span className="stats-text-error" title={value}>{value ?? "-"}</span>;
      },
    },
  ];

  return (
    <Modal
      open={open}
      title="批量测试结果"
      onCancel={onClose}
      footer={<Button onClick={onClose}>关闭</Button>}
      width="min(760px, 92vw)"
      destroyOnClose
    >
      <div className="detail-stats-row" style={{ marginBottom: 16 }}>
        <div className="detail-stat">
          <span>总计</span>
          <strong>{results.length}</strong>
        </div>
        <div className="detail-stat">
          <span>正常</span>
          <strong style={{ color: "#3d8b50" }}>{passedCount}</strong>
        </div>
        <div className="detail-stat">
          <span>异常</span>
          <strong style={{ color: failedCount > 0 ? "#a83535" : undefined }}>{failedCount}</strong>
        </div>
      </div>
      <Alert
        type={allOK ? "success" : "warning"}
        showIcon
        message={allOK ? "全部 Key 测试通过" : `有 ${failedCount} 个 Key 存在异常`}
        style={{ marginBottom: 16 }}
      />
      <Table
        columns={columns}
        dataSource={results}
        rowKey="key_id"
        pagination={false}
        size="small"
        scroll={{ x: 600 }}
      />
    </Modal>
  );
}

function PreviewSection({ title, items }: { title: string; items: AdminKeysPreviewResponse["new_keys"] }): JSX.Element {
  return (
    <Space direction="vertical" size={6} style={{ width: "100%" }}>
      <Typography.Text strong>{title}</Typography.Text>
      {items.length === 0 ? (
        <Typography.Text type="secondary">-</Typography.Text>
      ) : (
        <div className="model-tags">
          {items.map((item) => (
            <Tag key={item.key_id} title={item.key_id}>
              {item.label ? `${item.label} · ${item.masked_key}` : item.masked_key}
            </Tag>
          ))}
        </div>
      )}
    </Space>
  );
}
