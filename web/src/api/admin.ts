import type {
  AdminAboutResponse,
  AdminChangeResponse,
  AdminDashboardResponse,
  AdminDeleteKeysPayload,
  AdminEventsResponse,
  AdminKeysPayload,
  AdminProviderCreatePayload,
  AdminProviderDetailResponse,
  AdminProvidersResponse,
  AdminProviderUpdatePayload,
  AdminReloadResponse,
  AdminSettingsPayload,
  AdminSettingsResponse,
  AdminStatsModelsResponse,
  AdminStatsRecentResponse,
  AdminStatsSummaryResponse,
  AdminStatsWindow,
} from "../types/admin";
import { requestDownload, requestJSON, saveDownloadBlob } from "./http";

function pathSegment(value: string): string {
  return encodeURIComponent(value);
}

// fetchDashboard 拉取控制台首页聚合数据，并用于定时轮询刷新。
export function fetchDashboard(): Promise<AdminDashboardResponse> {
  return requestJSON<AdminDashboardResponse>("/admin/api/v1/dashboard");
}

// fetchProviders 拉取提供商列表与汇总状态，供只读列表页展示。
export function fetchProviders(): Promise<AdminProvidersResponse> {
  return requestJSON<AdminProvidersResponse>("/admin/api/v1/providers");
}

// fetchProviderDetail 拉取单个 provider 的 key 详情与运行状态。
export function fetchProviderDetail(providerID: string): Promise<AdminProviderDetailResponse> {
  return requestJSON<AdminProviderDetailResponse>(`/admin/api/v1/providers/${pathSegment(providerID)}`);
}

// createProvider 提交一个新 provider 到配置文件并触发 reload。
export function createProvider(payload: AdminProviderCreatePayload): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>("/admin/api/v1/providers", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

// updateProvider 更新 provider 的基础信息，当前阶段只编辑 target_url。
export function updateProvider(providerID: string, payload: AdminProviderUpdatePayload): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>(`/admin/api/v1/providers/${pathSegment(providerID)}`, {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

// deleteProvider 删除一个非活跃 provider。
export function deleteProvider(providerID: string): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>(`/admin/api/v1/providers/${pathSegment(providerID)}`, {
    method: "DELETE",
  });
}

// activateProvider 把某个 provider 切换为当前活跃目标。
export function activateProvider(providerID: string): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>(`/admin/api/v1/providers/${pathSegment(providerID)}/activate`, {
    method: "POST",
  });
}

// appendProviderKeys 追加 keys 到指定 provider。
export function appendProviderKeys(providerID: string, payload: AdminKeysPayload): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>(`/admin/api/v1/providers/${pathSegment(providerID)}/keys:append`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

// replaceProviderKeys 全量替换 provider 的 keys。
export function replaceProviderKeys(providerID: string, payload: AdminKeysPayload): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>(`/admin/api/v1/providers/${pathSegment(providerID)}/keys:replace`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

// deleteProviderKeys 按 key_id 删除 provider 中的 keys。
export function deleteProviderKeys(providerID: string, payload: AdminDeleteKeysPayload): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>(`/admin/api/v1/providers/${pathSegment(providerID)}/keys:delete`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

// resetProviderKey 手动把某个 key 恢复为 active。
export function resetProviderKey(providerID: string, keyID: string): Promise<AdminReloadResponse> {
  return requestJSON<AdminReloadResponse>(
    `/admin/api/v1/providers/${pathSegment(providerID)}/keys/${pathSegment(keyID)}/reset`,
    {
    method: "POST",
    },
  );
}

// fetchRecentEvents 拉取最近事件，供 dashboard 摘要与后续事件页复用。
export function fetchRecentEvents(limit = 10): Promise<AdminEventsResponse> {
  return requestJSON<AdminEventsResponse>(`/admin/api/v1/events?limit=${limit}`);
}

// triggerReload 手动触发一次后端 reload，适合作为 dashboard 快捷操作。
export function triggerReload(): Promise<AdminReloadResponse> {
  return requestJSON<AdminReloadResponse>("/admin/api/v1/reload", {
    method: "POST",
  });
}

// fetchSettings 拉取设置页所需的完整配置与字段生效分类。
export function fetchSettings(): Promise<AdminSettingsResponse> {
  return requestJSON<AdminSettingsResponse>("/admin/api/v1/settings");
}

// updateSettings 提交设置页表单，并返回本次变更的字段分类结果。
export function updateSettings(payload: AdminSettingsPayload): Promise<AdminChangeResponse> {
  return requestJSON<AdminChangeResponse>("/admin/api/v1/settings", {
    method: "PUT",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });
}

// fetchStatsSummary 拉取指定窗口内的调用统计 KPI。
export function fetchStatsSummary(window: AdminStatsWindow): Promise<AdminStatsSummaryResponse> {
  return requestJSON<AdminStatsSummaryResponse>(`/admin/api/v1/stats/summary?window=${window}`);
}

// fetchStatsModels 拉取指定窗口内按模型聚合的调用统计。
export function fetchStatsModels(window: AdminStatsWindow): Promise<AdminStatsModelsResponse> {
  return requestJSON<AdminStatsModelsResponse>(`/admin/api/v1/stats/models?window=${window}`);
}

// fetchStatsRecent 拉取最近调用明细。
export function fetchStatsRecent(limit = 100): Promise<AdminStatsRecentResponse> {
  return requestJSON<AdminStatsRecentResponse>(`/admin/api/v1/stats/recent?limit=${limit}`);
}

// fetchAbout 拉取关于页所需的运行时信息。
export function fetchAbout(): Promise<AdminAboutResponse> {
  return requestJSON<AdminAboutResponse>("/admin/api/v1/about");
}

// downloadConfigBackup 下载当前配置备份文件。
export async function downloadConfigBackup(): Promise<void> {
  const file = await requestDownload("/admin/api/v1/config/backup", {
    method: "POST",
  });
  saveDownloadBlob(file.blob, file.filename);
}

// downloadStateBackup 下载当前状态快照备份。
export async function downloadStateBackup(): Promise<void> {
  const file = await requestDownload("/admin/api/v1/state/backup", {
    method: "POST",
  });
  saveDownloadBlob(file.blob, file.filename);
}
