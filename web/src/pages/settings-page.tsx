import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Alert,
  Button,
  Card,
  Checkbox,
  Col,
  Collapse,
  Divider,
  Form,
  Input,
  InputNumber,
  Result,
  Row,
  Select,
  Space,
  Spin,
  Tag,
  Typography,
  message,
} from "antd";
import { startTransition, useEffect, useMemo, useState } from "react";

import { fetchSettings, updateSettings } from "../api/admin";
import type { AdminChangeResponse, AdminSettingsPayload, AdminSettingsResponse } from "../types/admin";

type SaveSummary = {
  changedFields: string[];
  hotReloadedFields: string[];
  restartRequiredFields: string[];
};

const settingsQueryKey = ["settings"];

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
};

// SettingsPage 渲染运行配置编辑页，并明确区分热生效与需重启的设置项。
export function SettingsPage(): JSX.Element {
  const queryClient = useQueryClient();
  const [messageApi, contextHolder] = message.useMessage();
  const [form] = Form.useForm<AdminSettingsPayload>();
  const [saveSummary, setSaveSummary] = useState<SaveSummary | null>(null);

  const settingsQuery = useQuery({
    queryKey: settingsQueryKey,
    queryFn: fetchSettings,
  });

  useEffect(() => {
    if (!settingsQuery.data) {
      return;
    }
    form.setFieldsValue(settingsQuery.data.settings);
  }, [form, settingsQuery.data]);

  const updateSettingsMutation = useMutation({
    mutationFn: updateSettings,
    onSuccess: async (result) => {
      const summary = toSaveSummary(result);
      setSaveSummary(summary);
      messageApi.success(summary.changedFields.length === 0 ? "没有检测到配置变化" : "设置已保存");
      startTransition(() => {
        void queryClient.invalidateQueries({ queryKey: settingsQueryKey });
        void queryClient.invalidateQueries({ queryKey: ["dashboard"] });
        void queryClient.invalidateQueries({ queryKey: ["events"] });
      });
    },
    onError: (error: Error) => {
      messageApi.error(`保存失败：${error.message}`);
    },
  });

  const settingGroups = useMemo(() => {
    if (!settingsQuery.data) {
      return null;
    }
    return buildSettingGroups(settingsQuery.data);
  }, [settingsQuery.data]);

  if (settingsQuery.isLoading) {
    return (
      <div className="console-loading">
        <Spin size="large" />
      </div>
    );
  }

  if (settingsQuery.isError || !settingsQuery.data || !settingGroups) {
    return (
      <Result
        status="error"
        title="设置加载失败"
        subTitle={settingsQuery.error instanceof Error ? settingsQuery.error.message : "未知错误"}
      />
    );
  }

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={20} className="console-stack">
        {saveSummary ? <SaveSummaryBanner summary={saveSummary} /> : null}

        <Card className="surface-card" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Settings</Typography.Text>
              <Typography.Title level={3} className="section-title">
                运行配置
              </Typography.Title>
            </div>
            <Space wrap>
              <Tag color="green">{`当前活跃：${settingsQuery.data.settings.active_provider}`}</Tag>
              <Button onClick={() => resetFormFromServer(settingsQuery.data)}>重置为服务端配置</Button>
              <Button
                type="primary"
                loading={updateSettingsMutation.isPending}
                onClick={() => form.submit()}
              >
                保存设置
              </Button>
            </Space>
          </div>

          <Form<AdminSettingsPayload> form={form} layout="vertical" onFinish={(values) => updateSettingsMutation.mutate(values)}>
            <SettingSection title="核心运行参数" fields={settingGroups.coreFields} />

            <Collapse
              ghost
              className="settings-advanced-collapse"
              items={[
                {
                  key: "advanced",
                  label: (
                    <Space size={8}>
                      <Typography.Text strong>高级重试与超时</Typography.Text>
                      <Tag color="gold">高级</Tag>
                    </Space>
                  ),
                  children: <SettingSection fields={settingGroups.advancedFields} />,
                },
              ]}
            />

            <Divider />

            <SettingSection title="网络监听与日志" fields={settingGroups.serverAndLogFields} />

            <Divider />

            <SettingSection title="状态持久化" fields={settingGroups.stateFields} />
          </Form>
        </Card>
      </Space>
    </>
  );

  function resetFormFromServer(response: AdminSettingsResponse) {
    form.setFieldsValue(response.settings);
    setSaveSummary(null);
  }
}

type SaveSummaryBannerProps = {
  summary: SaveSummary;
};

// SaveSummaryBanner 在保存后集中展示本次变更的字段及其生效方式。
function SaveSummaryBanner({ summary }: SaveSummaryBannerProps): JSX.Element {
  if (summary.changedFields.length === 0) {
    return <Alert type="success" showIcon message="已保存，但没有检测到字段变化" />;
  }

  return (
    <Space direction="vertical" size={12} className="console-stack">
      <Alert
        type={summary.restartRequiredFields.length > 0 ? "warning" : "success"}
        showIcon
        message={summary.restartRequiredFields.length > 0 ? "保存成功，部分配置需重启生效" : "保存成功，变更已热生效"}
        description={
          summary.restartRequiredFields.length > 0
            ? "以下字段已经写入配置文件，但需要重启进程后才会完全生效。"
            : "以下字段已经写入配置文件，并且已经通过热重载生效。"
        }
      />
      <Card className="surface-card settings-summary-card" bordered={false}>
        <Space direction="vertical" size={10} className="console-stack">
          <div>
            <Typography.Text className="settings-summary-label">本次变更</Typography.Text>
            <div className="settings-tag-list">
              {summary.changedFields.map((field) => (
                <Tag key={field}>{fieldToLabel(field)}</Tag>
              ))}
            </div>
          </div>
          {summary.hotReloadedFields.length > 0 ? (
            <div>
              <Typography.Text className="settings-summary-label">已热生效</Typography.Text>
              <div className="settings-tag-list">
                {summary.hotReloadedFields.map((field) => (
                  <Tag color="green" key={field}>
                    {fieldToLabel(field)}
                  </Tag>
                ))}
              </div>
            </div>
          ) : null}
          {summary.restartRequiredFields.length > 0 ? (
            <div>
              <Typography.Text className="settings-summary-label">需重启生效</Typography.Text>
              <div className="settings-tag-list">
                {summary.restartRequiredFields.map((field) => (
                  <Tag color="gold" key={field}>
                    {fieldToLabel(field)}
                  </Tag>
                ))}
              </div>
            </div>
          ) : null}
        </Space>
      </Card>
    </Space>
  );
}

type SettingFieldMeta = {
  name: keyof AdminSettingsPayload;
  label: string;
  hint: string;
  effect: "hot" | "restart" | "readonly";
  render: () => JSX.Element;
};

type SettingSectionProps = {
  title?: string;
  fields: SettingFieldMeta[];
};

// SettingSection 按语义分组渲染设置字段，并在每一项旁边标注生效方式。
function SettingSection({ title, fields }: SettingSectionProps): JSX.Element {
  return (
    <div className="settings-section">
      {title ? (
        <div className="settings-section-head">
          <Typography.Title level={4} className="section-title">
            {title}
          </Typography.Title>
        </div>
      ) : null}
      <Row gutter={[18, 10]}>
        {fields.map((field) => (
          <Col xs={24} md={12} key={String(field.name)}>
            <Form.Item
              label={
                <Space size={8}>
                  <span>{field.label}</span>
                  {field.effect === "hot" ? <Tag color="green">热生效</Tag> : null}
                  {field.effect === "restart" ? <Tag color="gold">需重启</Tag> : null}
                  {field.effect === "readonly" ? <Tag>只读</Tag> : null}
                </Space>
              }
              name={field.name}
              tooltip={field.hint}
              rules={buildFieldRules(field.name)}
              valuePropName={field.name === "log_compress" || field.name === "persist_state" ? "checked" : "value"}
            >
              {field.render()}
            </Form.Item>
          </Col>
        ))}
      </Row>
    </div>
  );
}

// buildSettingGroups 把设置页字段拆成核心运行、高级重试、网络日志和状态持久化四组。
function buildSettingGroups(response: AdminSettingsResponse): {
  coreFields: SettingFieldMeta[];
  advancedFields: SettingFieldMeta[];
  serverAndLogFields: SettingFieldMeta[];
  stateFields: SettingFieldMeta[];
} {
  const hotSet = new Set(response.hot_reload_fields);
  const restartSet = new Set(response.restart_required_fields);

  const effectOf = (field: keyof AdminSettingsPayload): SettingFieldMeta["effect"] => {
    if (field === "active_provider") {
      return "readonly";
    }
    if (hotSet.has(field)) {
      return "hot";
    }
    if (restartSet.has(field)) {
      return "restart";
    }
    return "readonly";
  };

  return {
    coreFields: [
      {
        name: "active_provider",
        label: "当前活跃 Provider",
        hint: "在总览或提供商页面切换 active provider，这里仅做只读展示。",
        effect: effectOf("active_provider"),
        render: () => <Input disabled />,
      },
      {
        name: "cooling_seconds",
        label: "429 冷却秒数",
        hint: "当上游未返回 Retry-After 时，ModelMux 会使用这个秒数作为默认冷却时间。",
        effect: effectOf("cooling_seconds"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "max_retries",
        label: "最大重试次数",
        hint: "401、429 和配额不足类 403 会在当前 provider 内按 key 轮换重试。",
        effect: effectOf("max_retries"),
        render: () => <InputNumber min={0} step={1} className="full-width" />,
      },
      {
        name: "request_timeout_seconds",
        label: "上游请求超时（秒）",
        hint: "控制上游 HTTP client 的请求超时时间。",
        effect: effectOf("request_timeout_seconds"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "max_body_bytes",
        label: "请求体大小上限（字节）",
        hint: "超过该值的请求会在代理入口被直接拒绝。",
        effect: effectOf("max_body_bytes"),
        render: () => <InputNumber min={1024} step={1024} className="full-width" />,
      },
    ],
    advancedFields: [
      {
        name: "max_transient_retries",
        label: "临时故障最大重试次数",
        hint: "provider 级 502/503/504 与连接级抖动会使用这组独立预算，避免把整个 key 池扫一遍。",
        effect: effectOf("max_transient_retries"),
        render: () => <InputNumber min={0} step={1} className="full-width" />,
      },
      {
        name: "connect_timeout_seconds",
        label: "连接超时（秒）",
        hint: "控制 TCP 建连和 TLS 握手阶段的超时，适合缩短坏网络下的首次等待。",
        effect: effectOf("connect_timeout_seconds"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "response_header_timeout_seconds",
        label: "响应头超时（秒）",
        hint: "上游迟迟不返回首包时会触发该超时，并按临时故障策略处理。",
        effect: effectOf("response_header_timeout_seconds"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "transient_cooling_seconds",
        label: "临时故障冷却秒数",
        hint: "EOF、连接重置等连接级抖动会先短暂摘除当前 key，再尝试下一个。",
        effect: effectOf("transient_cooling_seconds"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "wait_for_key_timeout_ms",
        label: "等待 cooling Key 恢复（毫秒）",
        hint: "当所有 key 只是短暂 cooling 时，代理会在这个预算内等最近一个 key 恢复，尽量避免立刻向客户端暴露 503。",
        effect: effectOf("wait_for_key_timeout_ms"),
        render: () => <InputNumber min={0} step={50} className="full-width" />,
      },
    ],
    serverAndLogFields: [
      {
        name: "listen",
        label: "代理监听地址",
        hint: "例如 127.0.0.1:18080。",
        effect: effectOf("listen"),
        render: () => <Input placeholder="127.0.0.1:18080" />,
      },
      {
        name: "admin_listen",
        label: "管理监听地址",
        hint: "建议继续保持本地回环地址，避免把管理端暴露到公网。",
        effect: effectOf("admin_listen"),
        render: () => <Input placeholder="127.0.0.1:18081" />,
      },
      {
        name: "log_level",
        label: "日志级别",
        hint: "debug 会附带更多运行细节与 source 信息。",
        effect: effectOf("log_level"),
        render: () => (
          <Select
            options={[
              { value: "debug", label: "debug" },
              { value: "info", label: "info" },
              { value: "warn", label: "warn" },
              { value: "error", label: "error" },
            ]}
          />
        ),
      },
      {
        name: "log_format",
        label: "日志格式",
        hint: "json 更适合被日志平台采集，text 更适合本地阅读。",
        effect: effectOf("log_format"),
        render: () => (
          <Select
            options={[
              { value: "text", label: "text" },
              { value: "json", label: "json" },
            ]}
          />
        ),
      },
      {
        name: "log_output",
        label: "日志输出",
        hint: "可选择仅输出到 stdout，或写文件，或双写。",
        effect: effectOf("log_output"),
        render: () => (
          <Select
            options={[
              { value: "stdout", label: "stdout" },
              { value: "file", label: "file" },
              { value: "both", label: "both" },
            ]}
          />
        ),
      },
      {
        name: "log_file",
        label: "日志文件路径",
        hint: "当日志输出为 file 或 both 时必须填写。",
        effect: effectOf("log_file"),
        render: () => <Input placeholder="logs/modelmux.log" />,
      },
      {
        name: "log_max_size_mb",
        label: "单个日志文件大小（MB）",
        hint: "超过该体积后会触发日志轮转。",
        effect: effectOf("log_max_size_mb"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "log_max_backups",
        label: "日志保留数量",
        hint: "保留的旧日志文件数量上限。",
        effect: effectOf("log_max_backups"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "log_max_age_days",
        label: "日志保留天数",
        hint: "旧日志达到保留天数后会被清理。",
        effect: effectOf("log_max_age_days"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
      {
        name: "log_compress",
        label: "压缩旧日志",
        hint: "开启后，轮转出来的旧日志会被压缩保存。",
        effect: effectOf("log_compress"),
        render: () => <Checkbox>启用压缩</Checkbox>,
      },
    ],
    stateFields: [
      {
        name: "persist_state",
        label: "状态持久化",
        hint: "开启后会把 key 的 cooling/invalid 状态写入 state 文件。",
        effect: effectOf("persist_state"),
        render: () => <Checkbox>启用状态持久化</Checkbox>,
      },
      {
        name: "state_file",
        label: "状态文件路径",
        hint: "用于保存 provider/key 的运行状态快照。",
        effect: effectOf("state_file"),
        render: () => <Input placeholder="state.json" />,
      },
      {
        name: "invalid_ttl_hours",
        label: "失效 Key 保留时长（小时）",
        hint: "invalid key 在下一次启动时超过该时长会重新恢复为 active。",
        effect: effectOf("invalid_ttl_hours"),
        render: () => <InputNumber min={1} step={1} className="full-width" />,
      },
    ],
  };
}

// buildFieldRules 为设置页字段生成基础校验规则，避免提交无意义空值。
function buildFieldRules(field: keyof AdminSettingsPayload) {
  const requiredMessage = `请填写${fieldToLabel(field)}`;
  switch (field) {
    case "listen":
    case "admin_listen":
    case "log_level":
    case "log_format":
    case "log_output":
    case "state_file":
      return [{ required: true, message: requiredMessage }];
    case "log_file":
      return [];
    default:
      return [];
  }
}

// toSaveSummary 把后端保存响应规范化为前端结果摘要。
function toSaveSummary(result: AdminChangeResponse): SaveSummary {
  return {
    changedFields: result.changed_fields ?? [],
    hotReloadedFields: result.hot_reloaded_fields ?? [],
    restartRequiredFields: result.restart_required_fields ?? [],
  };
}

// fieldToLabel 把字段名转换成设置页更适合展示的中文标签。
function fieldToLabel(field: string): string {
  return fieldLabels[field] ?? field;
}
