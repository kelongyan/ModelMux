import { Button, Checkbox, Form, Input, Modal, Space, Tag, Typography } from "antd";
import type { FormInstance } from "antd/es/form";
import type { AdminKeysPreviewResponse } from "../../types/admin";

function pasteFromClipboard(form: FormInstance, fieldName: string): void {
  void navigator.clipboard.readText().then((text) => {
    if (text) {
      const existing = (form.getFieldValue(fieldName) as string) ?? "";
      const combined = existing ? existing + "\n" + text : text;
      form.setFieldValue(fieldName, combined);
    }
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
