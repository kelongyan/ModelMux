import type { AdminStatsWindow } from "../../types/admin";

export const statsWindowOptions: Array<{ label: string; value: AdminStatsWindow }> = [
  { label: "1 小时", value: "1h" },
  { label: "24 小时", value: "24h" },
  { label: "7 天", value: "7d" },
  { label: "30 天", value: "30d" },
];

export const statsStatusOptions = [
  { label: "全部状态", value: "" },
  { label: "成功", value: "success" },
  { label: "失败", value: "failed" },
];
