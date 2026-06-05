export function formatNumber(value: number): string {
  return new Intl.NumberFormat("zh-CN").format(value);
}

export function formatPercent(part: number, total: number): string {
  return total <= 0 ? "0%" : `${Math.round((part / total) * 100)}%`;
}

export function formatLocalDateTime(iso: string): string {
  if (!iso) return "-";
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  const pad = (value: number) => String(value).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export function latencyClass(ms: number): string {
  if (ms < 500) return "stats-latency-good";
  if (ms < 2000) return "stats-latency-warn";
  return "stats-latency-bad";
}
