import type { AdminCallRecord } from "../../types/admin";

// exportStatsToCSV 将调用日志导出为 CSV 文件并触发下载。
export function exportStatsToCSV(records: AdminCallRecord[], filename?: string): void {
  if (records.length === 0) {
    return;
  }

  const headers = [
    "时间",
    "Provider",
    "模型",
    "端点",
    "方法",
    "状态码",
    "成功",
    "延迟(ms)",
    "尝试次数",
    "输入Token",
    "输出Token",
    "总Token",
    "错误",
  ];

  const rows = records.map((record) => [
    record.at,
    record.provider_id,
    record.model ?? "",
    record.endpoint,
    record.method,
    String(record.status),
    record.success ? "是" : "否",
    String(record.latency_ms),
    String(record.attempts),
    record.prompt_tokens != null ? String(record.prompt_tokens) : "",
    record.completion_tokens != null ? String(record.completion_tokens) : "",
    record.total_tokens != null ? String(record.total_tokens) : "",
    record.error ?? "",
  ]);

  const csvContent = [
    headers.join(","),
    ...rows.map((row) =>
      row
        .map((cell) => {
          // 如果包含逗号、引号或换行，用引号包裹
          if (cell.includes(",") || cell.includes('"') || cell.includes("\n")) {
            return `"${cell.replace(/"/g, '""')}"`;
          }
          return cell;
        })
        .join(","),
    ),
  ].join("\n");

  // 添加 BOM 头以支持中文在 Excel 中正确显示
  const bom = "﻿";
  const blob = new Blob([bom + csvContent], { type: "text/csv;charset=utf-8;" });

  const url = window.URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename ?? `modelmux-stats-${new Date().toISOString().slice(0, 10)}.csv`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  window.URL.revokeObjectURL(url);
}
