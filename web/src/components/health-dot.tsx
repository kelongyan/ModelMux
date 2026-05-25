// HealthDot 渲染单个状态圆点，可叠加 pulse 动画提示 LIVE 状态。
type HealthDotProps = {
  state: "active" | "cooling" | "invalid" | "idle";
  pulse?: boolean;
};

export function HealthDot({ state, pulse = false }: HealthDotProps): JSX.Element {
  const cls = `health-dot health-dot--${state}${pulse ? " health-dot--pulse" : ""}`;
  return <span className={cls} aria-hidden="true" />;
}
