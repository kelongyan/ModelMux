import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Drawer, Empty, Form, Result, Space, Spin, Typography, message } from "antd";
import { useEffect, useState } from "react";
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
  previewProviderKeys,
  replaceProviderKeys,
  replaceProviderModels,
  resetProviderKey,
  resetAllProviderKeys,
  testProviderKey,
  updateProvider,
  updateProviderKeyMetadata,
} from "../api/admin";
import { queryKeys } from "../api/query-keys";
import { ProviderDetailContent } from "../features/providers/provider-detail-content";
import {
  KeyEditorModal,
  KeyMetadataModal,
  KeyPreviewModal,
  ModelEditorModal,
  ProviderEditorModal,
} from "../features/providers/provider-modals";
import { ProviderTable } from "../features/providers/provider-table";
import type {
  KeyFormMode,
  KeyFormValues,
  KeyModalState,
  KeyMetadataFormValues,
  KeyMetadataModalState,
  KeyPreviewModalState,
  ModelFormValues,
  ModelModalState,
  ProviderFormValues,
  ProviderModalState,
} from "../features/providers/provider-types";
import { splitLinesText } from "../features/providers/provider-utils";
import type { AdminKeyMetadataPayload, AdminKeyStatus, AdminProviderSummary } from "../types/admin";

export function ProvidersPage(): JSX.Element {
  const queryClient = useQueryClient();
  const [messageApi, contextHolder] = message.useMessage();
  const [searchParams, setSearchParams] = useSearchParams();
  const [selectedProviderID, setSelectedProviderID] = useState<string | null>(() => searchParams.get("provider"));
  const [selectedKeyIDs, setSelectedKeyIDs] = useState<string[]>([]);
  const [providerModal, setProviderModal] = useState<ProviderModalState>({ open: false, mode: "create" });
  const [keyModal, setKeyModal] = useState<KeyModalState>({ open: false, mode: "append" });
  const [keyMetadataModal, setKeyMetadataModal] = useState<KeyMetadataModalState>({ open: false });
  const [keyPreviewModal, setKeyPreviewModal] = useState<KeyPreviewModalState>({
    open: false,
    providerID: null,
    mode: "append",
    preview: null,
    keys: [],
  });
  const [modelModal, setModelModal] = useState<ModelModalState>({ open: false });
  const [testingKeyID, setTestingKeyID] = useState<string | null>(null);

  const [providerForm] = Form.useForm<ProviderFormValues>();
  const [keyForm] = Form.useForm<KeyFormValues>();
  const [keyMetadataForm] = Form.useForm<KeyMetadataFormValues>();
  const [modelForm] = Form.useForm<ModelFormValues>();

  const providersQuery = useQuery({
    queryKey: queryKeys.providers,
    queryFn: fetchProviders,
    refetchInterval: 8000,
  });

  const providerDetailQuery = useQuery({
    queryKey: queryKeys.providerDetail(selectedProviderID),
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

  useEffect(() => {
    const fromUrl = searchParams.get("provider");
    if (fromUrl !== selectedProviderID) {
      setSelectedProviderID(fromUrl);
    }
  }, [searchParams, selectedProviderID]);

  useEffect(() => {
    setKeyMetadataModal({ open: false });
    setKeyPreviewModal({
      open: false,
      providerID: null,
      mode: "append",
      preview: null,
      keys: [],
    });
  }, [selectedProviderID]);

  const invalidateAdminQueries = async (providerID?: string) => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: queryKeys.dashboard }),
      queryClient.invalidateQueries({ queryKey: queryKeys.providers }),
      queryClient.invalidateQueries({ queryKey: queryKeys.events(200) }),
      providerID
        ? queryClient.invalidateQueries({ queryKey: queryKeys.providerDetail(providerID) })
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
    onError: (error: Error) => messageApi.error(`新增失败：${error.message}`),
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
    onError: (error: Error) => messageApi.error(`更新失败：${error.message}`),
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
    onError: (error: Error) => messageApi.error(`删除失败：${error.message}`),
  });

  const activateProviderMutation = useMutation({
    mutationFn: activateProvider,
    onSuccess: async (_, providerID) => {
      messageApi.success(`已切换到 ${providerID}`);
      await invalidateAdminQueries(providerID);
    },
    onError: (error: Error) => messageApi.error(`切换失败：${error.message}`),
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
    onError: (error: Error) => messageApi.error(`追加失败：${error.message}`),
  });

  const previewKeysMutation = useMutation({
    mutationFn: async (payload: { providerID: string; mode: KeyFormMode; keys: string[] }) =>
      previewProviderKeys(payload.providerID, { mode: payload.mode, keys: payload.keys }),
    onSuccess: async (preview, variables) => {
      setKeyPreviewModal({
        open: true,
        providerID: variables.providerID,
        mode: variables.mode,
        preview,
        keys: variables.keys,
      });
    },
    onError: (error: Error) => messageApi.error(`预览失败：${error.message}`),
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
    onError: (error: Error) => messageApi.error(`替换失败：${error.message}`),
  });

  const updateKeyMetadataMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keyID: string; metadata: AdminKeyMetadataPayload }) =>
      updateProviderKeyMetadata(payload.providerID, payload.keyID, payload.metadata),
    onSuccess: async (_, variables) => {
      const message =
        variables.metadata.disabled === true
          ? "已停用 key"
          : variables.metadata.label === undefined && variables.metadata.note === undefined && variables.metadata.disabled === false
            ? "已启用 key"
            : "已更新 key 元数据";
      messageApi.success(message);
      setKeyMetadataModal({ open: false });
      keyMetadataForm.resetFields();
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => messageApi.error(`更新 key 元数据失败：${error.message}`),
  });

  const deleteKeysMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keyIDs: string[] }) =>
      deleteProviderKeys(payload.providerID, { key_ids: payload.keyIDs }),
    onSuccess: async (_, variables) => {
      messageApi.success("已删除选中 keys");
      setSelectedKeyIDs([]);
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => messageApi.error(`删除 keys 失败：${error.message}`),
  });

  const resetKeyMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keyID: string }) =>
      resetProviderKey(payload.providerID, payload.keyID),
    onSuccess: async (_, variables) => {
      messageApi.success("已重置 key 状态");
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => messageApi.error(`重置失败：${error.message}`),
  });

  const resetAllKeysMutation = useMutation({
    mutationFn: resetAllProviderKeys,
    onSuccess: async (_, providerID) => {
      messageApi.success("已重置全部 key 状态");
      await invalidateAdminQueries(providerID);
    },
    onError: (error: Error) => messageApi.error(`重置全部失败：${error.message}`),
  });

  const testKeyMutation = useMutation({
    mutationFn: async (payload: { providerID: string; keyID: string }) =>
      testProviderKey(payload.providerID, payload.keyID),
    onMutate: (variables) => {
      setTestingKeyID(variables.keyID);
    },
    onSuccess: (result) => {
      if (result.ok) {
        messageApi.success(`Key 测试通过（${result.status_code}）`);
        return;
      }
      const suffix = result.error ? `：${result.error}` : "";
      messageApi.warning(`Key 测试失败（${result.status_code || "无响应"}${suffix}）`);
    },
    onError: (error: Error) => messageApi.error(`测试失败：${error.message}`),
    onSettled: () => {
      setTestingKeyID(null);
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
    onError: (error: Error) => messageApi.error(`更新模型失败：${error.message}`),
  });

  const fetchModelsMutation = useMutation({
    mutationFn: async (providerID: string) => fetchProviderModels(providerID),
    onSuccess: async (data, providerID) => {
      messageApi.success(`从上游拉取到 ${data.count} 个模型`);
      modelForm.setFieldsValue({ models_text: data.models.join("\n") });
      setModelModal({ open: true });
      await invalidateAdminQueries(providerID);
    },
    onError: (error: Error) => messageApi.error(`拉取模型失败：${error.message}`),
  });

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

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={16} className="console-stack">
        <Card className="surface-card reveal-card reveal-delay-0" bordered={false}>
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
          <ProviderTable
            providers={providersQuery.data.providers}
            activating={activateProviderMutation.isPending}
            deleting={deleteProviderMutation.isPending}
            onOpenDetail={setSelectedProvider}
            onActivate={(providerID) => activateProviderMutation.mutate(providerID)}
            onEdit={openProviderEdit}
            onDelete={(providerID) => deleteProviderMutation.mutate(providerID)}
          />
        </Card>
      </Space>

      <ProviderEditorModal
        state={providerModal}
        form={providerForm}
        confirmLoading={createProviderMutation.isPending || updateProviderMutation.isPending}
        onCancel={closeProviderModal}
        onSubmit={(values) => void submitProviderForm(values)}
      />
      <KeyEditorModal
        state={keyModal}
        form={keyForm}
        confirmLoading={previewKeysMutation.isPending}
        onCancel={closeKeyModal}
        onSubmit={(values) => void submitKeyForm(values)}
      />
      <KeyMetadataModal
        state={keyMetadataModal}
        form={keyMetadataForm}
        confirmLoading={updateKeyMetadataMutation.isPending}
        onCancel={closeKeyMetadataModal}
        onSubmit={(values) => void submitKeyMetadataForm(values)}
      />
      <KeyPreviewModal
        state={keyPreviewModal}
        confirmLoading={appendKeysMutation.isPending || replaceKeysMutation.isPending}
        onCancel={closeKeyPreviewModal}
        onConfirm={() => void confirmKeyPreview()}
      />
      <ModelEditorModal
        state={modelModal}
        selectedProviderID={selectedProviderID}
        form={modelForm}
        confirmLoading={replaceModelsMutation.isPending}
        onCancel={closeModelModal}
        onSubmit={(values) => void submitModelForm(values)}
      />

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
            onSelectedKeyIDsChange={setSelectedKeyIDs}
            onResetKey={(keyID) => {
              if (selectedProviderID) {
                resetKeyMutation.mutate({ providerID: selectedProviderID, keyID });
              }
            }}
            resettingKey={resetKeyMutation.isPending}
            onResetAllKeys={() => {
              if (selectedProviderID) {
                resetAllKeysMutation.mutate(selectedProviderID);
              }
            }}
            resettingAllKeys={resetAllKeysMutation.isPending}
            onEditKeyMetadata={openKeyMetadataModal}
            onToggleKeyDisabled={(key, disabled) => {
              if (!selectedProviderID) {
                return;
              }
              updateKeyMetadataMutation.mutate({
                providerID: selectedProviderID,
                keyID: key.key_id,
                metadata: {
                  disabled,
                },
              });
            }}
            updatingKeyMetadata={updateKeyMetadataMutation.isPending}
            onTestKey={(keyID) => {
              if (!selectedProviderID) {
                return;
              }
              testKeyMutation.mutate({ providerID: selectedProviderID, keyID });
            }}
            testingKeyID={testingKeyID}
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

  function setSelectedProvider(providerID: string | null) {
    setSelectedProviderID(providerID);
    const next = new URLSearchParams(searchParams);
    if (providerID) {
      next.set("provider", providerID);
    } else {
      next.delete("provider");
    }
    setSearchParams(next, { replace: true });
  }

  function openProviderCreate() {
    providerForm.setFieldsValue({ id: "", target_url: "", keys_text: "" });
    setProviderModal({ open: true, mode: "create" });
  }

  function openProviderEdit(provider: AdminProviderSummary) {
    providerForm.setFieldsValue({ id: provider.id, target_url: provider.target_url, keys_text: "" });
    setProviderModal({ open: true, mode: "edit", provider });
  }

  function closeProviderModal() {
    setProviderModal({ open: false, mode: "create" });
    providerForm.resetFields();
  }

  function openKeyModal(mode: KeyFormMode) {
    keyForm.setFieldsValue({ keys_text: "" });
    setKeyModal({ open: true, mode });
    closeKeyPreviewModal();
  }

  function closeKeyModal() {
    setKeyModal({ open: false, mode: "append" });
    keyForm.resetFields();
    closeKeyPreviewModal();
  }

  function openKeyMetadataModal(key: AdminKeyStatus) {
    keyMetadataForm.setFieldsValue({
      label: key.label ?? "",
      note: key.note ?? "",
      disabled: key.disabled ?? key.state === "disabled",
    });
    setKeyMetadataModal({ open: true, key });
  }

  function closeKeyMetadataModal() {
    setKeyMetadataModal({ open: false });
    keyMetadataForm.resetFields();
  }

  function closeKeyPreviewModal() {
    setKeyPreviewModal({
      open: false,
      providerID: null,
      mode: "append",
      preview: null,
      keys: [],
    });
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
    await replaceModelsMutation.mutateAsync({ providerID: selectedProviderID, models: splitLinesText(values.models_text) });
  }

  async function submitProviderForm(values: ProviderFormValues) {
    const keys = splitLinesText(values.keys_text);
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
    await updateProviderMutation.mutateAsync({ id: providerID, target_url: values.target_url.trim() });
  }

  async function submitKeyForm(values: KeyFormValues) {
    if (!selectedProviderID) {
      messageApi.error("请先选择 provider");
      return;
    }
    const keys = splitLinesText(values.keys_text);
    await previewKeysMutation.mutateAsync({ providerID: selectedProviderID, mode: keyModal.mode, keys });
  }

  async function confirmKeyPreview() {
    if (!keyPreviewModal.providerID || !keyPreviewModal.preview) {
      messageApi.error("预览结果不存在");
      return;
    }
    const payload = {
      providerID: keyPreviewModal.providerID,
      keys: keyPreviewModal.keys,
    };
    if (keyPreviewModal.mode === "append") {
      await appendKeysMutation.mutateAsync(payload);
    } else {
      await replaceKeysMutation.mutateAsync(payload);
    }
    closeKeyPreviewModal();
  }

  async function submitKeyMetadataForm(values: KeyMetadataFormValues) {
    const key = keyMetadataModal.key;
    if (!selectedProviderID || !key) {
      messageApi.error("请先选择 key");
      return;
    }
    await updateKeyMetadataMutation.mutateAsync({
      providerID: selectedProviderID,
      keyID: key.key_id,
      metadata: {
        label: (values.label ?? "").trim(),
        note: (values.note ?? "").trim(),
        disabled: values.disabled ?? false,
      },
    });
  }
}
