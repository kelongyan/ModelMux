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
  fetchProviders,
  replaceProviderKeys,
  resetProviderKey,
  updateProvider,
} from "../api/admin";
import { CooldownText } from "../components/cooldown-text";
import { formatRelativeTime } from "../components/format-time";
import { KeyPoolDots } from "../components/key-pool-dots";
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

  const [providerForm] = Form.useForm<ProviderFormValues>();
  const [keyForm] = Form.useForm<KeyFormValues>();

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

  const providerColumns = useMemo<TableColumnsType<AdminProviderSummary>>(
    () => [
      {
        title: "Provider",
        dataIndex: "id",
        key: "id",
        render: (_: string, record) => (
          <div className="provider-table-name">
            <div className="provider-table-title-row">
              <strong>{record.id}</strong>
              {record.active ? <Tag color="green">当前活跃</Tag> : <Tag>待命</Tag>}
            </div>
            <div className="table-subtext">{record.target_url}</div>
            <div className="provider-table-pool">
              <KeyPoolDots
                active={record.active_keys}
                cooling={record.cooling_keys}
                invalid={record.invalid_keys}
                max={18}
                size="small"
              />
              <span>{`可用 ${record.active_keys} · 冷却 ${record.cooling_keys} · 失效 ${record.invalid_keys}`}</span>
            </div>
          </div>
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
          <Space wrap>
            <Button size="small" onClick={() => setSelectedProvider(record.id)}>
              查看详情
            </Button>
            {!record.active ? (
              <Button
                size="small"
                type="primary"
                loading={activateProviderMutation.isPending}
                onClick={() => activateProviderMutation.mutate(record.id)}
              >
                设为活跃
              </Button>
            ) : null}
            <Button size="small" onClick={() => openProviderEdit(record)}>
              编辑
            </Button>
            <Popconfirm
              title={`确认删除 provider ${record.id}？`}
              description="删除后将同时移除其全部 keys。"
              okText="删除"
              cancelText="取消"
              onConfirm={() => deleteProviderMutation.mutate(record.id)}
            >
              <Button danger size="small" loading={deleteProviderMutation.isPending}>
                删除
              </Button>
            </Popconfirm>
          </Space>
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
  const providerSummary = summarizeProviders(providers);

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={20} className="console-stack">
        <Card className="surface-card" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Providers</Typography.Text>
              <Typography.Title level={3} className="section-title">
                Provider 列表
              </Typography.Title>
              <Typography.Paragraph className="dashboard-section-copy">
                用表格集中管理 provider 与 key 池状态，详情放到抽屉中处理，避免首页过度分散。
              </Typography.Paragraph>
            </div>
            <Space wrap>
              <Button onClick={() => void providersQuery.refetch()}>刷新</Button>
              <Button type="primary" onClick={openProviderCreate}>
                新增 Provider
              </Button>
            </Space>
          </div>
          <div className="providers-summary-strip">
            <span className="summary-pill">{`当前活跃：${providersQuery.data.active_provider}`}</span>
            <span className="summary-pill">{`Provider ${providers.length}`}</span>
            <span className="summary-pill">{`总 Key ${providerSummary.totalKeys}`}</span>
            <span className="summary-pill">{`可用 ${providerSummary.activeKeys}`}</span>
            <span className="summary-pill">{`冷却 ${providerSummary.coolingKeys}`}</span>
            <span className="summary-pill">{`失效 ${providerSummary.invalidKeys}`}</span>
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
}: ProviderDetailContentProps): JSX.Element {
  return (
    <Space direction="vertical" size={20} className="console-stack">
      <Card className="surface-card" bordered={false}>
        <div className="provider-detail-pool">
          <KeyPoolDots
            active={detail.active_keys}
            cooling={detail.cooling_keys}
            invalid={detail.invalid_keys}
            max={48}
          />
          <span className="provider-detail-pool-summary">
            {`可用 ${detail.active_keys} · 冷却 ${detail.cooling_keys} · 失效 ${detail.invalid_keys} · 总 ${detail.total_keys}`}
          </span>
        </div>
        <Descriptions
          className="provider-detail-descriptions"
          column={{ xs: 1, md: 2 }}
          items={[
            { key: "id", label: "Provider ID", children: detail.id },
            {
              key: "active",
              label: "状态",
              children: detail.active ? <Tag color="green">当前活跃</Tag> : <Tag>待命</Tag>,
            },
            { key: "target", label: "Target URL", children: detail.target_url },
            { key: "total", label: "总 Key", children: detail.total_keys },
            { key: "active_keys", label: "可用 Key", children: detail.active_keys },
            { key: "cooling_keys", label: "冷却 Key", children: detail.cooling_keys },
            { key: "invalid_keys", label: "失效 Key", children: detail.invalid_keys },
          ]}
        />
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
            <Button onClick={onOpenAppendKeys}>追加 Keys</Button>
            <Button danger onClick={onOpenReplaceKeys}>
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
              <Button danger disabled={selectedKeyIDs.length === 0} loading={deletingKeys}>
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
    return <Tag color="green">当前活跃</Tag>;
  }
  if (provider.active_keys === 0 && provider.cooling_keys === 0) {
    return <Tag color="red">不可用</Tag>;
  }
  if (provider.cooling_keys > 0 || provider.invalid_keys > 0) {
    return <Tag color="gold">波动中</Tag>;
  }
  return <Tag color="blue">待命</Tag>;
}

function renderKeyState(state: AdminKeyStatus["state"]): JSX.Element {
  switch (state) {
    case "active":
      return <Tag color="green">可用</Tag>;
    case "cooling":
      return <Tag color="gold">冷却中</Tag>;
    default:
      return <Tag color="red">失效</Tag>;
  }
}

// splitKeysText 把多行文本框内容解析成 key 数组，并忽略空行与前后空格。
function splitKeysText(input: string): string[] {
  return input
    .split(/\r?\n/g)
    .map((key) => key.trim())
    .filter((key) => key.length > 0);
}

function summarizeProviders(providers: AdminProviderSummary[]) {
  return providers.reduce(
    (summary, provider) => ({
      totalKeys: summary.totalKeys + provider.total_keys,
      activeKeys: summary.activeKeys + provider.active_keys,
      coolingKeys: summary.coolingKeys + provider.cooling_keys,
      invalidKeys: summary.invalidKeys + provider.invalid_keys,
    }),
    {
      totalKeys: 0,
      activeKeys: 0,
      coolingKeys: 0,
      invalidKeys: 0,
    }
  );
}
