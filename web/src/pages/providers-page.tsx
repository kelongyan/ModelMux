import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Empty, Form, Modal, Result, Skeleton, Space, Typography, message } from "antd";
import { useEffect, useRef, useState } from "react";
import { useSearchParams } from "react-router-dom";

import {
  activateProvider,
  appendProviderKeys,
  createProvider,
  deleteProvider,
  deleteProviderKeys,
  downloadProvidersExport,
  fetchProviderDetail,
  fetchProviderModels,
  fetchProviders,
  importProviders,
  previewProviderKeys,
  replaceProviderKeys,
  replaceProviderModels,
  resetProviderKey,
  resetAllProviderKeys,
  testAllProviderKeys,
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
  KeyTestAllResultModal,
  ModelEditorModal,
  ModelSyncModal,
  ProviderEditorModal,
} from "../features/providers/provider-modals";
import { ProviderTable } from "../features/providers/provider-table";
import { buildModelSaveList, type ModelSaveMode } from "../features/providers/model-sync";
import type {
  KeyFormMode,
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
} from "../features/providers/provider-types";
import { splitLinesText } from "../features/providers/provider-utils";
import type { AdminKeyMetadataPayload, AdminKeyStatus, AdminKeyTestAllResult, AdminProviderSummary } from "../types/admin";

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
  const [modelSyncModal, setModelSyncModal] = useState<ModelSyncModalState>({
    open: false,
    providerID: null,
    currentModels: [],
    fetchedModels: [],
  });
  const [selectedSyncModelIDs, setSelectedSyncModelIDs] = useState<string[]>([]);
  const [modelSyncSearch, setModelSyncSearch] = useState("");
  const importFileRef = useRef<HTMLInputElement>(null);
  const [testingKeyID, setTestingKeyID] = useState<string | null>(null);
  const [testAllModal, setTestAllModal] = useState<{ open: boolean; results: AdminKeyTestAllResult[]; allOK: boolean }>({
    open: false,
    results: [],
    allOK: false,
  });

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
    setKeyMetadataModal({ open: false });
    setKeyPreviewModal({
      open: false,
      providerID: null,
      mode: "append",
      preview: null,
      keys: [],
    });
    setModelSyncModal({
      open: false,
      providerID: null,
      currentModels: [],
      fetchedModels: [],
    });
    setSelectedSyncModelIDs([]);
    setModelSyncSearch("");
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
        closeDetailModal();
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

  const testAllKeysMutation = useMutation({
    mutationFn: testAllProviderKeys,
    onSuccess: (data) => {
      setTestAllModal({ open: true, results: data.results, allOK: data.ok });
    },
    onError: (error: Error) => messageApi.error(`批量测试失败：${error.message}`),
  });

  const replaceModelsMutation = useMutation({
    mutationFn: async (payload: { providerID: string; models: string[] }) =>
      replaceProviderModels(payload.providerID, { models: payload.models }),
    onSuccess: async (_, variables) => {
      messageApi.success(`已更新模型记录（${variables.models.length} 个）`);
      setModelModal({ open: false });
      closeModelSyncModal();
      modelForm.resetFields();
      await invalidateAdminQueries(variables.providerID);
    },
    onError: (error: Error) => messageApi.error(`更新模型失败：${error.message}`),
  });

  const fetchModelsMutation = useMutation({
    mutationFn: async (providerID: string) => fetchProviderModels(providerID),
    onSuccess: async (data, providerID) => {
      if (selectedProviderID !== providerID) {
        return;
      }
      const currentModels = providerDetailQuery.data?.id === providerID ? providerDetailQuery.data.models : [];
      if (data.count === 0) {
        messageApi.warning("上游未返回模型");
      } else {
        messageApi.success(`从上游拉取到 ${data.count} 个模型`);
      }
      setSelectedSyncModelIDs(data.models);
      setModelSyncSearch("");
      setModelSyncModal({
        open: true,
        providerID,
        currentModels,
        fetchedModels: data.models,
      });
      await invalidateAdminQueries(providerID);
    },
    onError: (error: Error) => messageApi.error(`拉取模型失败：${error.message}`),
  });

  const exportMutation = useMutation({
    mutationFn: downloadProvidersExport,
    onSuccess: () => messageApi.success("导出完成"),
    onError: (error: Error) => messageApi.error(`导出失败：${error.message}`),
  });

  const importMutation = useMutation({
    mutationFn: importProviders,
    onSuccess: async (result) => {
      const parts: string[] = [];
      if (result.imported > 0) {
        parts.push(`导入 ${result.imported} 个`);
      }
      if (result.skipped_ids && result.skipped_ids.length > 0) {
        parts.push(`跳过 ${result.skipped_ids.length} 个已存在`);
      }
      messageApi.success(parts.length > 0 ? parts.join("，") : "导入完成");
      if (importFileRef.current) {
        importFileRef.current.value = "";
      }
      await invalidateAdminQueries();
    },
    onError: (error: Error) => {
      if (importFileRef.current) {
        importFileRef.current.value = "";
      }
      messageApi.error(`导入失败：${error.message}`);
    },
  });

  const selectedProvider = providersQuery.data?.providers.find((provider) => provider.id === selectedProviderID);
  const providerDetail = providerDetailQuery.data;
  const detailLoading = providerDetailQuery.isLoading && selectedProviderID !== null;

  if (providersQuery.isLoading) {
    return (
      <div className="console-loading">
        <Skeleton active paragraph={{ rows: 8 }} />
      </div>
    );
  }

  if (providersQuery.isError || !providersQuery.data) {
    return (
      <Result
        status="error"
        title="Provider 列表加载失败"
        subTitle={providersQuery.error instanceof Error ? providersQuery.error.message : "未知错误"}
        extra={<Button onClick={() => void providersQuery.refetch()}>重试</Button>}
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
              <Button loading={exportMutation.isPending} onClick={() => exportMutation.mutate()}>
                导出
              </Button>
              <Button
                loading={importMutation.isPending}
                onClick={() => importFileRef.current?.click()}
              >
                导入
              </Button>
              <input
                ref={importFileRef}
                type="file"
                accept=".json,application/json"
                hidden
                onChange={(event) => {
                  const file = event.target.files?.[0];
                  if (file) {
                    importMutation.mutate(file);
                  }
                }}
              />
              <Button type="primary" onClick={openProviderCreate}>
                新增 Provider
              </Button>
            </Space>
          </div>
          <ProviderTable
            providers={providersQuery.data?.providers ?? []}
            activating={activateProviderMutation.isPending}
            deleting={deleteProviderMutation.isPending}
            onOpenDetail={openDetailModal}
            onActivate={(providerID) => activateProviderMutation.mutate(providerID)}
            onEdit={openProviderEdit}
            onDelete={(providerID) => deleteProviderMutation.mutate(providerID)}
          />
        </Card>
      </Space>

      {/* Provider 详情弹出层 */}
      <Modal
        open={selectedProviderID !== null}
        title={selectedProvider ? `Provider 详情：${selectedProvider.id}` : "Provider 详情"}
        onCancel={closeDetailModal}
        footer={null}
        width="min(1100px, 92vw)"
        destroyOnHidden
        className="provider-detail-modal"
      >
        {detailLoading ? (
          <div className="console-loading">
            <Skeleton active paragraph={{ rows: 6 }} />
          </div>
        ) : providerDetailQuery.isError ? (
          <Result
            status="error"
            title="Provider 详情加载失败"
            subTitle={providerDetailQuery.error instanceof Error ? providerDetailQuery.error.message : "未知错误"}
            extra={<Button onClick={() => void providerDetailQuery.refetch()}>重试</Button>}
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
                metadata: { disabled },
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
            onTestAllKeys={() => {
              if (selectedProviderID) {
                testAllKeysMutation.mutate(selectedProviderID);
              }
            }}
            testingAllKeys={testAllKeysMutation.isPending}
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
      </Modal>

      {renderModals()}

      <KeyTestAllResultModal
        open={testAllModal.open}
        results={testAllModal.results}
        allOK={testAllModal.allOK}
        onClose={() => setTestAllModal({ open: false, results: [], allOK: false })}
      />
    </>
  );

  function openDetailModal(providerID: string) {
    setSelectedProviderID(providerID);
    const next = new URLSearchParams(searchParams);
    next.set("provider", providerID);
    setSearchParams(next, { replace: true });
  }

  function closeDetailModal() {
    setSelectedProviderID(null);
    const next = new URLSearchParams(searchParams);
    next.delete("provider");
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

  function closeModelSyncModal() {
    setModelSyncModal({
      open: false,
      providerID: null,
      currentModels: [],
      fetchedModels: [],
    });
    setSelectedSyncModelIDs([]);
    setModelSyncSearch("");
  }

  function openManualModelEditorFromSync() {
    const models = selectedSyncModelIDs.length > 0 ? selectedSyncModelIDs : modelSyncModal.fetchedModels;
    modelForm.setFieldsValue({ models_text: models.join("\n") });
    setModelModal({ open: true });
    setModelSyncModal((current) => ({ ...current, open: false }));
  }

  async function submitModelForm(values: ModelFormValues) {
    if (!selectedProviderID) {
      messageApi.error("请先选择 provider");
      return;
    }
    await replaceModelsMutation.mutateAsync({ providerID: selectedProviderID, models: splitLinesText(values.models_text) });
  }

  async function submitModelSync(mode: ModelSaveMode) {
    if (!modelSyncModal.providerID) {
      messageApi.error("缺少 provider id");
      return;
    }
    const models = buildModelSaveList({
      mode,
      currentModels: modelSyncModal.currentModels,
      selectedModels: selectedSyncModelIDs,
    });
    if (models.length === 0) {
      messageApi.warning("请至少选择一个模型");
      return;
    }
    await replaceModelsMutation.mutateAsync({ providerID: modelSyncModal.providerID, models });
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

  function renderModals() {
    return (
      <>
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
        <ModelSyncModal
          state={modelSyncModal}
          selectedModelIDs={selectedSyncModelIDs}
          searchValue={modelSyncSearch}
          confirmLoading={replaceModelsMutation.isPending}
          onCancel={closeModelSyncModal}
          onSearchChange={setModelSyncSearch}
          onSelectedModelIDsChange={setSelectedSyncModelIDs}
          onSave={(mode) => void submitModelSync(mode)}
          onOpenManualEdit={openManualModelEditorFromSync}
        />
      </>
    );
  }
}
