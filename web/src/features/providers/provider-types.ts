import type { AdminProviderSummary } from "../../types/admin";

export type ProviderFormMode = "create" | "edit";
export type KeyFormMode = "append" | "replace";

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

export type ProviderFormValues = {
  id: string;
  target_url: string;
  keys_text: string;
};

export type KeyFormValues = {
  keys_text: string;
};

export type ModelFormValues = {
  models_text: string;
};
