import type { AdminSettingsPayload } from "../../types/admin";

export type SaveSummary = {
  changedFields: string[];
  hotReloadedFields: string[];
  restartRequiredFields: string[];
};

export type SettingFieldMeta = {
  name: keyof AdminSettingsPayload;
  label: string;
  hint: string;
  effect: "hot" | "restart" | "readonly";
  render: () => JSX.Element;
};
