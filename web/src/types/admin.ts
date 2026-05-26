// AdminEvent 表示后端最近事件缓冲区中的一条记录。
export type AdminEvent = {
  seq: number;
  at: string;
  level: string;
  category: string;
  event: string;
  message: string;
  data?: Record<string, unknown>;
};

// AdminProviderSummary 表示 provider 列表和 dashboard 中使用的汇总信息。
export type AdminProviderSummary = {
  id: string;
  active: boolean;
  target_url: string;
  total_keys: number;
  active_keys: number;
  cooling_keys: number;
  invalid_keys: number;
};

// AdminKeyStatus 表示 provider 详情中单个 key 的运行状态。
export type AdminKeyStatus = {
  index: number;
  key_id: string;
  masked_key: string;
  state: "active" | "cooling" | "invalid";
  req_count: number;
  err_count: number;
  avg_latency_ms: number;
  cool_until?: string;
  last_401_at?: string;
};

// AdminProviderDetailResponse 对应 provider 详情接口的响应结构。
export type AdminProviderDetailResponse = {
  id: string;
  active: boolean;
  target_url: string;
  total_keys: number;
  active_keys: number;
  cooling_keys: number;
  invalid_keys: number;
  keys: AdminKeyStatus[];
};

// AdminDashboardResponse 对应 dashboard 聚合接口的响应结构。
export type AdminDashboardResponse = {
  active_provider: string;
  provider_count: number;
  active_keys: number;
  cooling_keys: number;
  invalid_keys: number;
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
