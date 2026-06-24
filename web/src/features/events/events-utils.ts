import type { AdminEvent } from "../../types/admin";

export function filterEvents(events: AdminEvent[], keyword: string, level: string, category: string, provider: string): AdminEvent[] {
  const normalizedKeyword = keyword.trim().toLowerCase();

  return events.filter((event) => {
    if (level !== "all" && event.level !== level) {
      return false;
    }
    if (category !== "all" && event.category !== category) {
      return false;
    }
    if (provider !== "all" && event.provider_id !== provider) {
      return false;
    }
    if (!normalizedKeyword) {
      return true;
    }
    return searchableEventText(event).includes(normalizedKeyword);
  });
}

export function buildEventSelectOptions(events: AdminEvent[], field: "category" | "provider_id") {
  const values = new Set<string>();
  for (const event of events) {
    const value = event[field];
    if (value) {
      values.add(value);
    }
  }
  return Array.from(values)
    .sort((left, right) => left.localeCompare(right))
    .map((value) => ({ label: value, value }));
}

export function summarizeEvents(events: AdminEvent[]) {
  return {
    total: events.length,
    info: events.filter((event) => event.level === "info").length,
    warn: events.filter((event) => event.level === "warn").length,
    error: events.filter((event) => event.level === "error").length,
    lastErrorAt: events.find((event) => event.level === "error")?.at,
  };
}

export function buildExpandedPayload(event: AdminEvent): Record<string, unknown> {
  return {
    category: event.category,
    event: event.event,
    request_id: event.request_id,
    client_request_id: event.client_request_id,
    provider_id: event.provider_id,
    key_id: event.key_id,
    key_hint: event.key_hint,
    method: event.method,
    path: event.path,
    model: event.model,
    stream: event.stream,
    status: event.status,
    latency_ms: event.latency_ms,
    attempts: event.attempts,
    retry_scope: event.retry_scope,
    ...(event.data ?? {}),
  };
}

export function dedupeEvents(events: AdminEvent[]): AdminEvent[] {
  const seen = new Set<number>();
  const unique: AdminEvent[] = [];
  for (const event of events) {
    if (!seen.has(event.seq)) {
      seen.add(event.seq);
      unique.push(event);
    }
  }
  return unique;
}

function searchableEventText(event: AdminEvent): string {
  return [
    event.message,
    event.category,
    event.event,
    event.provider_id ?? "",
    event.request_id ?? "",
    event.client_request_id ?? "",
    event.key_id ?? "",
    event.key_hint ?? "",
    event.model ?? "",
    String(event.status ?? ""),
    JSON.stringify(event.data ?? {}),
  ]
    .join(" ")
    .toLowerCase();
}
