import type { AdminKeyStatus, AdminKeysPreviewResponse, AdminProviderSummary } from "../../types/admin";

export type ProviderFormMode = "create" | "edit";
export type KeyFormMode = "append" | "replace";

export type KeyMetadataModalState = {
  open: boolean;
  key?: AdminKeyStatus;
};

export type KeyPreviewModalState = {
  open: boolean;
  providerID: string | null;
  mode: KeyFormMode;
  preview: AdminKeysPreviewResponse | null;
  keys: string[];
};

export type ProviderModalState = {
  open: boolean;
  mode: ProviderFormMode;
  provider?: AdminProviderSummary;
};

export type KeyModalState = {
  open: boolean;
  mode: KeyFormMode;
};

export type ModelModalState = {
  open: boolean;
};

export type ModelSyncModalState = {
  open: boolean;
  providerID: string | null;
  currentModels: string[];
  fetchedModels: string[];
};

export type ProviderFormValues = {
  id: string;
  target_url: string;
  keys_text: string;
  protocol: string;
  strip_tools: boolean;
};

export type KeyFormValues = {
  keys_text: string;
};

export type KeyMetadataFormValues = {
  label: string;
  note: string;
  disabled: boolean;
};

export type ModelFormValues = {
  models_text: string;
};
