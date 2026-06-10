// AdminEvent 表示后端最近事件缓冲区中的一条记录。
export type AdminEvent = {
  seq: number;
  at: string;
  level: string;
  category: string;
  event: string;
  message: string;
  request_id?: string;
  client_request_id?: string;
  provider_id?: string;
  key_id?: string;
  key_hint?: string;
  method?: string;
  path?: string;
  model?: string;
  stream?: boolean;
  status?: number;
  latency_ms?: number;
  attempts?: number;
  retry_scope?: string;
  data?: Record<string, unknown>;
};

// AdminProviderSummary 表示 provider 列表和 dashboard 中使用的汇总信息。
export type AdminProviderSummary = {
  id: string;
  active: boolean;
  target_url: string;
  total_keys: number;
  disabled_keys: number;
  active_keys: number;
  cooling_keys: number;
  invalid_keys: number;
  models: string[];
};

export type AdminProviderCircuit = {
  provider_id: string;
  state: "closed" | "open" | "half_open" | string;
  consecutive_failures: number;
  open_until?: string;
  half_open_in_flight: number;
  current_cooling_seconds: number;
};

export type AdminStatsHealth = {
  enabled: boolean;
  dropped_records: number;
  queue_depth: number;
  queue_capacity: number;
};

// AdminKeyStatus 表示 provider 详情中单个 key 的运行状态。
export type AdminKeyStatus = {
  index?: number;
  key_id: string;
  masked_key: string;
  state: "active" | "cooling" | "invalid" | "disabled";
  req_count: number;
  err_count: number;
  avg_latency_ms: number;
  cool_until?: string;
  last_401_at?: string;
  invalid_reason?: "unauthorized" | "quota_exhausted" | string;
  label?: string;
  note?: string;
  disabled?: boolean;
};

// AdminProviderDetailResponse 对应 provider 详情接口的响应结构。
export type AdminProviderDetailResponse = {
  id: string;
  active: boolean;
  target_url: string;
  total_keys: number;
  disabled_keys: number;
  active_keys: number;
  cooling_keys: number;
  invalid_keys: number;
  keys: AdminKeyStatus[];
  models: string[];
  strip_tools: boolean;
};

// AdminDashboardResponse 对应 dashboard 聚合接口的响应结构。
export type AdminDashboardResponse = {
  active_provider: string;
  provider_count: number;
  active_keys: number;
  cooling_keys: number;
  invalid_keys: number;
  provider_circuit?: AdminProviderCircuit;
  stats: AdminStatsHealth;
  providers: AdminProviderSummary[];
  events: AdminEvent[];
};

// AdminProvidersResponse 对应 provider 列表接口的响应结构。
export type AdminProvidersResponse = {
  active_provider: string;
  providers: AdminProviderSummary[];
};

// AdminEventsResponse 对应最近事件接口的响应结构。
export type AdminEventsResponse = {
  events: AdminEvent[];
};

// AdminReloadResponse 对应手动 reload 接口的响应结构。
export type AdminReloadResponse = {
  ok: boolean;
};

// AdminChangeResponse 对应会修改配置或状态的写接口响应结构。
export type AdminChangeResponse = {
  ok: boolean;
  active_provider?: string;
  changed_fields?: string[];
  hot_reloaded_fields?: string[];
  restart_required_fields?: string[];
};

export type AdminKeyMetadataPayload = {
  label?: string;
  note?: string;
  disabled?: boolean;
};

export type AdminKeysPreviewPayload = {
  mode: "append" | "replace";
  keys: string[];
};

export type AdminKeyPreviewEntry = {
  key_id: string;
  masked_key: string;
  label?: string;
  disabled?: boolean;
};

export type AdminKeysPreviewResponse = {
  mode: "append" | "replace" | string;
  input_count: number;
  normalized_count: number;
  duplicate_count: number;
  existing_count: number;
  new_count: number;
  removed_count: number;
  existing_keys: AdminKeyPreviewEntry[];
  new_keys: AdminKeyPreviewEntry[];
  removed_keys: AdminKeyPreviewEntry[];
};

export type AdminKeyTestResponse = {
  ok: boolean;
  status_code: number;
  latency_ms: number;
  scope?: string;
  error?: string;
  retry_after_seconds?: number;
};

export type AdminKeysResetAllResponse = {
  ok: boolean;
  reset_count: number;
};

// AdminProviderCreatePayload 对应 provider 新增表单提交结构。
export type AdminProviderCreatePayload = {
  id: string;
  target_url: string;
  keys: string[];
};

// AdminProviderUpdatePayload 对应 provider 基础信息编辑结构。
export type AdminProviderUpdatePayload = {
  target_url: string;
};

// AdminKeysPayload 对应 provider key 追加与替换动作。
export type AdminKeysPayload = {
  keys: string[];
};

// AdminDeleteKeysPayload 对应按 key_id 删除 key 的动作结构。
export type AdminDeleteKeysPayload = {
  key_ids: string[];
};

// AdminSettingsPayload 对应设置页的完整配置表单结构。
export type AdminSettingsPayload = {
  listen: string;
  admin_listen: string;
  active_provider: string;
  cooling_seconds: number;
  max_retries: number;
  max_transient_retries: number;
  request_timeout_seconds: number;
  connect_timeout_seconds: number;
  response_header_timeout_seconds: number;
  transient_cooling_seconds: number;
  wait_for_key_timeout_ms: number;
  stream_keepalive_seconds: number;
  stream_idle_timeout_seconds: number;
  stream_max_duration_seconds: number;
  provider_circuit_failure_threshold: number;
  provider_circuit_open_seconds: number;
  provider_circuit_max_open_seconds: number;
  provider_circuit_half_open_max: number;
  max_body_bytes: number;
  log_level: string;
  log_format: string;
  log_output: string;
  log_file: string;
  log_max_size_mb: number;
  log_max_backups: number;
  log_max_age_days: number;
  log_compress: boolean;
  persist_state: boolean;
  state_file: string;
  invalid_ttl_hours: number;
  stats_enabled: boolean;
  stats_dir: string;
  stats_retention_days: number;
  stats_max_recent_records: number;
};

// AdminSettingsResponse 对应设置页读取接口的响应结构。
export type AdminSettingsResponse = {
  settings: AdminSettingsPayload;
  hot_reload_fields: string[];
  restart_required_fields: string[];
};

// AdminAboutResponse 对应关于页的运行信息接口。
export type AdminAboutResponse = {
  app_name: string;
  version: string;
  go_version: string;
  platform: string;
  build_time: string;
  config_path: string;
  listen: string;
  admin_listen: string;
  state_file: string;
  active_provider: string;
  provider_count: number;
  features: string[];
  api_endpoints: string[];
  backup_endpoints: string[];
};

export type AdminStatsWindow = "1h" | "24h" | "7d" | "30d";

export type AdminStatsSummary = {
  total_calls: number;
  success_calls: number;
  failed_calls: number;
  usage_known_calls: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  avg_latency_ms: number;
};

export type AdminModelStats = {
  model: string;
  calls: number;
  success_calls: number;
  failed_calls: number;
  usage_known_calls: number;
  prompt_tokens: number;
  completion_tokens: number;
  total_tokens: number;
  avg_latency_ms: number;
};

export type AdminCallRecord = {
  id: string;
  at: string;
  provider_id: string;
  model?: string;
  endpoint: string;
  method: string;
  status: number;
  success: boolean;
  stream?: boolean;
  latency_ms: number;
  attempts: number;
  key_id?: string;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  usage_source: "upstream" | "unknown" | string;
  error?: string;
};

export type AdminStatsSummaryResponse = {
  window: AdminStatsWindow;
  since: string;
  summary: AdminStatsSummary;
  dropped_records: number;
  queue_depth: number;
  queue_capacity: number;
};

export type AdminStatsModelsResponse = {
  window: AdminStatsWindow;
  since: string;
  models: AdminModelStats[];
};

export type AdminStatsRecentResponse = {
  records: AdminCallRecord[];
};

// AdminModelsPayload 对应 provider 模型 ID 替换动作。
export type AdminModelsPayload = {
  models: string[];
};

// AdminFetchModelsResponse 对应从上游拉取模型列表的响应。
export type AdminFetchModelsResponse = {
  models: string[];
  count: number;
};

// AdminStatsLogsResponse 对应调用日志查询接口（过滤+分页）。
export type AdminStatsLogsResponse = {
  window: AdminStatsWindow;
  since: string;
  records: AdminCallRecord[];
  total: number;
  page: number;
  page_size: number;
};
