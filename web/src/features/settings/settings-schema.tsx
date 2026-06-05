import { Checkbox, Input, InputNumber, Select } from "antd";

import type { AdminChangeResponse, AdminSettingsPayload, AdminSettingsResponse } from "../../types/admin";
import type { SaveSummary, SettingFieldMeta } from "./settings-types";

const fieldLabels: Record<string, string> = {
  listen: "代理监听地址",
  admin_listen: "管理监听地址",
  active_provider: "当前活跃 Provider",
  cooling_seconds: "429 冷却秒数",
  max_retries: "最大重试次数",
  max_transient_retries: "临时故障最大重试次数",
  request_timeout_seconds: "上游请求超时",
  connect_timeout_seconds: "连接超时",
  response_header_timeout_seconds: "响应头超时",
  transient_cooling_seconds: "临时故障冷却秒数",
  wait_for_key_timeout_ms: "等待 cooling Key 恢复时长",
  max_body_bytes: "请求体大小上限",
  log_level: "日志级别",
  log_format: "日志格式",
  log_output: "日志输出",
  log_file: "日志文件路径",
  log_max_size_mb: "日志单文件大小",
  log_max_backups: "日志保留数量",
  log_max_age_days: "日志保留天数",
  log_compress: "日志压缩",
  persist_state: "状态持久化",
  state_file: "状态文件路径",
  invalid_ttl_hours: "失效 Key 保留时长",
  stats_enabled: "调用统计",
  stats_dir: "统计文件目录",
  stats_retention_days: "统计保留天数",
  stats_max_recent_records: "最近记录内存上限",
};

export function buildSettingGroups(response: AdminSettingsResponse) {
  const hotSet = new Set(response.hot_reload_fields);
  const restartSet = new Set(response.restart_required_fields);

  const effectOf = (field: keyof AdminSettingsPayload): SettingFieldMeta["effect"] => {
    if (field === "active_provider") return "readonly";
    if (hotSet.has(field)) return "hot";
    if (restartSet.has(field)) return "restart";
    return "readonly";
  };

  const mkField = (name: keyof AdminSettingsPayload, label: string, hint: string, render: () => JSX.Element): SettingFieldMeta => ({
    name, label, hint, effect: effectOf(name), render,
  });

  return {
    coreFields: [
      mkField("active_provider", "当前活跃 Provider", "在总览或提供商页面切换 active provider，这里仅做只读展示。", () => <Input disabled />),
      mkField("cooling_seconds", "429 冷却秒数", "当上游未返回 Retry-After 时，ModelMux 会使用这个秒数作为默认冷却时间。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("max_retries", "最大重试次数", "401、429 和配额不足类 403 会在当前 provider 内按 key 轮换重试。", () => <InputNumber min={0} step={1} className="full-width" />),
      mkField("request_timeout_seconds", "上游请求超时（秒）", "控制上游 HTTP client 的请求超时时间。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("max_body_bytes", "请求体大小上限（字节）", "超过该值的请求会在代理入口被直接拒绝。", () => <InputNumber min={1024} step={1024} className="full-width" />),
    ] as SettingFieldMeta[],
    advancedFields: [
      mkField("max_transient_retries", "临时故障最大重试次数", "provider 级 502/503/504 与连接级抖动会使用这组独立预算。", () => <InputNumber min={0} step={1} className="full-width" />),
      mkField("connect_timeout_seconds", "连接超时（秒）", "控制 TCP 建连和 TLS 握手阶段的超时。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("response_header_timeout_seconds", "响应头超时（秒）", "上游迟迟不返回首包时会触发该超时。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("transient_cooling_seconds", "临时故障冷却秒数", "EOF、连接重置等连接级抖动会先短暂摘除当前 key。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("wait_for_key_timeout_ms", "等待 cooling Key 恢复（毫秒）", "当所有 key 只是短暂 cooling 时，代理会在这个预算内等最近一个 key 恢复。", () => <InputNumber min={0} step={50} className="full-width" />),
    ] as SettingFieldMeta[],
    networkFields: [
      mkField("listen", "代理监听地址", "例如 127.0.0.1:18080。", () => <Input placeholder="127.0.0.1:18080" />),
      mkField("admin_listen", "管理监听地址", "建议继续保持本地回环地址。", () => <Input placeholder="127.0.0.1:18081" />),
    ] as SettingFieldMeta[],
    logFields: [
      mkField("log_level", "日志级别", "debug 会附带更多运行细节与 source 信息。", () => (
        <Select options={[{ value: "debug", label: "debug" }, { value: "info", label: "info" }, { value: "warn", label: "warn" }, { value: "error", label: "error" }]} />
      )),
      mkField("log_format", "日志格式", "json 更适合被日志平台采集，text 更适合本地阅读。", () => (
        <Select options={[{ value: "text", label: "text" }, { value: "json", label: "json" }]} />
      )),
      mkField("log_output", "日志输出", "可选择仅输出到 stdout，或写文件，或双写。", () => (
        <Select options={[{ value: "stdout", label: "stdout" }, { value: "file", label: "file" }, { value: "both", label: "both" }]} />
      )),
      mkField("log_file", "日志文件路径", "当日志输出为 file 或 both 时必须填写。", () => <Input placeholder="logs/modelmux.log" />),
      mkField("log_max_size_mb", "单个日志文件大小（MB）", "超过该体积后会触发日志轮转。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("log_max_backups", "日志保留数量", "保留的旧日志文件数量上限。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("log_max_age_days", "日志保留天数", "旧日志达到保留天数后会被清理。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("log_compress", "压缩旧日志", "开启后，轮转出来的旧日志会被压缩保存。", () => <Checkbox>启用压缩</Checkbox>),
    ] as SettingFieldMeta[],
    stateFields: [
      mkField("persist_state", "状态持久化", "开启后会把 key 的 cooling/invalid 状态写入 state 文件。", () => <Checkbox>启用状态持久化</Checkbox>),
      mkField("state_file", "状态文件路径", "用于保存 provider/key 的运行状态快照。", () => <Input placeholder="state.json" />),
      mkField("invalid_ttl_hours", "失效 Key 保留时长（小时）", "invalid key 在下一次启动时超过该时长会重新恢复为 active。", () => <InputNumber min={1} step={1} className="full-width" />),
    ] as SettingFieldMeta[],
    statsFields: [
      mkField("stats_enabled", "调用统计", "开启后会把调用明细写入本地 JSONL 文件。", () => <Checkbox>启用调用统计</Checkbox>),
      mkField("stats_dir", "统计文件目录", "按天保存调用明细，默认 stats_data。", () => <Input placeholder="stats_data" />),
      mkField("stats_retention_days", "统计保留天数", "启动或跨天写入时会清理超过该天数的调用明细文件。", () => <InputNumber min={1} step={1} className="full-width" />),
      mkField("stats_max_recent_records", "最近记录内存上限", "限制启动时加载到内存、供管理台快速查询的最近调用记录数量。", () => <InputNumber min={100} step={100} className="full-width" />),
    ] as SettingFieldMeta[],
  };
}

export function buildFieldRules(field: keyof AdminSettingsPayload) {
  const requiredMessage = `请填写${fieldToLabel(field)}`;
  switch (field) {
    case "listen":
    case "admin_listen":
    case "log_level":
    case "log_format":
    case "log_output":
    case "state_file":
    case "stats_dir":
      return [{ required: true, message: requiredMessage }];
    case "log_file":
      return [];
    default:
      return [];
  }
}

export function toSaveSummary(result: AdminChangeResponse): SaveSummary {
  return {
    changedFields: result.changed_fields ?? [],
    hotReloadedFields: result.hot_reloaded_fields ?? [],
    restartRequiredFields: result.restart_required_fields ?? [],
  };
}

export function fieldToLabel(field: string): string {
  return fieldLabels[field] ?? field;
}
