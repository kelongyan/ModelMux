# Shared UI Components — ModelMux Aurora Console

## Overview

- **Framework**: React 18 + TypeScript + Vite 6
- **UI library**: Ant Design 5 (`antd` ^5.24.8) — Button, Card, Table, Modal, Drawer, Select, Segmented, Tag, Result, Empty, Skeleton, Popconfirm, Form, DatePicker, Collapse, Switch, Input
- **Data fetching**: TanStack Query v5
- **Routing**: react-router-dom v6
- **Charts**: Recharts 3
- **CSS**: vanilla CSS files under `src/styles/*.css`, imported via `src/styles.css`; design tokens as CSS custom properties (`--mm-*`) defined in `base.css`
- **Styling convention**: semantic CSS classes (`.surface-card`, `.section-heading`, `.section-title`, `.provider-row`, etc.) — NOT Tailwind, NOT CSS modules
- **UI copy language**: Chinese (zh-CN locale)

## Custom shared components (full source)

### HealthDot — status dot with optional pulse

Source: `web/src/components/health-dot.tsx`

```tsx
// HealthDot 渲染单个状态圆点，可叠加 pulse 动画提示 LIVE 状态。
type HealthDotProps = {
  state: "active" | "cooling" | "invalid" | "idle";
  pulse?: boolean;
};

export function HealthDot({ state, pulse = false }: HealthDotProps): JSX.Element {
  const cls = `health-dot health-dot--${state}${pulse ? " health-dot--pulse" : ""}`;
  return <span className={cls} aria-hidden="true" />;
}
```

### CooldownText — live countdown for cool_until timestamps

Source: `web/src/components/cooldown-text.tsx`

```tsx
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
```

### ErrorBoundary — page-level error fallback

Source: `web/src/components/error-boundary.tsx`

```tsx
import { Component, type ErrorInfo, type ReactNode } from "react";
import { Button, Result } from "antd";

interface Props {
  children: ReactNode;
}

interface State {
  hasError: boolean;
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(): State {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo): void {
    console.error("ErrorBoundary caught:", error, info);
  }

  handleRetry = (): void => {
    this.setState({ hasError: false });
  };

  render(): ReactNode {
    if (this.state.hasError) {
      return (
        <Result
          status="error"
          title="页面渲染出错"
          subTitle="请尝试刷新页面或点击下方按钮重试"
          extra={
            <Button type="primary" onClick={this.handleRetry}>
              重试
            </Button>
          }
        />
      );
    }
    return this.props.children;
  }
}
```

### PageTransition — route change wrapper with CSS animation

Source: `web/src/components/page-transition.tsx` (+ `page-transition.css`)

```tsx
import "./page-transition.css";

interface PageTransitionProps {
  readonly animationKey: string;
  readonly children: React.ReactNode;
}

export function PageTransition({
  animationKey,
  children,
}: PageTransitionProps): JSX.Element {
  return (
    <div className="page-transition-wrapper">
      <div key={animationKey} className="page-transition-content">
        {children}
      </div>
    </div>
  );
}
```

### ShortcutsHelp — keyboard shortcuts modal

Source: `web/src/components/shortcuts-help.tsx`

```tsx
import { Modal, Table, Typography } from "antd";

type ShortcutsHelpProps = {
  open: boolean;
  onClose: () => void;
};

type ShortcutItem = {
  key: string;
  description: string;
};

const shortcuts: ShortcutItem[] = [
  { key: "Ctrl/Cmd + R", description: "重载配置" },
  { key: "g → d", description: "跳转到 Dashboard" },
  { key: "g → p", description: "跳转到 Providers" },
  { key: "g → t", description: "跳转到 Stats (统计)" },
  { key: "g → s", description: "跳转到 Settings (设置)" },
  { key: "g → e", description: "跳转到 Events (事件)" },
  { key: "g → a", description: "跳转到 About (关于)" },
  { key: "?", description: "显示此帮助面板" },
];

const columns = [
  {
    title: "快捷键",
    dataIndex: "key",
    key: "key",
    width: 160,
    render: (key: string) => (
      <Typography.Text keyboard style={{ fontFamily: "monospace" }}>
        {key}
      </Typography.Text>
    ),
  },
  {
    title: "功能",
    dataIndex: "description",
    key: "description",
  },
];

export function ShortcutsHelp({ open, onClose }: ShortcutsHelpProps): JSX.Element {
  return (
    <Modal
      title="键盘快捷键"
      open={open}
      onCancel={onClose}
      footer={null}
      width={480}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
        使用以下快捷键可以快速导航和操作：
      </Typography.Paragraph>
      <Table
        dataSource={shortcuts}
        columns={columns}
        rowKey="key"
        size="small"
        pagination={false}
        showHeader={false}
      />
      <Typography.Paragraph type="secondary" style={{ marginTop: 16, fontSize: "0.85em" }}>
        提示：g → d 表示先按 g，等待 1.5 秒内再按 d。
      </Typography.Paragraph>
    </Modal>
  );
}
```

## Utility helpers (no UI)

- `format-time.ts` — `formatCooldown(ms)` → "28s"/"3m12s"; `formatClockShort(ts)` → "HH:mm:ss"; `formatDateTime(iso)` → "YYYY-MM-DD HH:mm:ss"
- `use-countdown.ts` — per-second countdown hook from ISO timestamp
- `use-global-shortcuts.ts` — keyboard shortcuts (Ctrl/Cmd+R reload, g→x navigation, ? help)
- `use-visibility-polling.ts` — refetch interval enabled only when tab visible

## Charts (Recharts-based, feature-scoped)

- `components/charts/progress-bar.tsx` — custom progress bar (used in stats)
- `components/charts/` CSS: donut-chart.css, mini-trend.css, progress-bar.css

## Ant Design primitives

All UI primitives (Button, Card, Table, Tag, Input, Select, Segmented, Modal, Drawer, Tooltip) come from antd v5, themed globally by `createAppTheme` (see `theme.md` / `app/app-theme.ts`). No local wrapper components exist — pages consume antd directly with semantic CSS classes for layout.