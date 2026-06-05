import { Form, Input, Modal } from "antd";
import type { FormInstance } from "antd/es/form";

import type {
  KeyFormValues,
  KeyModalState,
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
            extra="一行一个 key，保存时会自动去重并忽略空行。"
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
      okText={state.mode === "append" ? "追加" : "替换"}
      cancelText="取消"
      okButtonProps={{ danger: state.mode === "replace" }}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={confirmLoading}
    >
      <Form<KeyFormValues> form={form} layout="vertical" onFinish={onSubmit}>
        <Form.Item
          label="Keys"
          name="keys_text"
          rules={[{ required: true, message: "请至少输入一个 key" }]}
          extra={state.mode === "append" ? "新 key 会自动去重后追加。" : "替换会覆盖当前 provider 下的全部 keys。"}
        >
          <Input.TextArea rows={10} placeholder={"sk-key-a\nsk-key-b"} />
        </Form.Item>
      </Form>
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
