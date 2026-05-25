// KeyPoolDots 用一排圆点把可用、冷却、失效的 key 状态可视化，便于在卡片中一眼读出健康度。
type KeyPoolDotsProps = {
  active: number;
  cooling: number;
  invalid: number;
  max?: number;
  size?: "small" | "medium";
};

export function KeyPoolDots({ active, cooling, invalid, max = 24, size = "medium" }: KeyPoolDotsProps): JSX.Element {
  const total = active + cooling + invalid;
  const dots = buildDots(active, cooling, invalid, max);
  const aria = `active ${active} cooling ${cooling} invalid ${invalid}`;
  const overflow = total > max ? total - max : 0;
  const sizeClass = size === "small" ? "key-pool-dots key-pool-dots--small" : "key-pool-dots";

  return (
    <div className={sizeClass} role="img" aria-label={aria}>
      {dots.length === 0 ? <span className="key-pool-dot key-pool-dot--empty" /> : null}
      {dots.map((kind, idx) => (
        <span key={`${kind}-${idx}`} className={`key-pool-dot key-pool-dot--${kind}`} />
      ))}
      {overflow > 0 ? <span className="key-pool-dots-more">{`+${overflow}`}</span> : null}
    </div>
  );
}

// buildDots 按状态优先级（active → cooling → invalid）展开圆点，超过 max 时截断。
function buildDots(active: number, cooling: number, invalid: number, max: number): Array<"active" | "cooling" | "invalid"> {
  const dots: Array<"active" | "cooling" | "invalid"> = [];
  const safe = (n: number) => Math.max(0, Math.floor(n));
  const remaining = () => Math.max(0, max - dots.length);

  for (let i = 0; i < Math.min(safe(active), remaining()); i++) {
    dots.push("active");
  }
  for (let i = 0; i < Math.min(safe(cooling), remaining()); i++) {
    dots.push("cooling");
  }
  for (let i = 0; i < Math.min(safe(invalid), remaining()); i++) {
    dots.push("invalid");
  }
  return dots;
}
