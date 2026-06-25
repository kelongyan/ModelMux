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

// pickTimelineGranularity 根据时间窗口自动选择后端支持的粒度。
// 后端只支持 "1h" 和 "1d" 两种粒度。
export function pickTimelineGranularity(window: AdminStatsWindow): "1h" | "1d" {
  return window === "7d" || window === "30d" ? "1d" : "1h";
}

// formatTimelineTime 根据时间窗口格式化 X 轴时间标签。
export function formatTimelineTime(timeStr: string, window: AdminStatsWindow): string {
  const d = new Date(timeStr);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const MM = String(d.getMonth() + 1).padStart(2, "0");
  const DD = String(d.getDate()).padStart(2, "0");

  switch (window) {
    case "1h":
      return `${hh}:${mm}`;
    case "24h":
      return `${hh}:${mm}`;
    case "7d":
      return `${MM}-${DD}`;
    case "30d":
      return `${MM}-${DD}`;
    default:
      return `${hh}:${mm}`;
  }
}
