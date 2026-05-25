import { useEffect, useState } from "react";

// useCountdown 返回距离目标时间的剩余毫秒数，每秒更新；目标到期或缺失时返回 null。
export function useCountdown(target: string | undefined | null): number | null {
  const [remainingMs, setRemainingMs] = useState<number | null>(() => computeRemaining(target));

  useEffect(() => {
    setRemainingMs(computeRemaining(target));
    if (!target) {
      return;
    }
    const timer = window.setInterval(() => {
      const next = computeRemaining(target);
      setRemainingMs(next);
      if (next === null) {
        window.clearInterval(timer);
      }
    }, 1000);
    return () => window.clearInterval(timer);
  }, [target]);

  return remainingMs;
}

function computeRemaining(target: string | undefined | null): number | null {
  if (!target) {
    return null;
  }
  const deadline = new Date(target).getTime();
  if (Number.isNaN(deadline)) {
    return null;
  }
  const diff = deadline - Date.now();
  return diff > 0 ? diff : null;
}
