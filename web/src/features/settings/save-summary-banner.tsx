import { Alert, Card, Space, Typography } from "antd";

import { fieldToLabel } from "./settings-schema";
import type { SaveSummary } from "./settings-types";

type SaveSummaryBannerProps = {
  summary: SaveSummary;
};

export function SaveSummaryBanner({ summary }: SaveSummaryBannerProps): JSX.Element {
  if (summary.changedFields.length === 0) {
    return <Alert className="reveal-card reveal-delay-0" type="success" showIcon message="已保存，但没有检测到字段变化" />;
  }

  return (
    <Space direction="vertical" size={10} className="console-stack">
      <Alert
        className="reveal-card reveal-delay-0"
        type={summary.restartRequiredFields.length > 0 ? "warning" : "success"}
        showIcon
        message={summary.restartRequiredFields.length > 0 ? "保存成功，部分配置需重启生效" : "保存成功，变更已热生效"}
        description={
          summary.restartRequiredFields.length > 0
            ? "以下字段已经写入配置文件，但需要重启进程后才会完全生效。"
            : "以下字段已经写入配置文件，并且已经通过热重载生效。"
        }
      />
      <Card className="surface-card settings-summary-card reveal-card reveal-delay-1" bordered={false}>
        <Space direction="vertical" size={8} className="console-stack">
          <div>
            <Typography.Text className="settings-summary-label">本次变更</Typography.Text>
            <Typography.Text className="settings-summary-fields">
              {summary.changedFields.map(fieldToLabel).join("、")}
            </Typography.Text>
          </div>
          {summary.hotReloadedFields.length > 0 ? (
            <div>
              <Typography.Text className="settings-summary-label">已热生效</Typography.Text>
              <Typography.Text className="settings-summary-fields" style={{ color: "#16a34a" }}>
                {summary.hotReloadedFields.map(fieldToLabel).join("、")}
              </Typography.Text>
            </div>
          ) : null}
          {summary.restartRequiredFields.length > 0 ? (
            <div>
              <Typography.Text className="settings-summary-label">需重启生效</Typography.Text>
              <Typography.Text className="settings-summary-fields" style={{ color: "#d97706" }}>
                {summary.restartRequiredFields.map(fieldToLabel).join("、")}
              </Typography.Text>
            </div>
          ) : null}
        </Space>
      </Card>
    </Space>
  );
}
