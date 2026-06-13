import "./progress-bar.css";

type ProgressBarProps = {
  value: number;
  max: number;
  color: string;
  label: string;
  detail?: string;
  className?: string;
};

export function ProgressBar({ value, max, color, label, detail, className }: ProgressBarProps): JSX.Element {
  const pct = max > 0 ? Math.min(Math.round((value / max) * 100), 100) : 0;

  return (
    <div className={`chart-progress ${className ?? ""}`}>
      <div className="chart-progress-header">
        <span className="chart-progress-label">{label}</span>
        {detail ? <span className="chart-progress-detail">{detail}</span> : null}
      </div>
      <div className="chart-progress-track">
        <div className="chart-progress-fill" style={{ width: `${pct}%`, background: color }} />
      </div>
    </div>
  );
}
