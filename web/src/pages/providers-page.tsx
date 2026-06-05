import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Button,
  Card,
  Descriptions,
  Drawer,
  Empty,
  Form,
  Input,
  Modal,
  Popconfirm,
  Result,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import type { TableColumnsType } from "antd";
import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import {
  activateProvider,
  appendProviderKeys,
  createProvider,
  deleteProvider,
  deleteProviderKeys,
  fetchProviderDetail,
  fetchProviderModels,
  fetchProviders,
  replaceProviderKeys,
  replaceProviderModels,
  resetProviderKey,
  updateProvider,
} from "../api/admin";
import { CooldownText } from "../components/cooldown-text";
import { formatRelativeTime } from "../components/format-time";
import type {
  AdminKeyStatus,
  AdminProviderDetailResponse,
  AdminProviderSummary,
} from "../types/admin";

type ProviderFormMode = "create" | "edit";
type KeyFormMode = "append" | "replace";

type ProviderFormValues = {
  id: string;
  target_url: string;
  keys_text: string;
};

type KeyFormValues = {
  keys_text: string;
};

type ModelFormValues = {
  models_text: string;
};

const providersQueryKey = ["providers"];

// ProvidersPage 提供阶段 3 的 provider 与 key 管理主界面。
export function ProvidersPage(): JSX.Element {
  const queryClient = useQueryClient();
  const [messageApi, contextHolder] = message.useMessage();
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedProviderID, setSelectedProviderID] = useState<string | null>(() => searchParams.get("provider"));
  const [selectedKeyIDs, setSelectedKeyIDs] = useState<string[]>([]);
  const [providerModal, setProviderModal] = useState<{ open: boolean; mode: ProviderFormMode; provider?: AdminProviderSummary }>({
    open: false,
    mode: "create",
  });
  const [keyModal, setKeyModal] = useState<{ open: boolean; mode: KeyFormMode }>({
    open: false,
    mode: "append",
  });
  const [modelModal, setModelModal] = useState<{ open: boolean }>({ open: false });

  const [providerForm] = Form.useForm<ProviderFormValues>();
  const [keyForm] = Form.useForm<KeyFormValues>();
  const [modelForm] = Form.useForm<ModelFormValues>();

  const providersQuery = useQuery({
    queryKey: providersQueryKey,
    queryFn: fetchProviders,
    refetchInterval: 8000,
  });

  const providerDetailQuery = useQuery({
    queryKey: ["provider-detail", selectedProviderID],
    queryFn: async () => {
      if (!selectedProviderID) {
        throw new Error("missing provider id");
      }
      return fetchProviderDetail(selectedProviderID);
    },
    enabled: selectedProviderID !== null,
    refetchInterval: selectedProviderID ? 5000 : false,
  });

  useEffect(() => {
    setSelectedKeyIDs([]);
  }, [selectedProviderID]);

  // syncSelectionFromURL 让 Dashboard 卡片"详情 →"跳转过来时能直接打开对应 drawer。
  useEffect(() => {
    const fromUrl = searchParams.get("provider");
    if (fromUrl !== selectedProviderID) {
      setSelectedProviderID(fromUrl);
    }
  }, [searchParams, selectedProviderID]);

  const setSelectedProvider = (providerID: string | null) => {
    setSelectedProviderID(providerID);
    const next = new URLSearchParams(searchParams);
    if (providerID) {
      next.set("provider", providerID);
    } else {
      next.delete("provider");
    }
    setSearchParams(next, { replace: true });
  };

  const invalidateAdminQueries = async (providerID?: string) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["dashboard"] }),
      queryClient.invalidateQueries({ queryKey: providersQueryKey }),
      queryClient.invalidateQueries({ queryKey: ["events"] }),
      providerID
        ? queryClient.invalidateQueries({ queryKey: ["provider-detail", providerID] })
        : Promise.resolve(),
    ]);
  };

  const createProviderMutation = useMutation({
    mutationFn: createProvider,
    onSuccess: async () => {
      messageApi.success("已新增 provider");
      setProviderModal({ open: false, mode: "create" });
      providerForm.resetFields();
      await invalidateAdminQueries();
    },
    onError: (error: Error) => {
      messageApi.error(`新增失败：${error.message}`);
    },
  });

  const updateProviderMutation = useMutation({
    mutationFn: async (payload: { id: string; target_url: string }) =>
      updateProvider(payload.id, { target_url: payload.target_url }),
    onSuccess: async (_, variables) => {
      messageApi.success("已更新 provider");
      setProviderModal({ open: false, mode: "create" });
      providerForm.resetFields();
      await invalidateAdminQueries(variables.id);
    },
    onError: (error: Error) => {
      messageApi.error(`更新失败：${error.message}`);
    },
  });

  const deleteProviderMutation = useMutation({
    mutationFn: deleteProvider,
    onSuccess: async (_, providerID) => {
      messageApi.success("已删除 provider");
      if (selectedProviderID === providerID) {
        setSelectedProvider(null);
      }
      await invalidateAdminQueries();
    },
    onError: (error: Error) => {
      messageApi.error(`删除失败：${error.message}`);
    },
  });

  const activateProviderMutation = useMutation({
    mutationFn: activateProvider,
    onSuccess: async (_, providerID) => {
      messageApi.success(`已切换到 ${providerID}`);
      await invalidateAdminQueries(providerID);
    },
    onError: (error: Error) => {
      messageApi.error(`切换失败：${error.message}`);
    },
  });

  const appendKeysMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keys: string[] }) =>
      appendProviderKeys(payload.providerID, { keys: payload.keys }),
    onSuccess: async (_, variables) => {
      messageApi.success("已追加 keys");
      setKeyModal({ open: false, mode: "append" });
      keyForm.resetFields();
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => {
      messageApi.error(`追加失败：${error.message}`);
    },
  });

  const replaceKeysMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keys: string[] }) =>
      replaceProviderKeys(payload.providerID, { keys: payload.keys }),
    onSuccess: async (_, variables) => {
      messageApi.success("已替换全部 keys");
      setKeyModal({ open: false, mode: "append" });
      keyForm.resetFields();
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => {
      messageApi.error(`替换失败：${error.message}`);
    },
  });

  const deleteKeysMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keyIDs: string[] }) =>
      deleteProviderKeys(payload.providerID, { key_ids: payload.keyIDs }),
    onSuccess: async (_, variables) => {
      messageApi.success("已删除选中 keys");
      setSelectedKeyIDs([]);
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => {
      messageApi.error(`删除 keys 失败：${error.message}`);
    },
  });

  const resetKeyMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keyID: string }) =>
      resetProviderKey(payload.providerID, payload.keyID),
    onSuccess: async (_, variables) => {
      messageApi.success("已重置 key 状态");
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => {
      messageApi.error(`重置失败：${error.message}`);
    },
  });

  const replaceModelsMutation = useMutation({
    mutationFn: async (payload: { providerID: string; models: string[] }) =>
      replaceProviderModels(payload.providerID, { models: payload.models }),
    onSuccess: async (_, variables) => {
      messageApi.success(`已更新模型记录（${variables.models.length} 个）`);
      setModelModal({ open: false });
      modelForm.resetFields();
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => {
      messageApi.error(`更新模型失败：${error.message}`);
    },
  });

  const fetchModelsMutation = useMutation({
    mutationFn: async (providerID: string) => fetchProviderModels(providerID),
    onSuccess: async (data, providerID) => {
      messageApi.success(`从上游拉取到 ${data.count} 个模型`);
      modelForm.setFieldsValue({ models_text: data.models.join("\n") });
      setModelModal({ open: true });
      await invalidateAdminQueries(providerID);
    },
    onError: (error: Error) => {
      messageApi.error(`拉取模型失败：${error.message}`);
    },
  });

  const providerColumns = useMemo<TableColumnsType<AdminProviderSummary>>(
    () => [
      {
        title: "Provider",
        dataIndex: "id",
        key: "id",
        render: (_: string, record) => (
          <div className="provider-table-id">{record.id}</div>
        ),
      },
      {
        title: "状态",
        dataIndex: "active",
        key: "active",
        render: (_active: boolean, record) => renderProviderState(record),
      },
      { title: "总 Key", dataIndex: "total_keys", key: "total_keys" },
      { title: "可用", dataIndex: "active_keys", key: "active_keys" },
      { title: "冷却", dataIndex: "cooling_keys", key: "cooling_keys" },
      { title: "失效", dataIndex: "invalid_keys", key: "invalid_keys" },
      {
        title: "操作",
        key: "actions",
        render: (_: unknown, record) => (
          <div className="provider-actions">
            <button
              className="provider-action provider-action--primary"
              onClick={() => setSelectedProvider(record.id)}
            >
              详情
            </button>
            {!record.active ? (
              <button
                className="provider-action provider-action--activate"
                disabled={activateProviderMutation.isPending}
                onClick={() => activateProviderMutation.mutate(record.id)}
              >
                激活
              </button>
            ) : null}
            <button
              className="provider-action provider-action--edit"
              onClick={() => openProviderEdit(record)}
            >
              编辑
            </button>
            <Popconfirm
              title={`确认删除 provider ${record.id}？`}
              description="删除后将同时移除其全部 keys。"
              okText="删除"
              cancelText="取消"
              onConfirm={() => deleteProviderMutation.mutate(record.id)}
            >
              <button
                className="provider-action provider-action--danger"
                disabled={deleteProviderMutation.isPending}
              >
                删除
              </button>
            </Popconfirm>
          </div>
        ),
      },
    ],
    [activateProviderMutation.isPending, deleteProviderMutation.isPending],
  );

  const keyColumns = useMemo<TableColumnsType<AdminKeyStatus>>(
    () => [
      {
        title: "Key 标识",
        dataIndex: "masked_key",
        key: "masked_key",
        render: (maskedKey: string, record) => (
          <div>
            <strong>{maskedKey}</strong>
            <div className="table-subtext">{record.key_id}</div>
          </div>
        ),
      },
      {
        title: "状态",
        dataIndex: "state",
        key: "state",
        render: (state: AdminKeyStatus["state"]) => renderKeyState(state),
      },
      { title: "请求数", dataIndex: "req_count", key: "req_count" },
      { title: "错误数", dataIndex: "err_count", key: "err_count" },
      {
        title: "平均延迟",
        dataIndex: "avg_latency_ms",
        key: "avg_latency_ms",
        render: (value: number) => `${value.toFixed(1)} ms`,
      },
      {
        title: "冷却倒计时",
        dataIndex: "cool_until",
        key: "cool_until",
        render: (value?: string) => <CooldownText until={value} />,
      },
      {
        title: "最近 401",
        dataIndex: "last_401_at",
        key: "last_401_at",
        render: (value?: string) => formatRelativeTime(value),
      },
      {
        title: "操作",
        key: "actions",
        render: (_: unknown, record) => (
          <Button
            size="small"
            disabled={!selectedProviderID}
            loading={resetKeyMutation.isPending}
            onClick={() => {
              if (!selectedProviderID) {
                return;
              }
              resetKeyMutation.mutate({ providerID: selectedProviderID, keyID: record.key_id });
            }}
          >
            重置状态
          </Button>
        ),
      },
    ],
    [resetKeyMutation.isPending, selectedProviderID],
  );

  const selectedProvider = providersQuery.data?.providers.find((provider) => provider.id === selectedProviderID);
  const providerDetail = providerDetailQuery.data;
  const detailLoading = providerDetailQuery.isLoading && selectedProviderID !== null;

  if (providersQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (providersQuery.isError || !providersQuery.data) {
    return (
      <Result
        status="error"
        title="Provider 列表加载失败"
        subTitle={providersQuery.error instanceof Error ? providersQuery.error.message : "未知错误"}
      />
    );
  }

  const providers = providersQuery.data.providers;

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={16} className="console-stack">
        <Card className="surface-card" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Providers</Typography.Text>
              <Typography.Title level={3} className="section-title">
                Provider 列表
              </Typography.Title>
            </div>
            <Space wrap>
              <Button onClick={() => void providersQuery.refetch()}>刷新</Button>
              <Button type="primary" onClick={openProviderCreate}>
                新增 Provider
              </Button>
            </Space>
          </div>
          {providers.length === 0 ? (
            <Empty description="当前没有 provider 配置" />
          ) : (
            <Table
              className="provider-table"
              columns={providerColumns}
              dataSource={providers}
              pagination={false}
              rowKey="id"
              rowClassName={(record) => (record.active ? "provider-table-row--active" : "")}
              scroll={{ x: 920 }}
            />
          )}
        </Card>
      </Space>

      <Modal
        destroyOnHidden
        open={providerModal.open}
        title={providerModal.mode === "create" ? "新增 Provider" : `编辑 Provider：${providerModal.provider?.id ?? ""}`}
        okText={providerModal.mode === "create" ? "创建" : "保存"}
        cancelText="取消"
        onCancel={closeProviderModal}
        onOk={() => providerForm.submit()}
        confirmLoading={createProviderMutation.isPending || updateProviderMutation.isPending}
      >
        <Form<ProviderFormValues> form={providerForm} layout="vertical" onFinish={submitProviderForm}>
          <Form.Item
            label="Provider ID"
            name="id"
            rules={[
              { required: true, message: "请输入 provider id" },
              { pattern: /^[A-Za-z0-9_.-]+$/, message: "仅支持字母、数字、点、下划线和短横线" },
            ]}
            extra={providerModal.mode === "edit" ? "阶段 3 暂不支持修改 provider id" : "建议使用稳定且易识别的 id，仅使用字母、数字、点、下划线和短横线。"}
          >
            <Input disabled={providerModal.mode === "edit"} placeholder="例如 primary 或 backup" />
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
          {providerModal.mode === "create" ? (
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

      <Modal
        destroyOnHidden
        open={keyModal.open}
        title={keyModal.mode === "append" ? "追加 Keys" : "替换全部 Keys"}
        okText={keyModal.mode === "append" ? "追加" : "替换"}
        cancelText="取消"
        okButtonProps={{ danger: keyModal.mode === "replace" }}
        onCancel={closeKeyModal}
        onOk={() => keyForm.submit()}
        confirmLoading={appendKeysMutation.isPending || replaceKeysMutation.isPending}
      >
        <Form<KeyFormValues> form={keyForm} layout="vertical" onFinish={submitKeyForm}>
          <Form.Item
            label="Keys"
            name="keys_text"
            rules={[{ required: true, message: "请至少输入一个 key" }]}
            extra={keyModal.mode === "append" ? "新 key 会自动去重后追加。" : "替换会覆盖当前 provider 下的全部 keys。"}
          >
            <Input.TextArea rows={10} placeholder={"sk-key-a\nsk-key-b"} />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        destroyOnHidden
        open={modelModal.open}
        title={`模型记录：${selectedProviderID ?? ""}`}
        okText="保存"
        cancelText="取消"
        onCancel={closeModelModal}
        onOk={() => modelForm.submit()}
        confirmLoading={replaceModelsMutation.isPending}
      >
        <Form<ModelFormValues> form={modelForm} layout="vertical" onFinish={submitModelForm}>
          <Form.Item
            label="模型 ID"
            name="models_text"
            extra="一行一个模型 ID，保存时自动去重、排序并忽略空行。也可点击「从上游拉取」自动获取。"
          >
            <Input.TextArea rows={10} placeholder={"gpt-4o\ngpt-4o-mini\nclaude-3-5-sonnet-20241022"} />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        open={selectedProviderID !== null}
        width={860}
        title={selectedProvider ? `Provider 详情：${selectedProvider.id}` : "Provider 详情"}
        onClose={() => setSelectedProvider(null)}
      >
        {detailLoading ? (
          <div className="console-loading">
            <Spin />
          </div>
        ) : providerDetailQuery.isError ? (
          <Result
            status="error"
            title="Provider 详情加载失败"
            subTitle={providerDetailQuery.error instanceof Error ? providerDetailQuery.error.message : "未知错误"}
          />
        ) : providerDetail ? (
          <ProviderDetailContent
            detail={providerDetail}
            selectedKeyIDs={selectedKeyIDs}
            setSelectedKeyIDs={setSelectedKeyIDs}
            keyColumns={keyColumns}
            onOpenAppendKeys={() => openKeyModal("append")}
            onOpenReplaceKeys={() => openKeyModal("replace")}
            onDeleteSelectedKeys={() => {
              if (!selectedProviderID || selectedKeyIDs.length === 0) {
                return;
              }
              deleteKeysMutation.mutate({ providerID: selectedProviderID, keyIDs: selectedKeyIDs });
            }}
            deletingKeys={deleteKeysMutation.isPending}
            onOpenEditModels={() => openModelModal(providerDetail.models)}
            onFetchModels={() => {
              if (selectedProviderID) {
                fetchModelsMutation.mutate(selectedProviderID);
              }
            }}
            fetchingModels={fetchModelsMutation.isPending}
          />
        ) : (
          <Empty description="未找到 provider 详情" />
        )}
      </Drawer>
    </>
  );

  function openProviderCreate() {
    providerForm.setFieldsValue({
      id: "",
      target_url: "",
      keys_text: "",
    });
    setProviderModal({ open: true, mode: "create" });
  }

  function openProviderEdit(provider: AdminProviderSummary) {
    providerForm.setFieldsValue({
      id: provider.id,
      target_url: provider.target_url,
      keys_text: "",
    });
    setProviderModal({ open: true, mode: "edit", provider });
  }

  function closeProviderModal() {
    setProviderModal({ open: false, mode: "create" });
    providerForm.resetFields();
  }

  function openKeyModal(mode: KeyFormMode) {
    keyForm.setFieldsValue({ keys_text: "" });
    setKeyModal({ open: true, mode });
  }

  function closeKeyModal() {
    setKeyModal({ open: false, mode: "append" });
    keyForm.resetFields();
  }

  function openModelModal(currentModels: string[]) {
    modelForm.setFieldsValue({ models_text: currentModels.join("\n") });
    setModelModal({ open: true });
  }

  function closeModelModal() {
    setModelModal({ open: false });
    modelForm.resetFields();
  }

  async function submitModelForm(values: ModelFormValues) {
    if (!selectedProviderID) {
      messageApi.error("请先选择 provider");
      return;
    }
    const models = values.models_text
      .split(/\r?\n/g)
      .map((m: string) => m.trim())
      .filter((m: string) => m.length > 0);
    await replaceModelsMutation.mutateAsync({ providerID: selectedProviderID, models });
  }

  async function submitProviderForm(values: ProviderFormValues) {
    const keys = splitKeysText(values.keys_text);

    if (providerModal.mode === "create") {
      await createProviderMutation.mutateAsync({
        id: values.id.trim(),
        target_url: values.target_url.trim(),
        keys,
      });
      return;
    }

    const providerID = providerModal.provider?.id;
    if (!providerID) {
      messageApi.error("缺少 provider id");
      return;
    }
    await updateProviderMutation.mutateAsync({
      id: providerID,
      target_url: values.target_url.trim(),
    });
  }

  async function submitKeyForm(values: KeyFormValues) {
    if (!selectedProviderID) {
      messageApi.error("请先选择 provider");
      return;
    }
    const keys = splitKeysText(values.keys_text);
    if (keyModal.mode === "append") {
      await appendKeysMutation.mutateAsync({ providerID: selectedProviderID, keys });
      return;
    }
    await replaceKeysMutation.mutateAsync({ providerID: selectedProviderID, keys });
  }
}

type ProviderDetailContentProps = {
  detail: AdminProviderDetailResponse;
  selectedKeyIDs: string[];
  setSelectedKeyIDs: (keyIDs: string[]) => void;
  keyColumns: TableColumnsType<AdminKeyStatus>;
  onOpenAppendKeys: () => void;
  onOpenReplaceKeys: () => void;
  onDeleteSelectedKeys: () => void;
  deletingKeys: boolean;
  onOpenEditModels: () => void;
  onFetchModels: () => void;
  fetchingModels: boolean;
};

// ProviderDetailContent 渲染 provider 详情抽屉中的摘要与 key 管理区域。
function ProviderDetailContent({
  detail,
  selectedKeyIDs,
  setSelectedKeyIDs,
  keyColumns,
  onOpenAppendKeys,
  onOpenReplaceKeys,
  onDeleteSelectedKeys,
  deletingKeys,
  onOpenEditModels,
  onFetchModels,
  fetchingModels,
}: ProviderDetailContentProps): JSX.Element {
  const models = detail.models ?? [];
  return (
    <Space direction="vertical" size={14} className="console-stack">
      <Card className="surface-card" bordered={false}>
        <div className="detail-stats-row">
          <div className="detail-stat">
            <span>总 Key</span>
            <strong>{detail.total_keys}</strong>
          </div>
          <div className="detail-stat">
            <span>可用</span>
            <strong style={{ color: "#16a34a" }}>{detail.active_keys}</strong>
          </div>
          <div className="detail-stat">
            <span>冷却</span>
            <strong style={{ color: "#d97706" }}>{detail.cooling_keys}</strong>
          </div>
          <div className="detail-stat">
            <span>失效</span>
            <strong style={{ color: "#dc2626" }}>{detail.invalid_keys}</strong>
          </div>
        </div>
        <Descriptions
          className="provider-detail-descriptions"
          column={{ xs: 1, sm: 2 }}
          items={[
            { key: "id", label: "Provider ID", children: detail.id },
            {
              key: "active",
              label: "状态",
              children: detail.active ? <StateText color="green">当前活跃</StateText> : <StateText color="gray">待命</StateText>,
            },
            { key: "target", label: "Target URL", children: <a href={detail.target_url} target="_blank" rel="noreferrer" style={{ color: "#2563eb" }}>{detail.target_url}</a> },
          ]}
        />
      </Card>

      <Card className="surface-card" bordered={false}>
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

      <Card className="surface-card" bordered={false}>
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
            onChange: (rowKeys) => setSelectedKeyIDs(rowKeys.map(String)),
          }}
        />
      </Card>
    </Space>
  );
}

// renderKeyState 把后端 key 状态映射为更易识别的标签。
function renderProviderState(provider: AdminProviderSummary): JSX.Element {
  if (provider.active) {
    return <StateText color="green">当前活跃</StateText>;
  }
  if (provider.active_keys === 0 && provider.cooling_keys === 0) {
    return <StateText color="red">不可用</StateText>;
  }
  if (provider.cooling_keys > 0 || provider.invalid_keys > 0) {
    return <StateText color="gold">波动中</StateText>;
  }
  return <StateText color="blue">待命</StateText>;
}

function renderKeyState(state: AdminKeyStatus["state"]): JSX.Element {
  switch (state) {
    case "active":
      return <StateText color="green">可用</StateText>;
    case "cooling":
      return <StateText color="gold">冷却中</StateText>;
    default:
      return <StateText color="red">失效</StateText>;
  }
}

// splitKeysText 把多行文本框内容解析成 key 数组，并忽略空行与前后空格。

const stateColors: Record<string, string> = {
  green: "#16a34a",
  red: "#dc2626",
  gold: "#d97706",
  blue: "#2563eb",
  gray: "#64748b",
};

function StateText({ color, children }: { color: string; children: React.ReactNode }): JSX.Element {
  return <span style={{ color: stateColors[color] ?? "#475569", fontWeight: 600, fontSize: "0.82rem" }}>{children}</span>;
}
function splitKeysText(input: string): string[] {
  return input
    .split(/\r?\n/g)
    .map((key) => key.trim())
    .filter((key) => key.length > 0);
}
