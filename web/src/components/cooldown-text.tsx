import { formatCooldown } from "./format-time";
import { useCountdown } from "./use-countdown";

// CooldownText 把绝对的 cool_until 时间渲染为每秒自更新的倒计时；过期后显示 fallback。
type CooldownTextProps = {
  until?: string;
  fallback?: string;
  className?: string;
};

export function CooldownText({ until, fallback = "-", className }: CooldownTextProps): JSX.Element {
  const remaining = useCountdown(until);
  if (remaining === null) {
    return <span className={className}>{fallback}</span>;
  }
  return <span className={className ? `${className} cooldown-text` : "cooldown-text"}>{formatCooldown(remaining)}</span>;
}
