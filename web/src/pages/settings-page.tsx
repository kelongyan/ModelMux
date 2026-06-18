import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Button, Card, Collapse, Form, Result, Skeleton, Space, Typography, message } from "antd";
import { startTransition, useEffect, useMemo, useState } from "react";

import { fetchSettings, updateSettings } from "../api/admin";
import { queryKeys } from "../api/query-keys";
import { SaveSummaryBanner } from "../features/settings/save-summary-banner";
import { buildSettingGroups, toSaveSummary } from "../features/settings/settings-schema";
import { SettingsGroup } from "../features/settings/settings-group";
import type { SaveSummary } from "../features/settings/settings-types";
import type { AdminSettingsPayload } from "../types/admin";

export function SettingsPage(): JSX.Element {
  const queryClient = useQueryClient();
  const [messageApi, contextHolder] = message.useMessage();
  const [form] = Form.useForm<AdminSettingsPayload>();
  const [saveSummary, setSaveSummary] = useState<SaveSummary | null>(null);
  const logOutput = Form.useWatch("log_output", form);
  const persistState = Form.useWatch("persist_state", form);
  const statsEnabled = Form.useWatch("stats_enabled", form);

  const settingsQuery = useQuery({
    queryKey: queryKeys.settings,
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
        void queryClient.invalidateQueries({ queryKey: queryKeys.settings });
        void queryClient.invalidateQueries({ queryKey: queryKeys.dashboard });
        void queryClient.invalidateQueries({ queryKey: queryKeys.events(200) });
      });
    },
    onError: (error: Error) => {
      messageApi.error(`保存失败：${error.message}`);
    },
  });

  const groups = useMemo(() => {
    if (!settingsQuery.data) {
      return null;
    }
    return buildSettingGroups(settingsQuery.data);
  }, [settingsQuery.data]);

  const inactiveFields = useMemo(() => {
    const fields = new Set<keyof AdminSettingsPayload>();
    if (logOutput === "stdout") {
      fields.add("log_file");
      fields.add("log_max_size_mb");
      fields.add("log_max_backups");
      fields.add("log_max_age_days");
      fields.add("log_compress");
    }
    if (persistState === false) {
      fields.add("state_file");
      fields.add("invalid_ttl_hours");
    }
    if (statsEnabled === false) {
      fields.add("stats_dir");
      fields.add("stats_retention_days");
      fields.add("stats_max_recent_records");
    }
    return fields;
  }, [logOutput, persistState, statsEnabled]);

  if (settingsQuery.isLoading) {
    return (
      <div className="console-loading">
        <Skeleton active paragraph={{ rows: 8 }} />
      </div>
    );
  }

  if (settingsQuery.isError || !settingsQuery.data || !groups) {
    return (
      <Result
        status="error"
        title="设置加载失败"
        subTitle={settingsQuery.error instanceof Error ? settingsQuery.error.message : "未知错误"}
        extra={<Button onClick={() => void settingsQuery.refetch()}>重试</Button>}
      />
    );
  }

  return (
    <>
      {contextHolder}
      <Space direction="vertical" size={12} className="console-stack">
        {saveSummary ? <SaveSummaryBanner summary={saveSummary} /> : null}

        <Card className="surface-card reveal-card reveal-delay-0" bordered={false}>
          <div className="section-heading">
            <div>
              <Typography.Text className="placeholder-kicker">Settings</Typography.Text>
              <Typography.Title level={3} className="section-title">
                运行配置
              </Typography.Title>
            </div>
            <Space wrap>
              <Button size="small" onClick={() => resetForm()}>重置</Button>
              <Button
                size="small"
                type="primary"
                loading={updateSettingsMutation.isPending}
                onClick={() => form.submit()}
              >
                保存设置
              </Button>
            </Space>
          </div>

          <Form<AdminSettingsPayload> form={form} layout="vertical" onFinish={(values) => updateSettingsMutation.mutate(values)}>
            <SettingsGroup title="核心运行参数" desc="重试、超时、请求体限制 — 最常调整的运行项。" fields={groups.coreFields} />
            <SettingsGroup title="高级重试与超时" desc="临时故障、响应头超时和 cooling 等边界行为。" fields={groups.advancedFields} />

            <Collapse
              ghost
              className="settings-collapse"
              items={[
                {
                  key: "adv",
                  label: <Typography.Text strong style={{ fontSize: "0.88rem" }}>高级设置</Typography.Text>,
                  children: (
                    <>
                      <SettingsGroup title="网络监听" desc="监听地址变更需要重启才能生效。" fields={groups.networkFields} inactiveFields={inactiveFields} />
                      <SettingsGroup title="日志" desc="日志输出、轮转和压缩策略。" fields={groups.logFields} inactiveFields={inactiveFields} />
                      <SettingsGroup title="状态持久化" desc="Key 状态文件、invalid 恢复窗口。" fields={groups.stateFields} inactiveFields={inactiveFields} />
                      <SettingsGroup title="调用统计" desc="JSONL 明细、保留天数与内存记录窗口。" fields={groups.statsFields} inactiveFields={inactiveFields} last />
                    </>
                  ),
                },
              ]}
            />
          </Form>
        </Card>
      </Space>
    </>
  );

  function resetForm() {
    form.setFieldsValue(settingsQuery.data!.settings);
    setSaveSummary(null);
  }
}
