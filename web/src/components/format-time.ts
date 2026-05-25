// formatCooldown 把剩余毫秒数格式化为简短倒计时文本，例如 28s 或 3m12s。
export function formatCooldown(remainingMs: number | null): string {
  if (remainingMs === null || remainingMs <= 0) {
    return "-";
  }
  const totalSeconds = Math.ceil(remainingMs / 1000);
  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (seconds === 0) {
    return `${minutes}m`;
  }
  return `${minutes}m${seconds}s`;
}

// formatRelativeTime 把过去时间字符串格式化为相对时间（如 5min ago）。
export function formatRelativeTime(value: string | undefined | null): string {
  if (!value) {
    return "-";
  }
  const ts = new Date(value).getTime();
  if (Number.isNaN(ts)) {
    return "-";
  }
  const diffSec = Math.max(0, Math.floor((Date.now() - ts) / 1000));
  if (diffSec < 60) {
    return `${diffSec}s ago`;
  }
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) {
    return `${diffMin}min ago`;
  }
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) {
    return `${diffHr}h ago`;
  }
  const diffDay = Math.floor(diffHr / 24);
  return `${diffDay}d ago`;
}

// formatClockShort 把时间戳格式化为页头使用的简短时钟。
export function formatClockShort(timestamp: number): string {
  if (!timestamp) {
    return "--:--:--";
  }
  return new Date(timestamp).toLocaleTimeString("zh-CN", { hour12: false });
}
