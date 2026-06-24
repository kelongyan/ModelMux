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

// formatClockShort 把时间戳格式化为页头使用的简短时钟。
export function formatClockShort(timestamp: number): string {
  if (!timestamp) {
    return "--:--:--";
  }
  return new Date(timestamp).toLocaleTimeString("zh-CN", { hour12: false });
}

// formatDateTime 把 ISO 时间字符串格式化为本地日期时间（YYYY-MM-DD HH:mm:ss）。
// 空值返回 "-"，无效日期返回原始输入。
export function formatDateTime(value: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
