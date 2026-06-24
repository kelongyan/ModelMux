export function formatNumber(value: number): string {
  return new Intl.NumberFormat("zh-CN").format(value);
}

export function deriveTotalTokens(record: {
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
}): number | undefined {
  if (record.total_tokens != null) return record.total_tokens;
  if (record.prompt_tokens != null && record.completion_tokens != null) {
    return record.prompt_tokens + record.completion_tokens;
  }
  return undefined;
}

export function formatLatencySec(ms: number): string {
  if (ms <= 0) return "0s";
  const sec = ms / 1000;
  return `${sec.toFixed(1)}s`;
}

export function formatPercent(part: number, total: number): string {
  return total <= 0 ? "0%" : `${Math.round((part / total) * 100)}%`;
}

export function latencyClass(ms: number): string {
  if (ms < 500) return "stats-latency-good";
  if (ms < 2000) return "stats-latency-warn";
  return "stats-latency-bad";
}
