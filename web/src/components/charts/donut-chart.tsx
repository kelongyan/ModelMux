import "./donut-chart.css";

export type DonutSegment = {
  value: number;
  color: string;
  label: string;
};

type DonutChartProps = {
  segments: DonutSegment[];
  centerText: string;
  centerSub?: string;
  size?: number;
  className?: string;
};

const GAP_DEG = 4;

export function DonutChart({ segments, centerText, centerSub, size = 100, className }: DonutChartProps): JSX.Element {
  const visible = segments.filter((s) => s.value > 0);
  const total = visible.reduce((sum, s) => sum + s.value, 0);

  if (total === 0) {
    return (
      <div className={`chart-donut-wrap ${className ?? ""}`}>
        <div className="chart-donut-empty">暂无数据</div>
      </div>
    );
  }

  const r = (size * 0.38);
  const circumference = 2 * Math.PI * r;
  const strokeW = size * 0.08;

  const totalGapDeg = visible.length > 1 ? visible.length * GAP_DEG : 0;
  const availableDeg = 360 - totalGapDeg;

  let cumulativeOffset = 0;

  return (
    <div className={`chart-donut-wrap ${className ?? ""}`}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} className="chart-donut-svg">
        <circle
          cx={size / 2}
          cy={size / 2}
          r={r}
          fill="none"
          stroke="var(--mm-glass-bg-subtle)"
          strokeWidth={strokeW}
        />
        {visible.map((seg, i) => {
          const deg = (seg.value / total) * availableDeg;
          const arcLen = (deg / 360) * circumference;
          const offset = -(cumulativeOffset / 360) * circumference;
          cumulativeOffset += deg + GAP_DEG;
          return (
            <circle
              key={i}
              cx={size / 2}
              cy={size / 2}
              r={r}
              fill="none"
              stroke={seg.color}
              strokeWidth={strokeW}
              strokeLinecap="butt"
              strokeDasharray={`${arcLen} ${circumference - arcLen}`}
              strokeDashoffset={offset}
              transform={`rotate(-90 ${size / 2} ${size / 2})`}
              className="chart-donut-segment"
            />
          );
        })}
        <text x={size / 2} y={size * 0.46} className="chart-donut-center">
          {centerText}
        </text>
        {centerSub ? (
          <text x={size / 2} y={size * 0.58} className="chart-donut-sub">
            {centerSub}
          </text>
        ) : null}
      </svg>
      <div className="chart-donut-legend">
        {visible.map((seg, i) => (
          <div key={i} className="chart-donut-legend-item">
            <span className="chart-donut-legend-dot" style={{ background: seg.color }} />
            <span>{seg.label}</span>
            <span className="chart-donut-legend-value">{seg.value}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
