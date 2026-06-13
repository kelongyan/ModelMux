import "./mini-trend.css";

export type TrendColumn = {
  label: string;
  value: number;
};

type MiniTrendProps = {
  columns: TrendColumn[];
  maxHeight?: number;
  className?: string;
};

function formatNum(n: number): string {
  if (n >= 10000) {
    const k = n / 1000;
    return k >= 100 ? `${Math.round(k)}k` : `${k.toFixed(1)}k`;
  }
  return n.toLocaleString();
}

export function MiniTrend({ columns, maxHeight = 60, className }: MiniTrendProps): JSX.Element {
  const maxVal = Math.max(...columns.map((c) => c.value), 1);

  return (
    <div className={`chart-mini-trend ${className ?? ""}`} style={{ minHeight: maxHeight }}>
      {columns.map((col, i) => {
        const pct = Math.max(Math.round((col.value / maxVal) * 100), 8);
        return (
          <div key={i} className="chart-mini-trend-col">
            <span className="chart-mini-trend-value">{formatNum(col.value)}</span>
            <div className="chart-mini-trend-bar" style={{ height: `${pct}%` }} />
            <span className="chart-mini-trend-label">{col.label}</span>
          </div>
        );
      })}
    </div>
  );
}
