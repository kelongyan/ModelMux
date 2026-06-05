import { Button, Space, Typography } from "antd";

import type { AdminEvent } from "../../types/admin";
import { buildExpandedPayload } from "./events-utils";

type EventDetailProps = {
  event: AdminEvent;
};

export function EventDetail({ event }: EventDetailProps): JSX.Element {
  const rawPayload = JSON.stringify(buildExpandedPayload(event), null, 2);

  return (
    <div className="event-detail">
      <div className="event-detail-actions">
        <Space wrap>
          <Button size="small" disabled={!event.request_id} onClick={() => void copyText(event.request_id ?? "")}>复制 request_id</Button>
          <Button size="small" onClick={() => void copyText(rawPayload)}>复制 JSON</Button>
        </Space>
      </div>
      <DetailGroup
        title="请求"
        items={[
          ["request_id", event.request_id],
          ["client_request_id", event.client_request_id],
          ["method", event.method],
          ["path", event.path],
          ["model", event.model],
          ["stream", formatBoolean(event.stream)],
        ]}
      />
      <DetailGroup
        title="Provider / Key"
        items={[
          ["provider_id", event.provider_id],
          ["key_hint", event.key_hint],
          ["key_id", event.key_id],
        ]}
      />
      <DetailGroup
        title="结果"
        items={[
          ["status", event.status],
          ["latency_ms", formatLatency(event.latency_ms)],
          ["attempts", event.attempts],
          ["retry_scope", event.retry_scope],
        ]}
      />
      <DetailGroup
        title="事件"
        items={[
          ["category", event.category],
          ["event", event.event],
          ["message", event.message],
        ]}
      />
      <div className="event-detail-raw">
        <details className="event-detail-raw-details">
          <summary>
            <span className="event-detail-title">原始数据</span>
          </summary>
          <pre className="events-table-payload">{rawPayload}</pre>
        </details>
      </div>
    </div>
  );
}

async function copyText(value: string): Promise<void> {
  if (!value || !navigator.clipboard) {
    return;
  }
  await navigator.clipboard.writeText(value);
}

type DetailGroupProps = {
  title: string;
  items: Array<[string, unknown]>;
};

function DetailGroup({ title, items }: DetailGroupProps): JSX.Element {
  return (
    <div className="event-detail-group">
      <Typography.Text className="event-detail-title">{title}</Typography.Text>
      <dl className="event-detail-list">
        {items.map(([label, value]) => (
          <div className="event-detail-item" key={label}>
            <dt>{label}</dt>
            <dd>{formatValue(value)}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function formatValue(value: unknown): string {
  if (value === undefined || value === null || value === "") return "-";
  return String(value);
}

function formatBoolean(value: boolean | undefined): string | undefined {
  if (typeof value !== "boolean") return undefined;
  return value ? "true" : "false";
}

function formatLatency(value: number | undefined): string | undefined {
  if (typeof value !== "number") return undefined;
  return `${value}ms`;
}
