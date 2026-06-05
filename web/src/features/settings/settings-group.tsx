import { Col, Form, Row, Typography } from "antd";

import type { AdminSettingsPayload } from "../../types/admin";
import { buildFieldRules } from "./settings-schema";
import type { SettingFieldMeta } from "./settings-types";

type SettingsGroupProps = {
  title: string;
  desc: string;
  fields: SettingFieldMeta[];
  inactiveFields?: Set<keyof AdminSettingsPayload>;
  last?: boolean;
};

export function SettingsGroup({ title, desc, fields, inactiveFields, last }: SettingsGroupProps): JSX.Element {
  return (
    <div className={`settings-group${last ? " settings-group--last" : ""}`}>
      <Typography.Text className="settings-group-title">{title}</Typography.Text>
      <Typography.Text className="settings-group-desc">{desc}</Typography.Text>
      <Row gutter={[16, 4]}>
        {fields.map((field) => (
          <Col xs={24} sm={12} key={String(field.name)}>
            <Form.Item<AdminSettingsPayload>
              label={renderFieldLabel(field)}
              name={field.name}
              className={inactiveFields?.has(field.name) ? "settings-form-item--inactive" : undefined}
              extra={<Typography.Text className="settings-field-hint">{field.hint}</Typography.Text>}
              rules={buildFieldRules(field.name)}
              valuePropName={isBooleanSetting(field.name) ? "checked" : "value"}
            >
              {field.render()}
            </Form.Item>
          </Col>
        ))}
      </Row>
    </div>
  );
}

function renderFieldLabel(field: SettingFieldMeta): JSX.Element {
  return (
    <span className="settings-field-label">
      <span>{field.label}</span>
      <span className={`settings-effect-${field.effect}`}>{effectLabel(field.effect)}</span>
    </span>
  );
}

function effectLabel(effect: SettingFieldMeta["effect"]): string {
  if (effect === "hot") return "热生效";
  if (effect === "restart") return "需重启";
  return "只读";
}

function isBooleanSetting(field: keyof AdminSettingsPayload): boolean {
  return field === "log_compress" || field === "persist_state" || field === "stats_enabled";
}
